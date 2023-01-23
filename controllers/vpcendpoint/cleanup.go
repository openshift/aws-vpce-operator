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

	"github.com/aws/aws-sdk-go-v2/aws"
	route53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// cleanupAwsResources cleans up AWS resources associated with a VPC Endpoint.
func (r *VpcEndpointReconciler) cleanupAwsResources(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error {
	if meta.IsStatusConditionTrue(resource.Status.Conditions, avov1alpha2.AWSRoute53RecordCondition) {
		resourceRecord, err := r.generateRoute53Record(ctx, resource)
		if err != nil {
			return err
		}

		resp, err := r.awsClient.GetHostedZone(ctx, resource.Status.HostedZoneId)
		if err != nil {
			return err
		}

		if resp.HostedZone != nil {
			r.log.V(0).Info("Deleting Route53 Hosted Zone Record")
			input := &route53Types.ResourceRecordSet{
				Name:            aws.String(fmt.Sprintf("%s.%s", resource.Spec.CustomDns.Route53PrivateHostedZone.Record.Hostname, *resp.HostedZone.Name)),
				ResourceRecords: []route53Types.ResourceRecord{*resourceRecord},
				TTL:             aws.Int64(300),
				Type:            route53Types.RRTypeCname,
			}

			if _, err := r.awsClient.DeleteResourceRecordSet(ctx, input, *resp.HostedZone.Id); err != nil {
				return err
			}
		}

		resource.Status.ResourceRecordSet = ""
		meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
			Type:    avov1alpha2.AWSRoute53RecordCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "Deleted",
			Message: "Deleted Route53 Hosted Zone Record",
		})

		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}
	}

	if resource.Status.HostedZoneId != "" {
		// Only delete a Route53 Private Hosted Zone if AVO created it
		if resource.Spec.CustomDns.Route53PrivateHostedZone.DomainName != "" {
			if _, err := r.awsClient.DeleteHostedZone(ctx, resource.Status.HostedZoneId); err != nil {
				return err
			}
		}
		resource.Status.HostedZoneId = ""
		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}
	}

	if resource.Status.VPCEndpointId != "" {
		if err := r.cleanupMetrics(ctx, resource); err != nil {
			return err
		}

		r.log.V(0).Info("Deleting AWS resources", "VpcEndpoint", resource.Status.VPCEndpointId)
		if _, err := r.awsClient.DeleteVPCEndpoint(ctx, resource.Status.VPCEndpointId); err != nil {
			return err
		}

		resource.Status.Status = "deleting"
		resource.Status.VPCEndpointId = ""
		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}
	}

	if resource.Status.SecurityGroupId != "" {
		r.log.V(0).Info("Deleting AWS resources", "SecurityGroup", resource.Status.SecurityGroupId)
		if _, err := r.awsClient.DeleteSecurityGroup(ctx, resource.Status.SecurityGroupId); err != nil {
			return err
		}

		resource.Status.SecurityGroupId = ""
		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}
	}

	r.log.V(0).Info("AWS cleanup complete")
	return nil
}

// cleanupMetrics deletes metrics associated with a specific VPCEndpoint custom resource in a best-effort manner
func (r *VpcEndpointReconciler) cleanupMetrics(_ context.Context, resource *avov1alpha2.VpcEndpoint) error {
	if resource.Status.VPCEndpointId != "" {
		// DeleteLabelValues returns true if the metric is deleted, false otherwise, currently we don't really care
		// either way, so just always return nil
		vpcePendingAcceptance.DeleteLabelValues(resource.Name, resource.Namespace, resource.Status.VPCEndpointId)
	}

	// If .status.VPCEndpointId is empty, we can't delete the metric, but don't care
	return nil
}
