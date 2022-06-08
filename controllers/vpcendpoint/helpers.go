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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/openshift/aws-vpce-operator/api/v1alpha1"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
	"github.com/openshift/aws-vpce-operator/pkg/dnses"
	"github.com/openshift/aws-vpce-operator/pkg/infrastructures"
	"github.com/openshift/aws-vpce-operator/pkg/util"
)

// parseClusterInfo fills in the ClusterInfo struct values inside the VpcEndpointReconciler
func (r *VpcEndpointReconciler) parseClusterInfo(ctx context.Context) error {
	r.ClusterInfo = new(ClusterInfo)

	region, err := infrastructures.GetAWSRegion(ctx, r.Client)
	if err != nil {
		return err
	}
	r.ClusterInfo.Region = region
	r.Log.V(1).Info("Parsed region from infrastructure", "region", region)

	sess, err := session.NewSession(&aws.Config{
		Region: &region,
	})
	if err != nil {
		return err
	}
	r.AWSClient = aws_client.New(sess)

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

	vpcId, err := r.AWSClient.GetVPCId(clusterTag)
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
