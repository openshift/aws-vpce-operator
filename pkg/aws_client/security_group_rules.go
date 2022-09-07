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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// DescribeSecurityGroupRules describes the security group rules attached to a specific security group id
func (c *AWSClient) DescribeSecurityGroupRules(ctx context.Context, groupId string) (*ec2.DescribeSecurityGroupRulesOutput, error) {
	return c.ec2Client.DescribeSecurityGroupRules(ctx, &ec2.DescribeSecurityGroupRulesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("group-id"),
				Values: []string{groupId},
			},
		},
	})
}

// AuthorizeSecurityGroupRules authorizes provided ingress and egress rules for a security group,
// returning the updated security group rules and any errors
func (c *AWSClient) AuthorizeSecurityGroupRules(ctx context.Context, ingress *ec2.AuthorizeSecurityGroupIngressInput, egress *ec2.AuthorizeSecurityGroupEgressInput) ([]types.SecurityGroupRule, error) {
	var rules []types.SecurityGroupRule

	if len(ingress.IpPermissions) > 0 {
		ingressResp, err := c.ec2Client.AuthorizeSecurityGroupIngress(ctx, ingress)
		if err != nil {
			return nil, err
		}
		rules = append(rules, ingressResp.SecurityGroupRules...)
	}

	if len(egress.IpPermissions) > 0 {
		egressResp, err := c.ec2Client.AuthorizeSecurityGroupEgress(ctx, egress)
		if err != nil {
			return nil, err
		}
		rules = append(rules, egressResp.SecurityGroupRules...)
	}

	return rules, nil
}
