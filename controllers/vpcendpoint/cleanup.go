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

	"github.com/aws/aws-sdk-go-v2/aws"
	route53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go"
	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
)

const (
	clusterDNSName = "cluster"
)

// cleanupAwsResources cleans up AWS resources associated with a VPC Endpoint.
func (r *VpcEndpointReconciler) cleanupAwsResources(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error {
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
		}
	}

	if resource.Status.HostedZoneId != "" {
		// Only delete a Route53 Private Hosted Zone if AVO created it
		if resource.Spec.CustomDns.Route53PrivateHostedZone.DomainName != "" || resource.Spec.CustomDns.Route53PrivateHostedZone.DomainNameRef != nil {
			// don't delete the zone if it's the cluster's private zone
			dnsConfig := &configv1.DNS{}
			err := r.Client.Get(ctx, client.ObjectKey{Name: clusterDNSName}, dnsConfig)
			if err != nil {
				return err
			}

			if resource.Spec.CustomDns.Route53PrivateHostedZone.DomainName == dnsConfig.Spec.BaseDomain {
				// only delete the record from the spec
				rrSet := &route53Types.ResourceRecordSet{
					Name: aws.String(resource.Spec.CustomDns.Route53PrivateHostedZone.Record.Hostname),
					Type: route53Types.RRTypeA,
				}

				_, err := r.awsClient.DeleteResourceRecordSet(ctx, rrSet, resource.Spec.CustomDns.Route53PrivateHostedZone.Id)
				if err != nil {
					return err
				}
			} else if count, _ := r.domainCount(resource.Spec.CustomDns.Route53PrivateHostedZone.DomainName); count >= 2 {
				// only clean up the record if another vpce is using the same zone
				rrSet := &route53Types.ResourceRecordSet{
					Name: aws.String(resource.Spec.CustomDns.Route53PrivateHostedZone.Record.Hostname),
					Type: route53Types.RRTypeA,
				}

				_, err := r.awsClient.DeleteResourceRecordSet(ctx, rrSet, resource.Spec.CustomDns.Route53PrivateHostedZone.Id)
				if err != nil {
					return err
				}
			} else if _, err := r.awsClient.DeleteHostedZone(ctx, resource.Status.HostedZoneId); err != nil {
				var ae smithy.APIError
				if errors.As(err, &ae) {
					switch ae.ErrorCode() {
					case new(route53Types.NoSuchHostedZone).ErrorCode():
						// If there's no such hosted zone, then it's already been deleted
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
					default:
						return err
					}
				} else {
					// Shouldn't happen
					return fmt.Errorf("unexpected error while deleting hosted zone: %v", err)
				}
			}
		}
	}

	if resource.Status.VPCEndpointId != "" {
		if err := r.cleanupMetrics(ctx, resource); err != nil {
			return err
		}

		r.log.V(0).Info("Deleting AWS resources", "VpcEndpoint", resource.Status.VPCEndpointId)
		if _, err := r.awsClient.DeleteVPCEndpoint(ctx, resource.Status.VPCEndpointId); err != nil {
			var ae smithy.APIError
			if errors.As(err, &ae) {
				if ae.ErrorCode() == "InvalidVpcEndpoint.NotFound" {
					resource.Status.VPCEndpointId = ""
				} else {
					return err
				}
			} else {
				// Shouldn't happen
				return fmt.Errorf("unexpected error while deleting VPC Endpoint: %v", err)
			}
		}

		resource.Status.Status = "deleting"
		if err := r.Status().Update(ctx, resource); err != nil {
			r.log.V(0).Error(err, "failed to update status")
			return err
		}
	}

	if resource.Status.SecurityGroupId != "" {
		r.log.V(0).Info("Deleting AWS resources", "SecurityGroup", resource.Status.SecurityGroupId)
		if _, err := r.awsClient.DeleteSecurityGroup(ctx, resource.Status.SecurityGroupId); err != nil {
			var ae smithy.APIError
			if errors.As(err, &ae) {
				if ae.ErrorCode() == "InvalidGroup.NotFound" {
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

// domainCount returns the number of VPCEs using a given custom DNS domain
func (r *VpcEndpointReconciler) domainCount(domain string) (count int, err error) {
	count = 0

	// get list of all VPCE resources to see if any others are using the zone we're tryign to clean up
	vpceList := &avov1alpha2.VpcEndpointList{}
	err = r.Client.List(context.TODO(), vpceList, &client.ListOptions{Namespace: ""})
	if err != nil {
		return 0, err
	}

	for _, v := range vpceList.Items {
		if v.Spec.CustomDns.Route53PrivateHostedZone.DomainName == domain {
			count++
		}
	}

	return
}
