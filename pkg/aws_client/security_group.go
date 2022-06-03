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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/openshift/aws-vpce-operator/pkg/util"
)

func (c *AWSClient) FilterClusterNodeSecurityGroupsByDefaultTags(infraName string) (*ec2.DescribeSecurityGroupsOutput, error) {
	clusterTag, err := util.GetClusterTagKey(infraName)
	if err != nil {
		return nil, err
	}

	return c.EC2Client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag-key"),
				Values: []*string{aws.String(clusterTag)},
			},
			{
				Name: aws.String("tag:Name"),
				Values: []*string{
					aws.String(fmt.Sprintf("%s-master-sg", infraName)),
					aws.String(fmt.Sprintf("%s-worker-sg", infraName)),
				},
			},
		},
	})
}

func (c *AWSClient) FilterSecurityGroupByDefaultTags(infraName string) (*ec2.DescribeSecurityGroupsOutput, error) {
	clusterTag, err := util.GetClusterTagKey(infraName)
	if err != nil {
		return nil, err
	}

	return c.EC2Client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag-key"),
				Values: []*string{aws.String(clusterTag)},
			},
			{
				Name:   aws.String("tag:" + util.OperatorTagKey),
				Values: []*string{aws.String(util.OperatorTagValue)},
			},
		},
	})
}

func (c *AWSClient) FilterSecurityGroupById(groupId string) (*ec2.DescribeSecurityGroupsOutput, error) {
	if groupId == "" {
		// Otherwise, AWS will return all security groups (interpreting, no specified filter)
		return &ec2.DescribeSecurityGroupsOutput{}, nil
	}

	input := &ec2.DescribeSecurityGroupsInput{
		GroupIds: []*string{
			aws.String(groupId),
		},
	}
	resp, err := c.EC2Client.DescribeSecurityGroups(input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			// Don't return an error if the security group with the specified ID doesn't exist
			if awsErr.Code() == "InvalidGroup.NotFound" {
				return nil, nil
			}
		}
		return nil, err
	}

	return resp, err
}

func (c *AWSClient) CreateSecurityGroup(name, vpcId, tagKey string) (*ec2.CreateSecurityGroupOutput, error) {
	tags, err := util.GenerateAwsTags(name, tagKey)
	if err != nil {
		return nil, err
	}

	input := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(name),
		Description: aws.String(util.SecurityGroupDescription),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("security-group"),
				Tags:         tags,
			},
		},
		VpcId: &vpcId,
	}
	return c.EC2Client.CreateSecurityGroup(input)
}

func (c *AWSClient) AuthorizeSecurityGroupRules(ingress *ec2.AuthorizeSecurityGroupIngressInput, egress *ec2.AuthorizeSecurityGroupEgressInput) ([]*ec2.SecurityGroupRule, error) {
	var rules []*ec2.SecurityGroupRule

	if len(ingress.IpPermissions) > 0 {
		ingressResp, err := c.EC2Client.AuthorizeSecurityGroupIngress(ingress)
		if err != nil {
			return nil, err
		}
		rules = append(rules, ingressResp.SecurityGroupRules...)
	}

	if len(egress.IpPermissions) > 0 {
		egressResp, err := c.EC2Client.AuthorizeSecurityGroupEgress(egress)
		if err != nil {
			return nil, err
		}
		rules = append(rules, egressResp.SecurityGroupRules...)
	}

	return rules, nil
}

func (c *AWSClient) DeleteSecurityGroup(groupId string) (*ec2.DeleteSecurityGroupOutput, error) {
	input := &ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(groupId),
	}
	return c.EC2Client.DeleteSecurityGroup(input)
}
