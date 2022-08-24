/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vpcendpoint

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/openshift/aws-vpce-operator/api/v1alpha1"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
	"github.com/openshift/aws-vpce-operator/pkg/dnses"
	"github.com/openshift/aws-vpce-operator/pkg/infrastructures"
	"github.com/openshift/aws-vpce-operator/pkg/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// defaultAVOLogger returns a zap.Logger using RFC3339 timestamps for the vpcendpoint controller
func defaultAVOLogger() (logr.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)
	// TODO: Make this configurable
	// config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)

	zapBase, err := config.Build()
	if err != nil {
		return logr.Logger{}, err
	}

	logger := zapr.NewLogger(zapBase)
	return logger.WithName(controllerName), nil
}

// defaultAVORateLimiter returns a rate limiter that reconciles more slowly than the default.
// The default is 5ms --> 1000s, but resources are created much more slowly in AWS than in
// Kubernetes, so this helps avoid AWS rate limits.
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits
func defaultAVORateLimiter() workqueue.RateLimiter {
	return workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(1*time.Second, 5000*time.Second),
		// 10 qps, 100 bucket size, only for overall retry limiting (not per item)
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(10, 100)},
	)
}

// parseClusterInfo fills in the clusterInfo struct values inside the VpcEndpointReconciler
// and gets a new AWS session if refreshAWSSession is true.
// Generally, refreshAWSSession is only set to false during testing to mock the AWS client.
func (r *VpcEndpointReconciler) parseClusterInfo(ctx context.Context, refreshAWSSession bool) error {
	r.clusterInfo = new(clusterInfo)

	region, err := infrastructures.GetAWSRegion(ctx, r.Client)
	if err != nil {
		return err
	}
	r.clusterInfo.region = region
	r.log.V(1).Info("Parsed region from infrastructure", "region", region)

	if refreshAWSSession {
		sess, err := session.NewSession(&aws.Config{
			Region: &region,
		})
		if err != nil {
			return err
		}
		r.awsClient = aws_client.NewAwsClient(sess)
	}

	infraName, err := infrastructures.GetInfrastructureName(ctx, r.Client)
	if err != nil {
		return err
	}
	r.clusterInfo.infraName = infraName
	r.log.V(1).Info("Found infrastructure name:", "name", infraName)

	clusterTag, err := util.GetClusterTagKey(infraName)
	if err != nil {
		return err
	}
	r.clusterInfo.clusterTag = clusterTag
	r.log.V(1).Info("Found cluster tag:", "clusterTag", clusterTag)

	vpcId, err := r.awsClient.GetVPCId(r.clusterInfo.clusterTag)
	if err != nil {
		return err
	}
	r.clusterInfo.vpcId = vpcId
	r.log.V(1).Info("Found vpc id:", "vpcId", vpcId)

	domainName, err := dnses.GetPrivateHostedZoneDomainName(ctx, r.Client)
	if err != nil {
		return err
	}
	r.clusterInfo.domainName = domainName
	r.log.V(1).Info("Found domain name:", "domainName", domainName)

	return nil
}

func (r *VpcEndpointReconciler) defaultResourceRecord(resource *v1alpha1.VpcEndpoint) (*route53.ResourceRecord, error) {
	if resource.Status.VPCEndpointId == "" {
		return nil, fmt.Errorf("VPCEndpointID status is missing")
	}

	vpceResp, err := r.awsClient.DescribeSingleVPCEndpointById(resource.Status.VPCEndpointId)
	if err != nil {
		return nil, err
	}

	// VPCEndpoint doesn't exist anymore for some reason
	if vpceResp == nil || len(vpceResp.VpcEndpoints) == 0 {
		return nil, nil
	}

	// DNSEntries won't be populated until the state is available
	if *vpceResp.VpcEndpoints[0].State != "available" {
		return nil, fmt.Errorf("VPCEndpoint is not in the available state")
	}

	if len(vpceResp.VpcEndpoints[0].DnsEntries) == 0 {
		return nil, fmt.Errorf("VPCEndpoint has no DNS entries")
	}

	return &route53.ResourceRecord{
		Value: vpceResp.VpcEndpoints[0].DnsEntries[0].DnsName,
	}, nil
}

// expectedServiceForVpce generates the expected ExternalName service for a VpcEndpoint CustomResource
func (r *VpcEndpointReconciler) expectedServiceForVpce(resource *v1alpha1.VpcEndpoint) (*corev1.Service, error) {
	if resource == nil {
		// Should never happen
		return nil, fmt.Errorf("resource must be specified")
	}

	if resource.Spec.SubdomainName == "" {
		return nil, fmt.Errorf("subdomainName is a required field")
	}

	if r.clusterInfo.domainName == "" {
		return nil, fmt.Errorf("empty domainName")
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resource.Spec.ExternalNameService.Name,
			Namespace: resource.Spec.ExternalNameService.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.%s", resource.Spec.SubdomainName, r.clusterInfo.domainName),
		},
	}

	if err := controllerutil.SetControllerReference(resource, svc, r.Scheme); err != nil {
		return nil, err
	}

	return svc, nil
}

// tagsContains returns true if the all the tags in tagsToCheck exist in tags
func tagsContains(tags []*ec2.Tag, tagsToCheck map[string]string) bool {
	for k, v := range tagsToCheck {
		found := false
		for _, tag := range tags {
			if *tag.Key == k && *tag.Value == v {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
