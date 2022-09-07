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

package aws_client

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/openshift/aws-vpce-operator/pkg/util"
)

// FilterClusterNodeSecurityGroupsByDefaultTags describes the security groups attached to the cluster nodes
// by filtering by the clusterTag and expected Name tags
func (c *AWSClient) FilterClusterNodeSecurityGroupsByDefaultTags(ctx context.Context, infraName string) (*ec2.DescribeSecurityGroupsOutput, error) {
	clusterTag, err := util.GetClusterTagKey(infraName)
	if err != nil {
		return nil, err
	}

	return c.ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag-key"),
				Values: []string{clusterTag},
			},
			{
				Name: aws.String("tag:Name"),
				Values: []string{
					fmt.Sprintf("%s-master-sg", infraName),
					fmt.Sprintf("%s-worker-sg", infraName),
				},
			},
		},
	})
}

// FilterSecurityGroupByDefaultTags describes the security group attached to the VPC Endpoint this operator manages
// by filtering by the clusterTag and operator tag
func (c *AWSClient) FilterSecurityGroupByDefaultTags(ctx context.Context, infraName, sgNameTag string) (*ec2.DescribeSecurityGroupsOutput, error) {
	clusterTag, err := util.GetClusterTagKey(infraName)
	if err != nil {
		return nil, err
	}

	return c.ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{sgNameTag},
			},
			{
				Name:   aws.String("tag-key"),
				Values: []string{clusterTag},
			},
			{
				Name:   aws.String("tag:" + util.OperatorTagKey),
				Values: []string{util.OperatorTagValue},
			},
		},
	})
}

// FilterSecurityGroupById describes a specific security group by ID
func (c *AWSClient) FilterSecurityGroupById(ctx context.Context, groupId string) (*ec2.DescribeSecurityGroupsOutput, error) {
	if groupId == "" {
		// Otherwise, AWS will return all security groups (interpreting, no specified filter)
		return &ec2.DescribeSecurityGroupsOutput{}, nil
	}

	input := &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{groupId},
	}
	resp, err := c.ec2Client.DescribeSecurityGroups(ctx, input)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "InvalidGroup.NotFound" {
				return nil, nil
			}
		}
		return nil, err
	}

	return resp, err
}

// CreateSecurityGroup creates a security group with the specified name and cluster tag key in a specified VPC
func (c *AWSClient) CreateSecurityGroup(ctx context.Context, name, vpcId, tagKey string) (*ec2.CreateSecurityGroupOutput, error) {
	tags, err := util.GenerateAwsTags(name, tagKey)
	if err != nil {
		return nil, err
	}

	input := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(name),
		Description: aws.String(util.SecurityGroupDescription),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags:         tags,
			},
		},
		VpcId: &vpcId,
	}
	return c.ec2Client.CreateSecurityGroup(ctx, input)
}

// DeleteSecurityGroup deletes a security group with the specified ID
func (c *AWSClient) DeleteSecurityGroup(ctx context.Context, groupId string) (*ec2.DeleteSecurityGroupOutput, error) {
	input := &ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(groupId),
	}
	return c.ec2Client.DeleteSecurityGroup(ctx, input)
}
