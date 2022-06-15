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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const controllerName = "vpcendpoint"

// defaultLogger returns a zap.Logger using RFC3339 timestamps for the vpcendpoint controller
func defaultLogger() (logr.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)

	zapBase, err := config.Build()
	if err != nil {
		return logr.Logger{}, err
	}

	logger := zapr.NewLogger(zapBase)
	return logger.WithName(controllerName), nil
}

// parseClusterInfo fills in the ClusterInfo struct values inside the VpcEndpointReconciler
// and gets a new AWS session if refreshAWSSession is true.
// Generally, refreshAWSSession is only set to false during testing to mock the AWS client.
func (r *VpcEndpointReconciler) parseClusterInfo(ctx context.Context, refreshAWSSession bool) error {
	r.ClusterInfo = new(ClusterInfo)

	region, err := infrastructures.GetAWSRegion(ctx, r.Client)
	if err != nil {
		return err
	}
	r.ClusterInfo.Region = region
	r.Log.V(1).Info("Parsed region from infrastructure", "region", region)

	if refreshAWSSession {
		sess, err := session.NewSession(&aws.Config{
			Region: &region,
		})
		if err != nil {
			return err
		}
		r.AWSClient = aws_client.New(sess)
	}

	infraName, err := infrastructures.GetInfrastructureName(ctx, r.Client)
	if err != nil {
		return err
	}
	r.ClusterInfo.InfraName = infraName
	r.Log.V(1).Info("Found infrastructure name:", "name", infraName)

	clusterTag, err := util.GetClusterTagKey(infraName)
	if err != nil {
		return err
	}
	r.ClusterInfo.ClusterTag = clusterTag
	r.Log.V(1).Info("Found cluster tag:", "clusterTag", clusterTag)

	vpcId, err := r.AWSClient.GetVPCId(r.ClusterInfo.ClusterTag)
	if err != nil {
		return err
	}
	r.ClusterInfo.VpcId = vpcId
	r.Log.V(1).Info("Found vpc id:", "vpcId", vpcId)

	domainName, err := dnses.GetPrivateHostedZoneDomainName(ctx, r.Client)
	if err != nil {
		return err
	}
	r.ClusterInfo.DomainName = domainName
	r.Log.V(1).Info("Found domain name:", "domainName", domainName)

	return nil
}

func (r *VpcEndpointReconciler) defaultResourceRecord(resource *v1alpha1.VpcEndpoint) (*route53.ResourceRecord, error) {
	if resource.Status.VPCEndpointId == "" {
		return nil, fmt.Errorf("VPCEndpointID status is missing")
	}

	vpceResp, err := r.AWSClient.DescribeSingleVPCEndpointById(resource.Status.VPCEndpointId)
	if err != nil {
		return nil, err
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

func (r *VpcEndpointReconciler) ensureExternalNameService(ctx context.Context, resource *v1alpha1.VpcEndpoint) error {
	if resource.Status.ExternalServiceNameStatus == "" {
		r.Log.V(1).Info("ExternalName service is missing, creating a new one.")
		r.Client.Create(ctx, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resource.Spec.SubdomainName,
				Namespace: resource.Namespace,
			},
			Spec: corev1.ServiceSpec{
				Type:         corev1.ServiceTypeExternalName,
				ExternalName: fmt.Sprintf("%s.%s", resource.Spec.SubdomainName, r.ClusterInfo.DomainName),
			},
		})
		return fmt.Errorf("failed to create externalName service")
	}
	return nil
}
