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
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	route53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"github.com/openshift/aws-vpce-operator/pkg/dnses"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Validation func(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error

func (r *VpcEndpointReconciler) validateResources(ctx context.Context, resource *avov1alpha2.VpcEndpoint, validations []Validation) error {
	for _, validation := range validations {
		if err := validation(ctx, resource); err != nil {
			return err
		}
	}

	return nil
}

// validateVpcEndpointCR is doing the job of a validating webhook and ensuring the provided CR is copacetic
// beyond what OpenAPI v3 validation can do.
func validateVpcEndpointCR(vpce *avov1alpha2.VpcEndpoint) error {
	// TODO: Ideally this should be a validating webhook

	// Cannot override region with autodiscovery of subnets or Route 53 Private Hosted Zone
	if vpce.Spec.Region != "" {
		if vpce.Spec.Vpc.AutoDiscoverSubnets {
			return errors.New(".spec.vpc.autoDiscoverSubnets is not supported with .spec.region")
		}

		if vpce.Spec.CustomDns.Route53PrivateHostedZone.AutoDiscover {
			return errors.New(".spec.customDns.route53PrivateHostedZone.autoDiscover is not supported with .spec.region")
		}
	}

	// Must auto-discover subnets and cannot specify subnet ids with VPC load balancing
	if len(vpce.Spec.Vpc.Ids) > 0 {
		if !vpce.Spec.Vpc.AutoDiscoverSubnets {
			return errors.New(".spec.vpc.autoDiscoverSubnets must be true when specifying VPCs to load balance")
		}

		if len(vpce.Spec.Vpc.SubnetIds) > 0 {
			return errors.New(".spec.vpc.subnetIds is not supported with .spec.vpc.autoDiscoverSubnets")
		}
	}

	// Custom DNS validations
	if vpce.Spec.CustomDns.Route53PrivateHostedZone.Id != "" && vpce.Spec.CustomDns.Route53PrivateHostedZone.DomainName != "" {
		return errors.New("cannot set both .spec.customDns.route53PrivateHostedZone.id and .spec.customDns.route53PrivateHostedZone.domainName")
	}

	if vpce.Spec.CustomDns.Route53PrivateHostedZone.Record.Hostname == "" && vpce.Spec.CustomDns.Route53PrivateHostedZone.Record.ExternalNameService.Name != "" {
		return errors.New("cannot create an ExternalName service without a Route53 Hosted Zone record")
	}

	return nil
}

// validateSecurityGroup checks a security group against what's expected, returning an error if there are differences.
// Security groups can't be updated-in-place, so a new one will need to be created before deleting this existing one.
func (r *VpcEndpointReconciler) validateSecurityGroup(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error {
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
		Type:    avov1alpha2.AWSSecurityGroupCondition,
		Status:  metav1.ConditionTrue,
		Reason:  "Validated",
		Message: "Validated",
	})
	if err := r.Status().Update(ctx, resource); err != nil {
		r.log.V(0).Error(err, "failed to update status")
		return err
	}

	return nil
}

// validateVPCEndpoint checks a VPC endpoint with what's expected and reconciles their state
// returning an error if it cannot do so.
func (r *VpcEndpointReconciler) validateVPCEndpoint(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error {
	if resource == nil {
		// Should never happen
		return fmt.Errorf("resource must be specified")
	}

	vpce, err := r.findOrCreateVpcEndpoint(ctx, resource)
	if err != nil {
		return err
	}

	if resource.Status.VPCEndpointId != *vpce.VpcEndpointId {
		resource.Status.VPCEndpointId = *vpce.VpcEndpointId
		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}
	}

	// When this bug is fixed we can switch/case off of enums
	// https://github.com/aws/aws-sdk/issues/116
	switch vpce.State {
	case "pendingAcceptance":
		vpcePendingAcceptance.WithLabelValues(resource.Name, resource.Namespace, resource.Status.VPCEndpointId).Set(1)
		// Nothing we can do at the moment, the VPC Endpoint needs to be accepted
		r.log.V(0).Info("Waiting for VPC Endpoint connection acceptance", "status", string(vpce.State))
		meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
			Type:   avov1alpha2.AWSVpcEndpointCondition,
			Status: metav1.ConditionFalse,
			Reason: string(vpce.State),
		})
		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}

		return nil
	case "deleting", "pending":
		// Nothing we can do at the moment, the VPC Endpoint needs to finish moving into a stable state
		vpcePendingAcceptance.WithLabelValues(resource.Name, resource.Namespace, resource.Status.VPCEndpointId).Set(0)
		r.log.V(0).Info("VPC Endpoint is transitioning state", "status", string(vpce.State))
		meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
			Type:   avov1alpha2.AWSVpcEndpointCondition,
			Status: metav1.ConditionFalse,
			Reason: string(vpce.State),
		})
		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}

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
			Type:   avov1alpha2.AWSVpcEndpointCondition,
			Status: metav1.ConditionFalse,
			Reason: string(vpce.State),
		})
		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}

		return fmt.Errorf("vpc endpoint in a bad state: %s", vpce.State)
	}

	err = r.ensureVpcEndpointSubnets(ctx, vpce, resource)
	if err != nil {
		return fmt.Errorf("failed to reconcile VPC Endpoint subnets: %w", err)
	}

	err = r.ensureVpcEndpointSecurityGroups(ctx, vpce, resource)
	if err != nil {
		return fmt.Errorf("failed to reconcile VPC Endpoint security groups: %w", err)
	}

	meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
		Type:    avov1alpha2.AWSVpcEndpointCondition,
		Status:  metav1.ConditionTrue,
		Reason:  string(vpce.State),
		Message: fmt.Sprintf("VPC Endpoint status is: %s", string(vpce.State)),
	})
	if err := r.Status().Update(ctx, resource); err != nil {
		r.log.V(0).Error(err, "failed to update status")
		return err
	}

	return nil
}

func (r *VpcEndpointReconciler) validateCustomDns(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error {
	if err := r.validateResources(ctx, resource,
		[]Validation{
			r.validateR53PrivateHostedZone,
			r.validateR53HostedZoneRecord,
			r.validateExternalNameService,
		}); err != nil {
		return err
	}

	return nil
}

// validateR53PrivateHostedZone ensures the configured CustomDns Private Hosted Zone exists
func (r *VpcEndpointReconciler) validateR53PrivateHostedZone(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error {
	if resource == nil {
		// Should never happen
		return errors.New("resource must be specified")
	}

	if resource.Spec.CustomDns.Route53PrivateHostedZone.AutoDiscover {
		domainName, err := dnses.GetPrivateHostedZoneDomainName(ctx, r.Client)
		if err != nil {
			return err
		}
		r.log.V(1).Info("Found domain name:", "domainName", domainName)

		r.log.V(1).Info("Searching for Route53 Hosted Zone by domain name", "domainName", domainName)
		hz, err := r.awsClient.GetDefaultPrivateHostedZoneId(ctx, domainName, r.clusterInfo.vpcId, r.clusterInfo.region)
		if err != nil {
			return err
		}

		if resource.Status.HostedZoneId != *hz.HostedZoneId {
			resource.Status.HostedZoneId = *hz.HostedZoneId
			if err := r.Status().Update(ctx, resource); err != nil {
				r.log.V(0).Error(err, "failed to update status")
				return err
			}
		}

		return nil
	}

	if resource.Spec.CustomDns.Route53PrivateHostedZone.Id != "" {
		r.log.V(0).Info("Searching for Route 53 Hosted Zone", "id", resource.Spec.CustomDns.Route53PrivateHostedZone.Id)
		resp, err := r.awsClient.GetHostedZone(ctx, resource.Spec.CustomDns.Route53PrivateHostedZone.Id)
		if err != nil {
			return err
		}

		if resource.Status.HostedZoneId != *resp.HostedZone.Id {
			resource.Status.HostedZoneId = *resp.HostedZone.Id
			if err := r.Status().Update(ctx, resource); err != nil {
				r.log.V(0).Error(err, "failed to update status")
				return err
			}
		}

		return nil
	}

	if resource.Spec.CustomDns.Route53PrivateHostedZone.DomainName != "" {
		if err := r.findOrCreatePrivateHostedZone(ctx, resource); err != nil {
			return err
		}

		if err := r.createMissingPrivateZoneTags(ctx, resource.Status.HostedZoneId); err != nil {
			return err
		}
		return nil
	}

	return nil
}

// validateR53HostedZoneRecord ensures a DNS record exists for the given VPC Endpoint
func (r *VpcEndpointReconciler) validateR53HostedZoneRecord(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error {
	if resource == nil {
		// Should never happen
		return errors.New("resource must be specified")
	}

	resp, err := r.awsClient.GetHostedZone(ctx, resource.Status.HostedZoneId)
	if err != nil {
		return err
	}

	resourceRecord, err := r.generateRoute53Record(ctx, resource)
	if err != nil {
		r.log.V(0).Info("Skipping Route53 Record, VPCEndpoint is not in the available state")
		return nil
	}

	input := &route53Types.ResourceRecordSet{
		Name:            aws.String(fmt.Sprintf("%s.%s", resource.Spec.CustomDns.Route53PrivateHostedZone.Record.Hostname, strings.TrimRight(*resp.HostedZone.Name, "."))),
		ResourceRecords: []route53Types.ResourceRecord{*resourceRecord},
		TTL:             aws.Int64(300),
		Type:            route53Types.RRTypeCname,
	}

	if _, err := r.awsClient.UpsertResourceRecordSet(ctx, input, *resp.HostedZone.Id); err != nil {
		return err
	}
	r.log.V(0).Info("Route53 Hosted Zone Record exists", "domainName", *input.Name)

	resource.Status.ResourceRecordSet = *input.Name
	meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
		Type:    avov1alpha2.AWSRoute53RecordCondition,
		Status:  metav1.ConditionTrue,
		Reason:  "Created",
		Message: fmt.Sprintf("Created: %s", *input.Name),
	})
	if err := r.Status().Update(ctx, resource); err != nil {
		r.log.V(0).Error(err, "failed to update status")
		return err
	}

	return nil
}

// validateExternalNameService checks if the expected ExternalName service exists, creating or updating it as needed
func (r *VpcEndpointReconciler) validateExternalNameService(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error {
	if resource == nil {
		// Should never happen
		return errors.New("cannot generate ExternalName service: custom resource is nil")
	}

	if resource.Spec.CustomDns.Route53PrivateHostedZone.Record.Hostname == "" ||
		resource.Spec.CustomDns.Route53PrivateHostedZone.Record.ExternalNameService.Name == "" {
		// Fields for generating an externalName service are not set
		return nil
	}

	found := &corev1.Service{}
	expected, err := r.generateExternalNameService(resource)
	if err != nil {
		return err
	}

	err = r.Get(ctx, types.NamespacedName{
		Name:      resource.Spec.CustomDns.Route53PrivateHostedZone.Record.ExternalNameService.Name,
		Namespace: resource.Namespace,
	}, found)
	if err != nil {
		if kerr.IsNotFound(err) {
			// Create the ExternalName service since it's missing
			r.log.V(0).Info("Creating ExternalName service", "service", expected)
			if err := r.Create(ctx, expected); err != nil {
				r.log.V(0).Error(err, "failed to create ExternalName service")
				return err
			}

			meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
				Type:   avov1alpha2.ExternalNameServiceCondition,
				Status: metav1.ConditionTrue,
				Reason: "Created",
			})
			if err := r.Status().Update(ctx, resource); err != nil {
				r.log.V(0).Error(err, "failed to update status")
				return err
			}

			// Requeue, but no error
			return fmt.Errorf("requeue to validate service")
		} else {
			meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
				Type:    avov1alpha2.ExternalNameServiceCondition,
				Status:  metav1.ConditionFalse,
				Reason:  "UnknownError",
				Message: fmt.Sprintf("Unkown error: %v", err),
			})
			if err := r.Status().Update(ctx, resource); err != nil {
				r.log.V(0).Error(err, "failed to update status")
				return err
			}

			return err
		}
	}

	// The only mutable field we care about is .spec.ExternalName, fix it if it got messed up
	if found.Spec.ExternalName != expected.Spec.ExternalName {
		found.Spec.ExternalName = expected.Spec.ExternalName
		r.log.V(0).Info("Updating ExternalName service", "service", found)
		if err := r.Update(ctx, found); err != nil {
			meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
				Type:    avov1alpha2.ExternalNameServiceCondition,
				Status:  metav1.ConditionFalse,
				Reason:  "UnknownError",
				Message: fmt.Sprintf("Unkown error: %v", err),
			})
			if err := r.Status().Update(ctx, resource); err != nil {
				r.log.V(0).Error(err, "failed to update status")
				return err
			}

			return err
		}

		meta.SetStatusCondition(&resource.Status.Conditions, metav1.Condition{
			Type:   avov1alpha2.ExternalNameServiceCondition,
			Status: metav1.ConditionTrue,
			Reason: "Reconciled",
		})
		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}
	}

	return nil
}
