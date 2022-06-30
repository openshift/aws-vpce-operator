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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
)

func (m *MockedEC2) DescribeSecurityGroupRules(input *ec2.DescribeSecurityGroupRulesInput) (*ec2.DescribeSecurityGroupRulesOutput, error) {
	// TODO: This is a no-op
	return &ec2.DescribeSecurityGroupRulesOutput{
		SecurityGroupRules: []*ec2.SecurityGroupRule{},
	}, nil
}

func (m *MockedEC2) AuthorizeSecurityGroupIngress(input *ec2.AuthorizeSecurityGroupIngressInput) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	rules := make([]*ec2.SecurityGroupRule, len(input.IpPermissions))
	for i, permission := range input.IpPermissions {
		rules[i] = &ec2.SecurityGroupRule{
			FromPort:   permission.FromPort,
			IpProtocol: permission.IpProtocol,
			ToPort:     permission.ToPort,
		}
	}

	return &ec2.AuthorizeSecurityGroupIngressOutput{
		SecurityGroupRules: rules,
	}, nil
}

func (m *MockedEC2) AuthorizeSecurityGroupEgress(input *ec2.AuthorizeSecurityGroupEgressInput) (*ec2.AuthorizeSecurityGroupEgressOutput, error) {
	rules := make([]*ec2.SecurityGroupRule, len(input.IpPermissions))
	for i, permission := range input.IpPermissions {
		rules[i] = &ec2.SecurityGroupRule{
			FromPort:   permission.FromPort,
			IpProtocol: permission.IpProtocol,
			ToPort:     permission.ToPort,
		}
	}

	return &ec2.AuthorizeSecurityGroupEgressOutput{
		SecurityGroupRules: rules,
	}, nil
}

func TestAWSClient_DescribeSecurityGroupRules(t *testing.T) {
	tests := []struct {
		groupId   string
		expectErr bool
	}{
		{
			groupId:   MockSecurityGroupId,
			expectErr: false,
		},
	}

	client := NewMockedAwsClient()

	for _, test := range tests {
		_, err := client.DescribeSecurityGroupRules(test.groupId)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestAWSClient_AuthorizeSecurityGroupRules(t *testing.T) {
	tests := []struct {
		ingress          *ec2.AuthorizeSecurityGroupIngressInput
		egress           *ec2.AuthorizeSecurityGroupEgressInput
		expectedNumRules int
		expectErr        bool
	}{
		{
			ingress: &ec2.AuthorizeSecurityGroupIngressInput{
				GroupId: aws.String(MockSecurityGroupId),
				IpPermissions: []*ec2.IpPermission{
					{
						FromPort:   aws.Int64(80),
						IpProtocol: aws.String("tcp"),
						ToPort:     aws.Int64(80),
					},
				},
			},
			egress: &ec2.AuthorizeSecurityGroupEgressInput{
				GroupId: aws.String(MockSecurityGroupId),
				IpPermissions: []*ec2.IpPermission{
					{
						FromPort:   aws.Int64(80),
						IpProtocol: aws.String("tcp"),
						ToPort:     aws.Int64(80),
					},
				},
			},
			expectedNumRules: 2,
			expectErr:        false,
		},
	}

	client := NewMockedAwsClient()

	for _, test := range tests {
		resp, err := client.AuthorizeSecurityGroupRules(test.ingress, test.egress)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, test.expectedNumRules, len(resp))
		}
	}
}
