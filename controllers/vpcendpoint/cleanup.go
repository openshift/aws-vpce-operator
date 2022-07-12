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
	"github.com/aws/aws-sdk-go/service/route53"

	avov1alpha1 "github.com/openshift/aws-vpce-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// deleteAWSResources cleans up AWS resources associated with a VPC Endpoint.
func (r *VpcEndpointReconciler) deleteAWSResources(ctx context.Context, resource *avov1alpha1.VpcEndpoint) error {
	if meta.IsStatusConditionTrue(resource.Status.Conditions, avov1alpha1.AWSRoute53RecordCondition) {
		resourceRecord, err := r.defaultResourceRecord(resource)
		if err != nil {
			return err
		}

		hostedZone, err := r.awsClient.GetDefaultPrivateHostedZoneId(r.clusterInfo.domainName)
		if err != nil {
			return err
		}

		input := &route53.ResourceRecordSet{
			Name:            aws.String(fmt.Sprintf("%s.%s", resource.Spec.SubdomainName, *hostedZone.Name)),
			ResourceRecords: []*route53.ResourceRecord{resourceRecord},
			TTL:             aws.Int64(300),
			Type:            aws.String("CNAME"),
		}

		r.log.V(0).Info("Deleting Route53 Hosted Zone Record")
		if _, err := r.awsClient.DeleteResourceRecordSet(input, *hostedZone.Id); err != nil {
			return err
		}

		meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
			Type:    avov1alpha1.AWSRoute53RecordCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "Deleted",
			Message: "Deleted Route53 Hosted Zone Record",
		})

		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}
	}

	if resource.Status.VPCEndpointId != "" {
		r.log.V(0).Info("Deleting AWS resources", "VpcEndpoint", resource.Status.VPCEndpointId)
		if _, err := r.awsClient.DeleteVPCEndpoint(resource.Status.VPCEndpointId); err != nil {
			return err
		}

		resource.Status.VPCEndpointId = ""
		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}
	}

	if resource.Status.SecurityGroupId != "" {
		r.log.V(0).Info("Deleting AWS resources", "SecurityGroup", resource.Status.SecurityGroupId)
		if _, err := r.awsClient.DeleteSecurityGroup(resource.Status.SecurityGroupId); err != nil {
			return err
		}

		resource.Status.SecurityGroupId = ""
		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}
	}

	return nil
}

// cleanupMetrics deletes metrics associated with a specific custom resource in a best-effort manner
func (r *VpcEndpointReconciler) cleanupMetrics(ctx context.Context, resource *avov1alpha1.VpcEndpoint) error {
	// DeleteLabelValues returns true if the metric is deleted, false otherwise, currently we don't really care
	// either way, so just always return nil
	vpcePendingAcceptance.DeleteLabelValues(resource.Name, resource.Namespace)

	return nil
}
