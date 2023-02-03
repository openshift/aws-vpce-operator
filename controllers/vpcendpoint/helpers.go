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
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	route53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go"

	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
	"github.com/openshift/aws-vpce-operator/pkg/infrastructures"
	"github.com/openshift/aws-vpce-operator/pkg/secrets"
	"github.com/openshift/aws-vpce-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// parseClusterInfo fills in the clusterInfo struct values inside the VpcEndpointReconciler
// and gets a new AWS session if refreshAWSSession is true.
// Generally, refreshAWSSession is only set to false during testing to mock the AWS client.
func (r *VpcEndpointReconciler) parseClusterInfo(ctx context.Context, vpce *avov1alpha2.VpcEndpoint, refreshAWSSession bool) error {
	if err := validateVpcEndpointCR(vpce); err != nil {
		return err
	}

	r.clusterInfo = new(clusterInfo)

	// TODO: For HyperShift, it would be better if the infra name was the hosted cluster's infra name
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

	if vpce.Spec.Region != "" {
		r.clusterInfo.region = vpce.Spec.Region
		r.log.V(1).Info("Using specified region override", "region", vpce.Spec.Region)
	} else {
		region, err := infrastructures.GetAWSRegion(ctx, r.Client)
		if err != nil {
			return err
		}
		r.clusterInfo.region = region
		r.log.V(1).Info("Parsed region from infrastructure", "region", region)
	}

	if vpce.Spec.AWSCredentialOverrideRef != nil {
		// Use the provided override credentials for this specific vpcendpoint
		cfg, err := secrets.ParseAWSCredentialOverride(ctx, r.APIReader, r.clusterInfo.region, vpce.Spec.AWSCredentialOverrideRef)
		if err != nil {
			return err
		}
		r.awsClient = aws_client.NewAwsClient(cfg)
	} else {
		// Load the default AWS credentials that are available to the controller
		if refreshAWSSession {
			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(r.clusterInfo.region))
			if err != nil {
				return err
			}
			r.awsClient = aws_client.NewAwsClient(cfg)
		}
	}

	if len(vpce.Spec.Vpc.Ids) > 0 {
		vpcId, err := r.awsClient.SelectVPCForVPCEndpoint(ctx, vpce.Spec.Vpc.Ids...)
		if err != nil {
			return fmt.Errorf("failed to select a VPC to place a VPC Endpoint in: %w", err)
		}
		r.log.V(1).Info("Selecting vpc id", "vpcId", vpcId)
		r.clusterInfo.vpcId = vpcId
	} else if vpce.Spec.Vpc.AutoDiscoverSubnets {
		resp, err := r.awsClient.AutodiscoverPrivateSubnets(ctx, r.clusterInfo.clusterTag)
		if err != nil {
			return fmt.Errorf("unable to autodiscover subnets: %w", err)
		}

		subnets := make([]string, len(resp))
		for i := range resp {
			subnets[i] = *resp[i].SubnetId
		}

		vpcId, err := r.awsClient.GetVPCId(ctx, subnets)
		if err != nil {
			return err
		}
		r.clusterInfo.vpcId = vpcId
		r.log.V(1).Info("Found vpc id:", "vpcId", vpcId)
	} else {
		vpcId, err := r.awsClient.GetVPCId(ctx, vpce.Spec.Vpc.SubnetIds)
		if err != nil {
			return err
		}
		r.clusterInfo.vpcId = vpcId
		r.log.V(1).Info("Found vpc id:", "vpcId", vpcId)
	}

	return nil
}

// awsUnauthorizedOperationMetricHandler determines if an error is an AWS UnauthorizedOperation or AccessDenied and
// increments the aws_vpce_operator_unauthorized_operation_total metric accordingly
func awsUnauthorizedOperationMetricHandler(err error) {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		if ae.ErrorCode() == "UnauthorizedOperation" || ae.ErrorCode() == "AccessDenied" {
			var oe *smithy.OperationError
			if errors.As(err, &oe) {
				awsUnauthorizedOperation.WithLabelValues(fmt.Sprintf("%s:%s", strings.ToLower(strings.ReplaceAll(oe.Service(), " ", "")), oe.Operation())).Inc()
			} else {
				awsUnauthorizedOperation.WithLabelValues("Unknown").Inc()
			}
		}
	}
}

// findOrCreateSecurityGroup queries AWS and returns the Security Group for the provided CR and updates its status.
// It first tries to use the Security Group ID that may be in the resource's status and falls back on
// searching for the VPC Endpoint by tags in case the status is lost. If it still cannot find a Security Group,
// it gets created.
func (r *VpcEndpointReconciler) findOrCreateSecurityGroup(ctx context.Context, resource *avov1alpha2.VpcEndpoint) (*ec2Types.SecurityGroup, error) {
	var sg *ec2Types.SecurityGroup

	r.log.V(1).Info("Searching for security group by ID", "id", resource.Status.SecurityGroupId)
	resp, err := r.awsClient.FilterSecurityGroupById(ctx, resource.Status.SecurityGroupId)
	if err != nil {
		return nil, err
	}

	// If there's no security group returned by ID, look for one by tag
	// first, generate the security group name to search tags or use it later to create it
	if resp == nil || len(resp.SecurityGroups) == 0 {
		sgName, err := util.GenerateSecurityGroupName(r.clusterInfo.infraName, resource.Name)
		if err != nil {
			return nil, err
		}

		r.log.V(1).Info("Searching for security group by tags")
		resp, err = r.awsClient.FilterSecurityGroupByDefaultTags(ctx, r.clusterInfo.infraName, sgName)
		if err != nil {
			return nil, err
		}

		// If there are still no security groups found, it needs to be created
		if resp == nil || len(resp.SecurityGroups) == 0 {
			createResp, err := r.awsClient.CreateSecurityGroup(ctx, sgName, r.clusterInfo.vpcId, r.clusterInfo.clusterTag)
			if err != nil {
				return nil, err
			}

			r.Recorder.Eventf(resource, corev1.EventTypeNormal, "Created", "Created security group: %s", *createResp.GroupId)
			r.log.V(0).Info("Created security group", "id", *createResp.GroupId)

			// Unfortunately CreateSecurityGroup doesn't return an *ec2.SecurityGroup so just return an error to
			// put this back in the work queue after recording the security group id
			resource.Status.SecurityGroupId = *createResp.GroupId
			if err := r.Status().Update(ctx, resource); err != nil {
				return nil, errors.New("failed to update status")
			}

			return nil, errors.New("initial security group creation, reconciling again to configure")
		} else {
			sg = &resp.SecurityGroups[0]
		}
	} else {
		sg = &resp.SecurityGroups[0]
	}

	if sg == nil {
		return nil, errors.New("unexpectedly got a nil security group response from AWS")
	}

	r.log.V(1).Info("Found security group", "id", *resp.SecurityGroups[0].GroupId)
	resource.Status.SecurityGroupId = *sg.GroupId
	if err := r.Status().Update(ctx, resource); err != nil {
		return nil, errors.New("failed to update status")
	}

	return sg, nil
}

// createMissingSecurityGroupTags ensures the expected AWS tags exist on a VpcEndpoint CR's Security Group.
// It will not delete any extra tags and only create missing ones.
func (r *VpcEndpointReconciler) createMissingSecurityGroupTags(ctx context.Context, sg *ec2Types.SecurityGroup, resource *avov1alpha2.VpcEndpoint) error {
	sgName, err := util.GenerateSecurityGroupName(r.clusterInfo.infraName, resource.Name)
	if err != nil {
		return fmt.Errorf("failed to generate security group name: %v", err)
	}

	defaultTagsMap, err := util.GenerateAwsTagsAsMap(sgName, r.clusterInfo.clusterTag)
	if err != nil {
		return err
	}

	// Fix tags if any are missing
	if !tagsContains(sg.Tags, defaultTagsMap) {
		r.log.V(1).Info("Adding missing security group tags")
		defaultTags, err := util.GenerateAwsTags(sgName, r.clusterInfo.clusterTag)
		if err != nil {
			return fmt.Errorf("failed to generate expected tags: %v", err)
		}
		if _, err := r.awsClient.CreateTags(ctx, &ec2.CreateTagsInput{
			Resources: []string{*sg.GroupId},
			Tags:      defaultTags,
		}); err != nil {
			return fmt.Errorf("failed to create tags: %w", err)
		}

		r.Recorder.Eventf(resource, corev1.EventTypeNormal, "Updated", "Updated security group tags: %s", *sg.GroupId)
	}

	return nil
}

// generateMissingSecurityGroupRules ensures that the cluster's worker and master security groups are allowed ingresses
// to the VPC Endpoint security group as well as and other configured rules from the CR.
// It will not remove an extra security group rules and only create missing ones.
func (r *VpcEndpointReconciler) generateMissingSecurityGroupRules(ctx context.Context, sg *ec2Types.SecurityGroup, resource *avov1alpha2.VpcEndpoint) (
	*ec2.AuthorizeSecurityGroupIngressInput, *ec2.AuthorizeSecurityGroupEgressInput, error) {
	if sg == nil || resource == nil {
		return nil, nil, fmt.Errorf("security group and resource must not be nil")
	}

	rulesResp, err := r.awsClient.DescribeSecurityGroupRules(ctx, *sg.GroupId)
	if err != nil {
		return nil, nil, err
	}

	sourceSgResp, err := r.awsClient.FilterClusterNodeSecurityGroupsByDefaultTags(ctx, r.clusterInfo.infraName)
	if err != nil {
		return nil, nil, err
	}

	sourceSgIds := make([]*string, len(sourceSgResp.SecurityGroups))
	for i := range sourceSgResp.SecurityGroups {
		sourceSgIds[i] = sourceSgResp.SecurityGroups[i].GroupId
	}

	// Ensure ingress/egress rules
	var (
		ingressRules []ec2Types.IpPermission
		egressRules  []ec2Types.IpPermission
	)

	for i := range resource.Spec.SecurityGroup.IngressRules {
		for _, sourceSgId := range sourceSgIds {
			create := true
			for _, rule := range rulesResp.SecurityGroupRules {
				// If we find a rule with the correct protocol, fromPort, and toPort, check the source security group
				if *rule.IpProtocol == resource.Spec.SecurityGroup.IngressRules[i].Protocol &&
					*rule.FromPort == resource.Spec.SecurityGroup.IngressRules[i].FromPort &&
					*rule.ToPort == resource.Spec.SecurityGroup.IngressRules[i].ToPort &&
					*rule.ReferencedGroupInfo.GroupId == *sourceSgId {
					create = false
					break
				}
			}

			if create {
				ingressRules = append(ingressRules, ec2Types.IpPermission{
					IpProtocol: aws.String(resource.Spec.SecurityGroup.IngressRules[i].Protocol),
					FromPort:   aws.Int32(resource.Spec.SecurityGroup.IngressRules[i].FromPort),
					ToPort:     aws.Int32(resource.Spec.SecurityGroup.IngressRules[i].ToPort),
					UserIdGroupPairs: []ec2Types.UserIdGroupPair{
						{
							GroupId: sourceSgId,
						},
					},
				})
			}
		}
	}

	for i := range resource.Spec.SecurityGroup.EgressRules {
		for _, sourceSgId := range sourceSgIds {
			create := true
			for _, rule := range rulesResp.SecurityGroupRules {
				if *rule.IpProtocol == resource.Spec.SecurityGroup.EgressRules[i].Protocol &&
					*rule.FromPort == resource.Spec.SecurityGroup.EgressRules[i].FromPort &&
					*rule.ToPort == resource.Spec.SecurityGroup.EgressRules[i].ToPort &&
					*rule.ReferencedGroupInfo.GroupId == *sourceSgId {
					create = false
					break
				}
			}

			if create {
				egressRules = append(egressRules, ec2Types.IpPermission{
					IpProtocol: aws.String(resource.Spec.SecurityGroup.EgressRules[i].Protocol),
					FromPort:   aws.Int32(resource.Spec.SecurityGroup.EgressRules[i].FromPort),
					ToPort:     aws.Int32(resource.Spec.SecurityGroup.EgressRules[i].ToPort),
					UserIdGroupPairs: []ec2Types.UserIdGroupPair{
						{
							GroupId: sourceSgId,
						},
					},
				})
			}
		}
	}

	if len(ingressRules) > 0 {
		r.log.V(1).Info("Need to create ingress rules", "ingressRules", ingressRules)
	}
	if len(egressRules) > 0 {
		r.log.V(1).Info("Need to create egress rules", "egressRules", egressRules)
	}

	ingressInput := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       sg.GroupId,
		IpPermissions: ingressRules,
		TagSpecifications: []ec2Types.TagSpecification{
			{
				ResourceType: ec2Types.ResourceTypeSecurityGroupRule,
				Tags: []ec2Types.Tag{
					{
						Key:   aws.String(r.clusterInfo.clusterTag),
						Value: aws.String(""),
					},
					{
						Key:   aws.String(util.OperatorTagKey),
						Value: aws.String(util.OperatorTagValue),
					},
				},
			},
		},
	}

	egressInput := &ec2.AuthorizeSecurityGroupEgressInput{
		GroupId:       sg.GroupId,
		IpPermissions: egressRules,
		TagSpecifications: []ec2Types.TagSpecification{
			{
				ResourceType: ec2Types.ResourceTypeSecurityGroupRule,
				Tags: []ec2Types.Tag{
					{
						Key:   aws.String(r.clusterInfo.clusterTag),
						Value: aws.String(""),
					},
					{
						Key:   aws.String(util.OperatorTagKey),
						Value: aws.String(util.OperatorTagValue),
					},
				},
			},
		},
	}

	return ingressInput, egressInput, nil
}

// findOrCreateVpcEndpoint queries AWS and returns the VPC Endpoint for the provided CR and updates its status.
// It first tries to use the VPC Endpoint ID that may be in the resource's status and falls back on
// searching for the VPC Endpoint by tags in case the status is lost. If it still cannot find a VPC Endpoint,
// it gets created.
func (r *VpcEndpointReconciler) findOrCreateVpcEndpoint(ctx context.Context, resource *avov1alpha2.VpcEndpoint) (*ec2Types.VpcEndpoint, error) {
	var vpce *ec2Types.VpcEndpoint

	r.log.V(1).Info("Searching for VPC Endpoint by ID", "id", resource.Status.VPCEndpointId)
	resp, err := r.awsClient.DescribeSingleVPCEndpointById(ctx, resource.Status.VPCEndpointId)
	if err != nil {
		return nil, err
	}

	// If there's no VPC Endpoint returned by ID, look for one by tag
	// first, generate the VPC Endpoint name to search tags or use it later to create it
	if resp == nil || len(resp.VpcEndpoints) == 0 {
		vpceName, err := util.GenerateVPCEndpointName(r.clusterInfo.infraName, resource.Name)
		if err != nil {
			return nil, err
		}

		r.log.V(1).Info("Searching for VPC Endpoint by tags")
		resp, err = r.awsClient.FilterVPCEndpointByDefaultTags(ctx, r.clusterInfo.clusterTag, vpceName)
		if err != nil {
			return nil, err
		}

		// If there are still no VPC Endpoints found, it needs to be created
		if resp == nil || len(resp.VpcEndpoints) == 0 {

			creationResp, err := r.awsClient.CreateDefaultInterfaceVPCEndpoint(ctx, vpceName, r.clusterInfo.vpcId, resource.Spec.ServiceName, r.clusterInfo.clusterTag)
			if err != nil {
				return nil, fmt.Errorf("failed to create vpc endpoint: %w", err)
			}

			vpce = creationResp.VpcEndpoint
			r.log.V(0).Info("Created VPC endpoint:", "vpcEndpoint", *vpce.VpcEndpointId)
			r.Recorder.Eventf(resource, corev1.EventTypeNormal, "Created", "Created VPC endpoint: %s", *vpce.VpcEndpointId)
		} else {
			vpce = &resp.VpcEndpoints[0]
		}
	} else {
		// There can only be one match returned by DescribeSingleVpcEndpointById
		vpce = &resp.VpcEndpoints[0]
	}

	if vpce == nil {
		return nil, errors.New("unexpectedly got a nil vpce response from AWS")
	}

	resource.Status.VPCEndpointId = *vpce.VpcEndpointId
	resource.Status.Status = string(vpce.State)
	if err := r.Status().Update(ctx, resource); err != nil {
		return nil, fmt.Errorf("failed to update status: %w", err)
	}

	return vpce, nil
}

// ensureVpcEndpointSubnets ensures that the subnets attached to the VPC Endpoint are the expected subnet ids
func (r *VpcEndpointReconciler) ensureVpcEndpointSubnets(ctx context.Context, vpce *ec2Types.VpcEndpoint, resource *avov1alpha2.VpcEndpoint) error {
	var (
		subnetsToAdd, subnetsToRemove []string
	)

	if resource.Spec.Vpc.AutoDiscoverSubnets {
		var discoveredSubnets []ec2Types.Subnet
		if len(resource.Spec.Vpc.Ids) > 0 {
			// Do not expect private subnets to have the cluster id when load balancing vpc ids
			privateSubnets, err := r.awsClient.AutodiscoverPrivateSubnets(ctx, "")
			if err != nil {
				return err
			}
			r.log.V(1).Info("Discovered private subnet(s):", "subnets", privateSubnets)
			discoveredSubnets = privateSubnets
		} else {
			if r.clusterInfo == nil || r.clusterInfo.clusterTag == "" {
				return fmt.Errorf("unable to parse cluster tag: %v", r.clusterInfo)
			}

			privateSubnets, err := r.awsClient.AutodiscoverPrivateSubnets(ctx, r.clusterInfo.clusterTag)
			if err != nil {
				return err
			}
			r.log.V(1).Info("Discovered private subnet(s):", "subnets", privateSubnets)
			discoveredSubnets = privateSubnets
		}

		// When auto-discovering the cluster's private subnet ids, only subnets supported by the VPC Endpoint
		// Service should be attached
		allowedAZs, err := r.awsClient.GetVpcEndpointServiceAZs(ctx, resource.Spec.ServiceName)
		if err != nil {
			return err
		}

		var expectedSubnetIds []string
		for _, subnet := range discoveredSubnets {
			for _, az := range allowedAZs {
				if *subnet.AvailabilityZone == az {
					expectedSubnetIds = append(expectedSubnetIds, *subnet.SubnetId)
					break
				}
			}
		}

		r.log.V(1).Info("Private subnet(s) in availability zones supported by the VPC Endpoint Service:", "subnets", expectedSubnetIds, "serviceName", resource.Spec.ServiceName)
		subnetsToAdd, subnetsToRemove = util.StringSliceTwoWayDiff(vpce.SubnetIds, expectedSubnetIds)
	} else {
		// When subnet ids are specified, use exactly those subnets
		subnetsToAdd, subnetsToRemove = util.StringSliceTwoWayDiff(vpce.SubnetIds, resource.Spec.Vpc.SubnetIds)
	}

	// Removing subnets first before adding to avoid
	// DuplicateSubnetsInSameZone: Found another VPC endpoint subnet in the availability zone of <existing subnet>
	if len(subnetsToRemove) > 0 {
		r.log.V(1).Info("Removing subnet(s) from VPC Endpoint", "subnetsToRemove", subnetsToRemove)
		if _, err := r.awsClient.ModifyVpcEndpoint(ctx, &ec2.ModifyVpcEndpointInput{
			RemoveSubnetIds: subnetsToRemove,
			VpcEndpointId:   vpce.VpcEndpointId,
		}); err != nil {
			return fmt.Errorf("failed to remove subnets: %v with error: %w", subnetsToRemove, err)
		}
	}

	if len(subnetsToAdd) > 0 {
		r.log.V(1).Info("Adding subnet(s) to VPC Endpoint", "subnetsToAdd", subnetsToAdd)
		if _, err := r.awsClient.ModifyVpcEndpoint(ctx, &ec2.ModifyVpcEndpointInput{
			AddSubnetIds:  subnetsToAdd,
			VpcEndpointId: vpce.VpcEndpointId,
		}); err != nil {
			return fmt.Errorf("failed to add subnets: %v with error: %w", subnetsToAdd, err)
		}
	}

	return nil
}

// ensureVpcEndpointSecurityGroups ensures that the security group associated with the VPC Endpoint
// is only the expected one.
func (r *VpcEndpointReconciler) ensureVpcEndpointSecurityGroups(ctx context.Context, vpce *ec2Types.VpcEndpoint, resource *avov1alpha2.VpcEndpoint) error {
	sgToAdd, sgToRemove, err := r.diffVpcEndpointSecurityGroups(vpce, resource)
	if err != nil {
		return err
	}

	if len(sgToAdd) > 0 {
		r.log.V(1).Info("Adding security group(s) to VPC Endpoint", "sgToAdd", sgToAdd)
		if _, err := r.awsClient.ModifyVpcEndpoint(ctx, &ec2.ModifyVpcEndpointInput{
			AddSecurityGroupIds: sgToAdd,
			VpcEndpointId:       vpce.VpcEndpointId,
		}); err != nil {
			return err
		}
	}

	if len(sgToRemove) > 0 {
		r.log.V(1).Info("Removing security group(s) from VPC Endpoint", "sgToRemove", sgToRemove)
		if _, err := r.awsClient.ModifyVpcEndpoint(ctx, &ec2.ModifyVpcEndpointInput{
			RemoveSecurityGroupIds: sgToRemove,
			VpcEndpointId:          vpce.VpcEndpointId,
		}); err != nil {
			return err
		}
	}

	return nil
}

// diffVpcEndpointSecurityGroups compares the security groups associated with the VPC Endpoint with
// the security group ID recorded in the resource's status, returning security groups that need to be added
// and security groups that need to be removed from the VPC Endpoint.
func (r *VpcEndpointReconciler) diffVpcEndpointSecurityGroups(vpce *ec2Types.VpcEndpoint, resource *avov1alpha2.VpcEndpoint) ([]string, []string, error) {
	vpceSgIds := make([]string, len(vpce.Groups))
	for i := range vpce.Groups {
		vpceSgIds[i] = *vpce.Groups[i].GroupId
	}

	sgToAdd, sgToRemove := util.StringSliceTwoWayDiff(
		vpceSgIds,
		[]string{resource.Status.SecurityGroupId},
	)

	return sgToAdd, sgToRemove, nil
}

// findOrCreatePrivateHostedZone ensures the existence of a Route53 Private Hosted Zone given a custom domain name
func (r *VpcEndpointReconciler) findOrCreatePrivateHostedZone(ctx context.Context, resource *avov1alpha2.VpcEndpoint) error {
	if resource == nil {
		// Should never happen
		return errors.New("resource must be specified")
	}

	if resource.Spec.CustomDns.Route53PrivateHostedZone.DomainName != "" {
		r.log.V(1).Info("Searching for Route 53 Private Hosted Zone", "vpc", r.clusterInfo.vpcId, "region", r.clusterInfo.region)
		// TODO: Unlikely, but would be nice to handle pagination
		resp, err := r.awsClient.ListHostedZonesByVPC(ctx, r.clusterInfo.vpcId, r.clusterInfo.region)
		if err != nil {
			return err
		}

		for _, hz := range resp.HostedZoneSummaries {
			// If we find a matching hosted zone, update status
			if strings.TrimRight(*hz.Name, ".") == resource.Spec.CustomDns.Route53PrivateHostedZone.DomainName {
				if resource.Status.HostedZoneId != *hz.HostedZoneId {
					resource.Status.HostedZoneId = *hz.HostedZoneId
					if err := r.Status().Update(ctx, resource); err != nil {
						r.log.V(0).Error(err, "failed to update status")
						return err
					}
				}

				return nil
			}
		}

		// Otherwise, create one
		createResp, err := r.awsClient.CreateHostedZone(ctx, resource.Spec.CustomDns.Route53PrivateHostedZone.DomainName, r.clusterInfo.vpcId, r.clusterInfo.region)
		if err != nil {
			return fmt.Errorf("failed to create hosted zone: %w", err)
		}
		r.log.V(0).Info("Created Route 53 Private Hosted Zone", "id", *createResp.HostedZone.Id)
		r.Recorder.Eventf(resource, corev1.EventTypeNormal, "Created", "Created Private Hosted Zone: %s", *createResp.HostedZone.Id)

		resource.Status.HostedZoneId = *createResp.HostedZone.Id
		if err := r.Status().Update(ctx, resource); err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}
	}

	return nil
}

// generateRoute53Record generates the expected Route53 Record for a provided VpcEndpoint CR
func (r *VpcEndpointReconciler) generateRoute53Record(ctx context.Context, resource *avov1alpha2.VpcEndpoint) (*route53Types.ResourceRecord, error) {
	if resource.Status.VPCEndpointId == "" {
		return nil, fmt.Errorf("VPCEndpointID status is missing")
	}

	vpceResp, err := r.awsClient.DescribeSingleVPCEndpointById(ctx, resource.Status.VPCEndpointId)
	if err != nil {
		return nil, err
	}

	// VPCEndpoint doesn't exist anymore for some reason
	if vpceResp == nil || len(vpceResp.VpcEndpoints) == 0 {
		return nil, nil
	}

	// DNSEntries won't be populated until the state is available
	if string(vpceResp.VpcEndpoints[0].State) != "available" {
		return nil, fmt.Errorf("VPCEndpoint is not in the available state")
	}

	if len(vpceResp.VpcEndpoints[0].DnsEntries) == 0 {
		return nil, fmt.Errorf("VPCEndpoint has no DNS entries")
	}

	return &route53Types.ResourceRecord{
		Value: vpceResp.VpcEndpoints[0].DnsEntries[0].DnsName,
	}, nil
}

// generateExternalNameService generates the expected ExternalName service for a VpcEndpoint CustomResource
func (r *VpcEndpointReconciler) generateExternalNameService(resource *avov1alpha2.VpcEndpoint) (*corev1.Service, error) {
	if resource.Status.ResourceRecordSet == "" {
		// Should only happen when a Route53 Hosted Zone Record has not been created yet
		return nil, fmt.Errorf("cannot generate ExternalName service for %s/%s: .status.resourceRecordSet is empty", resource.Namespace, resource.Name)
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resource.Spec.CustomDns.Route53PrivateHostedZone.Record.ExternalNameService.Name,
			Namespace: resource.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.%s", resource.Spec.CustomDns.Route53PrivateHostedZone.Record.Hostname, resource.Status.ResourceRecordSet),
		},
	}

	if err := controllerutil.SetControllerReference(resource, svc, r.Scheme); err != nil {
		return nil, err
	}

	return svc, nil
}

// tagsContains returns true if the all the tags in tagsToCheck exist in tags
func tagsContains(tags []ec2Types.Tag, tagsToCheck map[string]string) bool {
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

// createMissingPrivateZoneTags will compare existing tags to the required set and apply if missing
func (r *VpcEndpointReconciler) createMissingPrivateZoneTags(ctx context.Context, id string) error {
	// Find existing tags
	listTagsOut, err := r.awsClient.FetchPrivateZoneTags(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to list zone's tags %w", err)
	}

	// Generate default tags to compare against
	generatedDefaultTagInput, err := r.awsClient.GenerateDefaultTagsForHostedZoneInput(id, r.clusterInfo.clusterTag)
	if err != nil {
		return fmt.Errorf("failed to generate hosted zone's default tags %w", err)
	}

	actualTagsMap := map[string]string{}
	for _, tag := range listTagsOut.ResourceTagSet.Tags {
		actualTagsMap[*tag.Key] = *tag.Value
	}

	for _, tag := range generatedDefaultTagInput.AddTags {
		v, ok := actualTagsMap[*tag.Key]
		if !ok || v != *tag.Value {
			if _, err := r.awsClient.ChangeTagsForResource(ctx, generatedDefaultTagInput); err != nil {
				return fmt.Errorf("failed tag hosted zone with default tags %w", err)
			}
		}
	}

	return nil
}
