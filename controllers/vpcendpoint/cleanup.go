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

	route53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"github.com/openshift/aws-vpce-operator/pkg/dnses"
)

// cleanupAwsResources cleans up AWS resources associated with a VPC Endpoint.
func (r *VpcEndpointReconciler) cleanupAwsResources(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error {
	r.log.V(0).Info("Starting AWS resource cleanup",
		"vpcEndpoint", resource.Name,
		"namespace", resource.Namespace,
		"vpceId", resource.Status.VPCEndpointId,
		"securityGroupId", resource.Status.SecurityGroupId,
		"hostedZoneId", resource.Status.HostedZoneId,
		"resourceRecordSet", resource.Status.ResourceRecordSet,
	)
	r.Recorder.Eventf(resource, corev1.EventTypeNormal, "CleanupStarted",
		"Starting cleanup of AWS resources (vpceId=%s, sgId=%s, hzId=%s)",
		resource.Status.VPCEndpointId, resource.Status.SecurityGroupId, resource.Status.HostedZoneId)

	if meta.IsStatusConditionTrue(resource.Status.Conditions, avov1alpha2.AWSRoute53RecordCondition) {
		// Ensure .status.hostedZoneId is populated
		if err := r.validateR53PrivateHostedZone(ctx, resource); err != nil {
			return err
		}

		// HostedZoneId and resourceRecord are required if we want to clean up a ResourceRecordSet
		if resource.Status.HostedZoneId != "" && resource.Status.ResourceRecordSet != "" {
			resp, err := r.awsClient.GetHostedZone(ctx, resource.Status.HostedZoneId)
			if err != nil {
				return err
			}

			if resp.HostedZone != nil {
				listRRSResp, err := r.awsClient.ListResourceRecordSets(ctx, resource.Status.HostedZoneId)
				if err != nil {
					return err
				}

				// Delete all records in the hosted zone except the default SOA and NS records
				for _, resourceRecord := range listRRSResp.ResourceRecordSets {
					rr := resourceRecord
					switch *rr.Name {
					case fmt.Sprintf("%s.", resource.Status.ResourceRecordSet):
						r.log.V(0).Info("Deleting Route53 Hosted Zone Record", "name", *resourceRecord.Name, "type", resourceRecord.Type)
						if _, err := r.awsClient.DeleteResourceRecordSet(ctx, &rr, *resp.HostedZone.Id); err != nil {
							return err
						}
					default:
						continue
					}
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

			r.Recorder.Eventf(resource, corev1.EventTypeNormal, "Deleted", "Deleted Route53 record(s) in hosted zone %s", resource.Status.HostedZoneId)
		}
	}

	if resource.Status.HostedZoneId != "" {
		// Only delete a Route53 Private Hosted Zone if AVO created it
		if resource.Spec.CustomDns.Route53PrivateHostedZone.DomainName != "" || resource.Spec.CustomDns.Route53PrivateHostedZone.DomainNameRef != nil {
			// don't delete the zone if it's the cluster's private zone
			dnsConfig := &configv1.DNS{}
			err := r.Get(ctx, client.ObjectKey{Name: dnses.DefaultDnsesName}, dnsConfig)
			if err != nil {
				return err
			}

			// Safeguard against users supplying the cluster's domain name in a VPCE. We do not want to delete this
			// Route53 Hosted Zone in this case, even though the "correct" usage of the API would be to use
			// autoDiscoverPrivateHostedZone: true
			if resource.Spec.CustomDns.Route53PrivateHostedZone.DomainName != dnsConfig.Spec.BaseDomain {
				if _, err := r.awsClient.DeleteHostedZone(ctx, resource.Status.HostedZoneId); err != nil {
					var ae smithy.APIError
					if errors.As(err, &ae) {
						switch ae.ErrorCode() {
						case new(route53Types.NoSuchHostedZone).ErrorCode():
							r.log.V(0).Info("Route53 hosted zone already deleted", "hostedZoneId", resource.Status.HostedZoneId)
							r.Recorder.Eventf(resource, corev1.EventTypeNormal, "Deleted", "Route53 hosted zone already deleted: %s", resource.Status.HostedZoneId)
							resource.Status.HostedZoneId = ""
							if err := r.Status().Update(ctx, resource); err != nil {
								r.log.V(0).Error(err, "failed to update status")
								return err
							}
						case new(route53Types.HostedZoneNotEmpty).ErrorCode():
							// If there are other records in this hosted zone, delete them so that we can delete the
							// hosted zone that we own
							listRRSResp, err := r.awsClient.ListResourceRecordSets(ctx, resource.Status.HostedZoneId)
							if err != nil {
								return err
							}

							// Delete all records in the hosted zone except the default SOA and NS records
							for _, resourceRecord := range listRRSResp.ResourceRecordSets {
								rr := resourceRecord
								switch rr.Type {
								case route53Types.RRTypeNs:
									continue
								case route53Types.RRTypeSoa:
									continue
								default:
									r.log.V(0).Info("Deleting Route53 Hosted Zone Record", "name", *rr.Name, "type", rr.Type)
									if _, err := r.awsClient.DeleteResourceRecordSet(ctx, &rr, resource.Status.HostedZoneId); err != nil {
										return err
									}
								}
							}

							r.log.V(0).Info("Cleared remaining records from Route53 hosted zone, will retry zone deletion", "hostedZoneId", resource.Status.HostedZoneId)
							r.Recorder.Eventf(resource, corev1.EventTypeNormal, "CleanupProgress", "Cleared remaining records from hosted zone %s, will retry zone deletion", resource.Status.HostedZoneId)
						default:
							return err
						}
					} else {
						// Shouldn't happen
						return fmt.Errorf("unexpected error while deleting hosted zone: %v", err)
					}
				} else {
					r.log.V(0).Info("Deleted Route53 hosted zone", "hostedZoneId", resource.Status.HostedZoneId)
					r.Recorder.Eventf(resource, corev1.EventTypeNormal, "Deleted", "Deleted Route53 hosted zone: %s", resource.Status.HostedZoneId)
				}
			}
		}
	}

	if resource.Status.VPCEndpointId != "" {
		if err := r.cleanupMetrics(ctx, resource); err != nil {
			return err
		}

		vpceId := resource.Status.VPCEndpointId
		r.log.V(0).Info("Deleting VPC endpoint", "vpceId", vpceId)
		if _, err := r.awsClient.DeleteVPCEndpoint(ctx, vpceId); err != nil {
			var ae smithy.APIError
			if errors.As(err, &ae) {
				if ae.ErrorCode() == "InvalidVpcEndpoint.NotFound" {
					r.log.V(0).Info("VPC endpoint already deleted", "vpceId", vpceId)
					r.Recorder.Eventf(resource, corev1.EventTypeNormal, "Deleted", "VPC endpoint already deleted: %s", vpceId)
					resource.Status.VPCEndpointId = ""
				} else {
					return err
				}
			} else {
				// Shouldn't happen
				return fmt.Errorf("unexpected error while deleting VPC Endpoint: %v", err)
			}
		} else {
			r.Recorder.Eventf(resource, corev1.EventTypeNormal, "Deleted", "Deleted VPC endpoint: %s", vpceId)
		}

		resource.Status.Status = "deleting"
		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}
	}

	if resource.Status.SecurityGroupId != "" {
		sgId := resource.Status.SecurityGroupId
		r.log.V(0).Info("Deleting security group", "securityGroupId", sgId)
		if _, err := r.awsClient.DeleteSecurityGroup(ctx, sgId); err != nil {
			var ae smithy.APIError
			if errors.As(err, &ae) {
				if ae.ErrorCode() == "InvalidGroup.NotFound" {
					r.log.V(0).Info("Security group already deleted", "securityGroupId", sgId)
					r.Recorder.Eventf(resource, corev1.EventTypeNormal, "Deleted", "Security group already deleted: %s", sgId)
					resource.Status.SecurityGroupId = ""
					if err := r.Status().Update(ctx, resource); err != nil {
						r.log.V(0).Error(err, "failed to update status")
						return err
					}
				} else {
					return err
				}
			} else {
				// Shouldn't happen
				return fmt.Errorf("unexpected error while deleting security group: %v", err)
			}
		} else {
			r.Recorder.Eventf(resource, corev1.EventTypeNormal, "Deleted", "Deleted security group: %s", sgId)
		}
	}

	r.log.V(0).Info("AWS cleanup complete", "vpcEndpoint", resource.Name, "namespace", resource.Namespace)
	r.Recorder.Event(resource, corev1.EventTypeNormal, "CleanupComplete", "All AWS resources cleaned up successfully")
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
