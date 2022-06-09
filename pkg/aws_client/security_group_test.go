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

func (m *MockedEC2) DescribeSecurityGroups(input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error) {
	if len(input.GroupIds) > 0 {
		securityGroups := make([]*ec2.SecurityGroup, len(input.GroupIds))
		for i, groupId := range input.GroupIds {
			securityGroups[i] = &ec2.SecurityGroup{
				GroupId: groupId,
			}
		}
		return &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: securityGroups,
		}, nil
	}

	if len(input.Filters) > 0 {
		for _, filter := range input.Filters {
			if *filter.Name == "tag-key" {
				return &ec2.DescribeSecurityGroupsOutput{
					SecurityGroups: []*ec2.SecurityGroup{
						{
							Tags: []*ec2.Tag{
								{
									Key:   filter.Values[0],
									Value: nil,
								},
							},
						},
					},
				}, nil
			}
		}
	}

	return &ec2.DescribeSecurityGroupsOutput{}, nil
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

func (m *MockedEC2) CreateSecurityGroup(input *ec2.CreateSecurityGroupInput) (*ec2.CreateSecurityGroupOutput, error) {
	if len(input.TagSpecifications) > 0 {
		return &ec2.CreateSecurityGroupOutput{
			GroupId: aws.String(MockSecurityGroupId),
			Tags:    input.TagSpecifications[0].Tags,
		}, nil
	}

	return &ec2.CreateSecurityGroupOutput{
		GroupId: aws.String(MockSecurityGroupId),
	}, nil
}

func (m *MockedEC2) DeleteSecurityGroup(input *ec2.DeleteSecurityGroupInput) (*ec2.DeleteSecurityGroupOutput, error) {
	return &ec2.DeleteSecurityGroupOutput{}, nil
}

func TestAWSClient_FilterClusterNodeSecurityGroupsByDefaultTags(t *testing.T) {
	tests := []struct {
		tagKey    string
		expectErr bool
	}{
		{
			tagKey:    MockClusterTag,
			expectErr: false,
		},
	}

	client := &AWSClient{
		EC2Client: &MockedEC2{},
	}

	for _, test := range tests {
		_, err := client.FilterClusterNodeSecurityGroupsByDefaultTags(test.tagKey)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestAWSClient_FilterSecurityGroupByDefaultTags(t *testing.T) {
	tests := []struct {
		tagKey    string
		expectErr bool
	}{
		{
			tagKey:    MockClusterTag,
			expectErr: false,
		},
	}

	client := &AWSClient{
		EC2Client: &MockedEC2{},
	}

	for _, test := range tests {
		_, err := client.FilterSecurityGroupByDefaultTags(test.tagKey)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestAWSClient_FilterSecurityGroupById(t *testing.T) {
	tests := []struct {
		groupId   string
		expectErr bool
	}{
		{
			groupId:   MockSecurityGroupId,
			expectErr: false,
		},
	}

	client := &AWSClient{
		EC2Client: &MockedEC2{},
	}

	for _, test := range tests {
		resp, err := client.FilterSecurityGroupById(test.groupId)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, 1, len(resp.SecurityGroups))
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

	client := &AWSClient{
		EC2Client: &MockedEC2{},
	}

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

func TestAWSClient_CreateDeleteSecurityGroup(t *testing.T) {
	client := &AWSClient{
		EC2Client: &MockedEC2{},
	}

	resp, err := client.CreateSecurityGroup("name", MockVpcId, MockClusterTag)
	assert.NoError(t, err)

	_, err = client.DeleteSecurityGroup(*resp.GroupId)
	assert.NoError(t, err)
}
