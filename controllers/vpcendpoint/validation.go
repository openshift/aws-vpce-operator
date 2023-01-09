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
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	route53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	avov1alpha1 "github.com/openshift/aws-vpce-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"strings"
	"time"
)

type ValidateAWSResourceFunc func(ctx context.Context, resource *avov1alpha1.VpcEndpoint) error

func (r *VpcEndpointReconciler) validateAWSResources(
	ctx context.Context,
	resource *avov1alpha1.VpcEndpoint,
	validationFuncs []ValidateAWSResourceFunc) error {
	for _, validationFunc := range validationFuncs {
		if err := validationFunc(ctx, resource); err != nil {
			return err
		}

		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}
	}

	return nil
}

// validateSecurityGroup checks a security group against what's expected, returning an error if there are differences.
// Security groups can't be updated-in-place, so a new one will need to be created before deleting this existing one.
func (r *VpcEndpointReconciler) validateSecurityGroup(ctx context.Context, resource *avov1alpha1.VpcEndpoint) error {
	if resource == nil {
		// Should never happen
		return fmt.Errorf("resource must be specified")
	}

	sg, err := r.findOrCreateSecurityGroup(ctx, resource)
	if err != nil {
		return err
	}

	if err := r.createMissingSecurityGroupTags(ctx, sg, resource); err != nil {
		return err
	}

	ingressInput, egressInput, err := r.generateMissingSecurityGroupRules(ctx, sg, resource)
	if err != nil {
		return err
	}

	// Not idempotent
	if _, err := r.awsClient.AuthorizeSecurityGroupRules(ctx, ingressInput, egressInput); err != nil {
		return err
	}

	meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
		Type:    avov1alpha1.AWSSecurityGroupCondition,
		Status:  metav1.ConditionTrue,
		Reason:  "Validated",
		Message: "Validated",
	})

	return nil
}

// validateVPCEndpoint checks a VPC endpoint with what's expected and reconciles their state
// returning an error if it cannot do so.
func (r *VpcEndpointReconciler) validateVPCEndpoint(ctx context.Context, resource *avov1alpha1.VpcEndpoint) error {
	if resource == nil {
		// Should never happen
		return fmt.Errorf("resource must be specified")
	}

	vpce, err := r.findOrCreateVpcEndpoint(ctx, resource)
	if err != nil {
		return err
	}

	resource.Status.VPCEndpointId = *vpce.VpcEndpointId
	resource.Status.Status = string(vpce.State)

	// When this bug is fixed we can switch/case off of enums
	// https://github.com/aws/aws-sdk/issues/116
	switch vpce.State {
	case "pendingAcceptance":
		vpcePendingAcceptance.WithLabelValues(resource.Name, resource.Namespace, resource.Status.VPCEndpointId).Set(1)
		// Nothing we can do at the moment, the VPC Endpoint needs to be accepted
		r.log.V(0).Info("Waiting for VPC Endpoint connection acceptance", "status", string(vpce.State))
		meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
			Type:   avov1alpha1.AWSVpcEndpointCondition,
			Status: metav1.ConditionFalse,
			Reason: string(vpce.State),
		})

		return nil
	case "deleting", "pending":
		// Nothing we can do at the moment, the VPC Endpoint needs to finish moving into a stable state
		r.log.V(0).Info("VPC Endpoint is transitioning state", "status", string(vpce.State))
		meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
			Type:   avov1alpha1.AWSVpcEndpointCondition,
			Status: metav1.ConditionFalse,
			Reason: string(vpce.State),
		})

		return nil
	case "available":
		vpcePendingAcceptance.WithLabelValues(resource.Name, resource.Namespace, resource.Status.VPCEndpointId).Set(0)
		r.log.V(0).Info("VPC Endpoint ready", "status", string(vpce.State))
	case ec2Types.StateFailed, ec2Types.StateRejected, ec2Types.StateDeleted:
		// No other known states, but just in case catch with a default
		fallthrough
	default:
		// TODO: If rejected, we may want an option to recreate the VPC Endpoint and try again
		vpcePendingAcceptance.WithLabelValues(resource.Name, resource.Namespace, resource.Status.VPCEndpointId).Set(0)
		r.log.V(0).Info("VPC Endpoint in a bad state", "status", string(vpce.State))
		meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
			Type:   avov1alpha1.AWSVpcEndpointCondition,
			Status: metav1.ConditionFalse,
			Reason: string(vpce.State),
		})

		return fmt.Errorf("vpc endpoint in a bad state: %s", vpce.State)
	}

	err = r.ensureVpcEndpointSubnets(ctx, vpce)
	if err != nil {
		return fmt.Errorf("failed to reconcile VPC Endpoint subnets: %w", err)
	}

	err = r.ensureVpcEndpointSecurityGroups(ctx, vpce, resource)
	if err != nil {
		return fmt.Errorf("failed to reconcile VPC Endpoint security groups: %w", err)
	}

	meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
		Type:   avov1alpha1.AWSVpcEndpointCondition,
		Status: metav1.ConditionTrue,
		Reason: string(vpce.State),
	})

	return nil
}

// validateR53HostedZoneRecord ensures a DNS record exists for the given VPC Endpoint
func (r *VpcEndpointReconciler) validateR53HostedZoneRecord(ctx context.Context, resource *avov1alpha1.VpcEndpoint) error {
	if resource == nil {
		// Should never happen
		return fmt.Errorf("resource must be specified")
	}

	r.log.V(1).Info("Searching for Route53 Hosted Zone by domain name", "domainName", r.clusterInfo.domainName)
	hostedZone, err := r.awsClient.GetDefaultPrivateHostedZoneId(ctx, r.clusterInfo.domainName)
	if err != nil {
		return err
	}

	resourceRecord, err := r.generateRoute53Record(ctx, resource)
	if err != nil {
		r.log.V(0).Info("Skipping Route53 Record, VPCEndpoint is not in the available state")
		return nil
	}

	input := &route53Types.ResourceRecordSet{
		Name:            aws.String(fmt.Sprintf("%s.%s", resource.Spec.SubdomainName, *hostedZone.Name)),
		ResourceRecords: []route53Types.ResourceRecord{*resourceRecord},
		TTL:             aws.Int64(300),
		Type:            route53Types.RRTypeCname,
	}

	if _, err := r.awsClient.UpsertResourceRecordSet(ctx, input, *hostedZone.Id); err != nil {
		return err
	}
	r.log.V(1).Info("Route53 Hosted Zone Record exists", "domainName", fmt.Sprintf("%s.%s", resource.Spec.SubdomainName, *hostedZone.Name))

	meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
		Type:    avov1alpha1.AWSRoute53RecordCondition,
		Status:  metav1.ConditionTrue,
		Reason:  "Created",
		Message: fmt.Sprintf("Created: %s.%s", resource.Spec.SubdomainName, *hostedZone.Name),
	})

	return nil
}

// validateExternalNameService checks if the expected ExternalName service exists, creating or updating it as needed
func (r *VpcEndpointReconciler) validateExternalNameService(ctx context.Context, resource *avov1alpha1.VpcEndpoint) error {
	found := &corev1.Service{}
	expected, err := r.generateExternalNameService(resource)
	if err != nil {
		return err
	}

	err = r.Get(ctx, types.NamespacedName{
		Name:      resource.Spec.ExternalNameService.Name,
		Namespace: resource.Namespace,
	}, found)
	if err != nil {
		if kerr.IsNotFound(err) {
			// Create the ExternalName service since it's missing
			r.log.V(0).Info("Creating ExternalName service", "service", expected)
			err = r.Create(ctx, expected)
			if err != nil {
				r.log.V(0).Error(err, "failed to create ExternalName service")
				return err
			}

			meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
				Type:   avov1alpha1.ExternalNameServiceCondition,
				Status: metav1.ConditionTrue,
				Reason: "Created",
			})

			// Requeue, but no error
			return fmt.Errorf("requeue to validate service")
		} else {
			meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
				Type:    avov1alpha1.ExternalNameServiceCondition,
				Status:  metav1.ConditionFalse,
				Reason:  "UnknownError",
				Message: fmt.Sprintf("Unkown error: %v", err),
			})

			return err
		}
	}

	// The only mutable field we care about is .spec.ExternalName, fix it if it got messed up
	if found.Spec.ExternalName != expected.Spec.ExternalName {
		found.Spec.ExternalName = expected.Spec.ExternalName
		r.log.V(0).Info("Updating ExternalName service", "service", found)
		if err := r.Update(ctx, found); err != nil {
			meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
				Type:    avov1alpha1.ExternalNameServiceCondition,
				Status:  metav1.ConditionFalse,
				Reason:  "UnknownError",
				Message: fmt.Sprintf("Unkown error: %v", err),
			})

			return err
		}

		meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
			Type:   avov1alpha1.ExternalNameServiceCondition,
			Status: metav1.ConditionTrue,
			Reason: "Reconciled",
		})
	}

	return nil
}

// validatePrivateHostedZone attempts to find if the AddtlHostedZoneName has a zone, and if that zone is private, creating it if not found.
func (r *VpcEndpointReconciler) validatePrivateHostedZone(ctx context.Context, resource *avov1alpha1.VpcEndpoint) error {
	// AddtlHostedZoneName is optional, return if not present
	if resource.Spec.AddtlHostedZoneName == "" {
		return nil
	}

	zoneOut, err := r.awsClient.GetDefaultPrivateHostedZoneId(ctx, resource.Spec.AddtlHostedZoneName)
	if err != nil {
		return fmt.Errorf("failed to locate private hosted zone: %s", err)
	}

	if zoneOut.Config.PrivateZone { // AddtlHostedZoneName found with a PrivateZone
		r.log.V(1).Info("Found AddtlHostedZone's Route53 hosted zone", "domainName", zoneOut.Name)
		trimmedZoneID := strings.TrimPrefix(aws.ToString(zoneOut.Id), "/hostedzone/")
		if err := r.createMissingPrivateZoneTags(ctx, trimmedZoneID); err != nil {
			return fmt.Errorf("failed to tag hosted zone: %w", err)
		}
		r.log.V(1).Info("Private Hosted Zone validated", "domainName", zoneOut.Name)
		return nil
	}

	r.log.V(0).Info("Creating a new Route53 Hosted Zone", "domainName", resource.Spec.AddtlHostedZoneName)
	_, err = r.awsClient.CreateNewHostedZone(ctx, resource.Spec.AddtlHostedZoneName, r.clusterInfo.vpcId, time.Now().String(), r.clusterInfo.region)
	if err != nil {
		return fmt.Errorf("failed to create hosted zone: %s", err)
	}
	return nil
}
