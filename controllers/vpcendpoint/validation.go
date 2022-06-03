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
	"github.com/aws/aws-sdk-go/service/ec2"
	psov1alpha1 "github.com/openshift/aws-vpce-operator/api/v1alpha1"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
	"github.com/openshift/aws-vpce-operator/pkg/util"
)

// TagsContains returns true if the all the tags in tagsToCheck exist in tags
func TagsContains(tags []*ec2.Tag, tagsToCheck map[string]string) bool {
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

// validateSecurityGroup checks a security group against what's expected, returning an error if there are differences.
// Security groups can't be updated-in-place, so a new one will need to be created before deleting this existing one.
// TODO: Split out a ReconcileSecurityGroupRule function?
func (r *VpcEndpointReconciler) validateSecurityGroup(ctx context.Context, client *aws_client.AWSClient, resource *psov1alpha1.VpcEndpoint) (*string, error) {
	if resource == nil {
		// Should never happen
		return nil, fmt.Errorf("resource must be specified")
	}

	r.Log.V(1).Info("Searching for security group by ID", "id", resource.Status.SecurityGroupId)
	resp, err := client.FilterSecurityGroupById(resource.Status.SecurityGroupId)
	if err != nil {
		return nil, err
	}

	// If there's no security group returned by ID, look for one by tag
	if resp == nil || len(resp.SecurityGroups) == 0 {
		r.Log.V(1).Info("Searching for security group by tags")
		resp, err = client.FilterSecurityGroupByDefaultTags(r.InfraName)
		if err != nil {
			return nil, err
		}

		// If there are still no security groups found, it needs to be created
		if resp == nil || len(resp.SecurityGroups) == 0 {
			sgName, err := util.GenerateSecurityGroupName(r.InfraName, resource.Name)
			if err != nil {
				return nil, err
			}

			createResp, err := client.CreateSecurityGroup(sgName, r.VpcId, r.ClusterTag)
			if err != nil {
				return nil, err
			}

			resource.Status.SecurityGroupId = *createResp.GroupId
			if err := r.Status().Update(ctx, resource); err != nil {
				r.Log.V(0).Error(err, "Failed to update Security Group status")
				return nil, err
			}

			return createResp.GroupId, fmt.Errorf("created security group, configuring in next reconcile loop")
		}
	}

	sg := resp.SecurityGroups[0]

	defaultTags := map[string]string{
		r.ClusterTag:        "",
		util.OperatorTagKey: util.OperatorTagValue,
	}

	// Fix tags if any are missing
	if TagsContains(sg.Tags, defaultTags) == false {
		r.Log.V(1).Info("Adding missing security group tags")
		_, err := client.EC2Client.CreateTags(&ec2.CreateTagsInput{
			Resources: []*string{sg.GroupId},
			Tags: []*ec2.Tag{
				{
					Key:   aws.String(r.ClusterTag),
					Value: aws.String(""),
				},
				{
					Key:   aws.String(util.OperatorTagKey),
					Value: aws.String(util.OperatorTagValue),
				},
			},
		})

		if err != nil {
			return nil, err
		}
	}

	// TODO: Break out DescribeSecurityGroupRules into a separate function
	rulesResp, err := client.EC2Client.DescribeSecurityGroupRules(&ec2.DescribeSecurityGroupRulesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("group-id"),
				Values: []*string{sg.GroupId},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	sourceSgResp, err := client.FilterClusterNodeSecurityGroupsByDefaultTags(r.InfraName)
	if err != nil {
		return nil, err
	}

	sourceSgIds := make([]*string, len(sourceSgResp.SecurityGroups))
	for i := range sourceSgResp.SecurityGroups {
		sourceSgIds[i] = sourceSgResp.SecurityGroups[i].GroupId
	}

	// TODO: Break out AuthorizeSecurityGroupIngress and AuthorizeSecurityGroupEgress into a function
	// Ensure ingress/egress rules
	var (
		ingressRules []*ec2.IpPermission
		egressRules  []*ec2.IpPermission
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
				ingressRules = append(ingressRules, &ec2.IpPermission{
					IpProtocol: aws.String(resource.Spec.SecurityGroup.IngressRules[i].Protocol),
					FromPort:   aws.Int64(resource.Spec.SecurityGroup.IngressRules[i].FromPort),
					ToPort:     aws.Int64(resource.Spec.SecurityGroup.IngressRules[i].ToPort),
					UserIdGroupPairs: []*ec2.UserIdGroupPair{
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
				egressRules = append(egressRules, &ec2.IpPermission{
					IpProtocol: aws.String(resource.Spec.SecurityGroup.EgressRules[i].Protocol),
					FromPort:   aws.Int64(resource.Spec.SecurityGroup.EgressRules[i].FromPort),
					ToPort:     aws.Int64(resource.Spec.SecurityGroup.EgressRules[i].ToPort),
					UserIdGroupPairs: []*ec2.UserIdGroupPair{
						{
							GroupId: sourceSgId,
						},
					},
				})
			}
		}
	}

	if len(ingressRules) > 0 {
		r.Log.V(1).Info("Need to create ingress rules", "ingressRules", ingressRules)
	}
	if len(egressRules) > 0 {
		r.Log.V(1).Info("Need to create egress rules", "egressRules", egressRules)
	}

	ingressInput := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       sg.GroupId,
		IpPermissions: ingressRules,
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("security-group-rule"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String(r.ClusterTag),
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
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("security-group-rule"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String(r.ClusterTag),
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

	// Not idempotent
	if _, err := client.AuthorizeSecurityGroupRules(ingressInput, egressInput); err != nil {
		return nil, err
	}

	return sg.GroupId, nil
}

// validateVPCEndpoint checks a VPC endpoint with what's expected and reconciles their state
// returning an error if it cannot do so.
func (r *VpcEndpointReconciler) validateVPCEndpoint(ctx context.Context, client *aws_client.AWSClient, resource *psov1alpha1.VpcEndpoint) (*string, error) {
	if resource == nil {
		// Should never happen
		return nil, fmt.Errorf("resource must be specified")
	}

	var vpce *ec2.VpcEndpoint

	r.Log.V(1).Info("Searching for VPC Endpoint by ID", "id", resource.Status.VPCEndpointId)
	resp, err := client.DescribeSingleVPCEndpointById(resource.Status.VPCEndpointId)
	if err != nil {
		return nil, err
	}

	// If there's no VPC Endpoint returned by ID, look for one by tag
	if resp == nil || len(resp.VpcEndpoints) == 0 {
		r.Log.V(1).Info("Searching for VPC Endpoint by tags")
		resp, err = client.FilterVPCEndpointByDefaultTags(r.ClusterTag)
		if err != nil {
			return nil, err
		}

		// If there are still no VPC Endpoints found, it needs to be created
		if resp == nil || len(resp.VpcEndpoints) == 0 {
			vpceName, err := util.GenerateVPCEndpointName(r.InfraName, resource.Name)
			if err != nil {
				return nil, err
			}
			creationResp, err := r.AWSClient.CreateDefaultInterfaceVPCEndpoint(vpceName, r.VpcId, resource.Spec.ServiceName, r.ClusterTag)
			if err != nil {
				return nil, fmt.Errorf("failed to create vpc endpoint: %v", err)
			}

			vpce = creationResp.VpcEndpoint
			r.Log.V(0).Info("Created vpc endpoint:", "vpcEndpoint", *vpce.VpcEndpointId)
		} else {
			vpce = resp.VpcEndpoints[0]
		}
	} else {
		vpce = resp.VpcEndpoints[0]
	}

	resource.Status.VPCEndpointId = *vpce.VpcEndpointId
	resource.Status.Status = *vpce.State

	if err := r.Status().Update(ctx, resource); err != nil {
		r.Log.V(0).Error(err, "Failed to update VPC Endpoint status")
		return nil, err
	}

	switch *vpce.State {
	case "pending", "pendingAcceptance":
		r.Log.V(0).Info("VPC Endpoint is not available yet", "status", *vpce.State)
		// Nothing we can do at the moment, the VPC Endpoint needs to be accepted
		return vpce.VpcEndpointId, nil
	case "available":
		r.Log.V(1).Info("VPC Endpoint available", "status", *vpce.State)
	case "failed", "rejected", "deleting", "deleted":
		// No other known states, but just in case catch with a default
		fallthrough
	default:
		r.Log.V(0).Info("VPC Endpoint in a bad state", "status", *vpce.State)
		return vpce.VpcEndpointId, fmt.Errorf("vpc endpoint in a bad state: %s", *vpce.State)
	}

	subnetsResp, err := client.DescribePrivateSubnets(r.ClusterTag)
	if err != nil {
		return nil, err
	}

	privateSubnetIds := make([]*string, len(subnetsResp.Subnets))
	for i := range subnetsResp.Subnets {
		privateSubnetIds[i] = subnetsResp.Subnets[i].SubnetId
	}

	subnetsToAdd, subnetsToRemove := util.StringSliceTwoWayDiff(vpce.SubnetIds, privateSubnetIds)

	// TODO: ModifyVPCEndpoint should be made into a function
	// Removing subnets first before adding to avoid
	// DuplicateSubnetsInSameZone: Found another VPC endpoint subnet in the availability zone of <existing subnet>
	if len(subnetsToRemove) > 0 {
		r.Log.V(1).Info("Removing subnet(s) from VPC Endpoint", "subnetsToRemove", subnetsToRemove)
		if _, err := client.EC2Client.ModifyVpcEndpoint(&ec2.ModifyVpcEndpointInput{
			RemoveSubnetIds: subnetsToRemove,
			VpcEndpointId:   vpce.VpcEndpointId,
		}); err != nil {
			return nil, err
		}
	}

	if len(subnetsToAdd) > 0 {
		r.Log.V(1).Info("Adding subnet(s) to VPC Endpoint", "subnetsToAdd", subnetsToAdd)
		if _, err := client.EC2Client.ModifyVpcEndpoint(&ec2.ModifyVpcEndpointInput{
			AddSubnetIds:  subnetsToAdd,
			VpcEndpointId: vpce.VpcEndpointId,
		}); err != nil {
			return nil, err
		}
	}

	vpceSgIds := make([]*string, len(vpce.Groups))
	for i := range vpce.Groups {
		vpceSgIds[i] = vpce.Groups[i].GroupId
	}

	sgToAdd, sgToRemove := util.StringSliceTwoWayDiff(
		vpceSgIds,
		[]*string{&resource.Status.SecurityGroupId},
	)

	if len(sgToAdd) > 0 {
		r.Log.V(1).Info("Adding security group(s) to VPC Endpoint", "sgToAdd", sgToAdd)
		if _, err := client.EC2Client.ModifyVpcEndpoint(&ec2.ModifyVpcEndpointInput{
			AddSecurityGroupIds: sgToAdd,
			VpcEndpointId:       vpce.VpcEndpointId,
		}); err != nil {
			return nil, err
		}
	}

	if len(sgToRemove) > 0 {
		r.Log.V(1).Info("Removing security group(s) from VPC Endpoint", "sgToRemove", sgToRemove)
		if _, err := client.EC2Client.ModifyVpcEndpoint(&ec2.ModifyVpcEndpointInput{
			RemoveSecurityGroupIds: sgToRemove,
			VpcEndpointId:          vpce.VpcEndpointId,
		}); err != nil {
			return nil, err
		}
	}

	return vpce.VpcEndpointId, nil
}
