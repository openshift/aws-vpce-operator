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
	"github.com/aws/smithy-go"
	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
	"github.com/openshift/aws-vpce-operator/pkg/dnses"
	"github.com/openshift/aws-vpce-operator/pkg/secrets"
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

// validateSecurityGroup checks a security group against what's expected, returning an error if there are differences.
// Security groups can't be updated-in-place, so a new one will need to be created before deleting this existing one.
func (r *VpcEndpointReconciler) validateSecurityGroup(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error {
	if resource == nil {
		// Should never happen
		return fmt.Errorf("resource must be specified")
	}

	sg, err := r.findOrCreateSecurityGroup(ctx, resource)
	if err != nil {
		r.log.V(0).Error(err, "failed to find or create security groups")
		return err
	}

	if err := r.createMissingSecurityGroupTags(ctx, sg, resource); err != nil {
		r.log.V(0).Error(err, "failed to create missing security group tags")
		return err
	}

	ingressInput, egressInput, err := r.generateMissingSecurityGroupRules(ctx, sg, resource)
	if err != nil {
		r.log.V(0).Error(err, "failed to generate missing security group rules")
		return err
	}

	// Not idempotent
	if _, err := r.awsClient.AuthorizeSecurityGroupRules(ctx, ingressInput, egressInput); err != nil {
		r.log.V(1).Error(err, "failed to authorize security group rules")
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
		r.log.V(0).Info("VPC Endpoint ready", "id", resource.Status.VPCEndpointId)
	case "rejected":
		r.log.V(0).Info("VPC Endpoint rejected, starting deletion", "id", resource.Status.VPCEndpointId)
		if _, err := r.awsClient.DeleteVPCEndpoint(ctx, resource.Status.VPCEndpointId); err != nil {
			var ae smithy.APIError
			if errors.As(err, &ae) {
				if ae.ErrorCode() == "InvalidVpcEndpoint.NotFound" {
					resource.Status.VPCEndpointId = ""
				} else {
					return err
				}
			} else {
				return err
			}
		}

		return fmt.Errorf("VPC Endpoint unexpectedly needed to be deleted")
	case ec2Types.StateFailed, ec2Types.StateDeleted:
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
			r.validateR53HostedZoneAuthorization,
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
		domainName, err := dnses.GetPrivateHostedZoneDomainName(ctx, r.Client, dnses.DefaultDnsesName)
		if err != nil {
			return err
		}
		r.log.V(1).Info("Found domain name:", "domainName", domainName)

		r.log.V(1).Info("Searching for Route53 Hosted Zone by domain name", "domainName", domainName)
		hz, err := r.awsClient.GetDefaultPrivateHostedZoneId(ctx, domainName, resource.Status.VPCId, r.clusterInfo.region)
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

	if resource.Spec.CustomDns.Route53PrivateHostedZone.DomainName != "" || resource.Spec.CustomDns.Route53PrivateHostedZone.DomainNameRef != nil {
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

func (r *VpcEndpointReconciler) validateR53HostedZoneAuthorization(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error {
	if resource == nil {
		// Should never happen
		return errors.New("resource must be specified")
	}

	if len(resource.Spec.CustomDns.Route53PrivateHostedZone.AssociatedVpcs) == 0 {
		return nil
	}

	r.log.V(1).Info("Ensuring Route53 Hosted Zone has all additional authorized VPCs", "id", resource.Status.HostedZoneId)
	if resource.Status.HostedZoneId == "" {
		return errors.New("cannot validate hosted zone authorizations with an empty resource.status.hostedZoneId")
	}

	r.log.V(1).Info("Searching for Route53 Hosted Zone by id", "id", resource.Status.HostedZoneId)
	resp, err := r.awsClient.GetHostedZone(ctx, resource.Status.HostedZoneId)
	if err != nil {
		return err
	}

	associatedVpcs := map[string]struct{}{}
	for _, vpc := range resp.VPCs {
		associatedVpcs[*vpc.VPCId] = struct{}{}
	}

	for _, v := range resource.Spec.CustomDns.Route53PrivateHostedZone.AssociatedVpcs {
		// If the desired VPC is not already associated, do so
		if _, ok := associatedVpcs[v.VpcId]; !ok {
			r.log.V(1).Info("Associating VPC with Route53 Hosted Zone", "vpc", v.VpcId)

			if _, err := r.awsClient.CreateVPCAssociationAuthorization(ctx, resource.Status.HostedZoneId, v.VpcId, v.Region); err != nil {
				return err
			}

			// Use the provided override credentials for this specific vpcendpoint
			cfg, err := secrets.ParseAWSCredentialOverride(ctx, r.APIReader, v.Region, v.CredentialsSecretRef)
			if err != nil {
				return err
			}

			r.awsAssociatedVpcClient = aws_client.NewVpcAssociationClient(cfg)
			if _, err := r.awsAssociatedVpcClient.AssociateVPCWithHostedZone(ctx, resource.Status.HostedZoneId, v.VpcId, v.Region); err != nil {
				return err
			}
		}
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
		r.log.V(0).Info("Skipping Route53 Record", "error", err.Error())
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
				Message: fmt.Sprintf("Unknown error: %v", err),
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
				Message: fmt.Sprintf("Unknown error: %v", err),
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
