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
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
)

const (
	MockLegacyClusterTag       = "kubernetes.io/cluster/mock-12345"
	MockCapiClusterTag         = "sigs.k8s.io/cluster-api-provider-aws/cluster/mock-54321"
	MockClusterNameTag         = "mock-12345-vpce"
	MockHostedZoneId           = "R53HZ12345"
	MockPublicSubnetId         = "subnet-pub12345"
	MockPrivateSubnetId        = "subnet-priv12345"
	MockSecurityGroupId        = "sg-12345"
	MockVpcId                  = "vpc-12345"
	MockVpcEndpointServiceName = "com.amazonaws.vpce.service.mock-12345"
	MockVpcEndpointServiceId   = "vpce-svc-12345"
)

type MockedEC2 struct {
	AvoEC2API

	Subnets []*ec2Types.Subnet
}

type MockedRoute53 struct {
	AvoRoute53API
}

var mockResourceRecordSet = &route53Types.ResourceRecordSet{
	Name: aws.String("mock"),
	ResourceRecords: []route53Types.ResourceRecord{
		{
			Value: aws.String(testutil.MockVpcEndpointDnsName),
		},
	},
	TTL:  aws.Int64(300),
	Type: route53Types.RRTypeCname,
}

var mockSubnets = []*ec2Types.Subnet{
	{
		SubnetId: aws.String(MockPrivateSubnetId),
		Tags: []ec2Types.Tag{
			{
				Key:   aws.String(privateSubnetTagKey),
				Value: nil,
			},
			{
				Key:   aws.String(MockLegacyClusterTag),
				Value: aws.String("shared"),
			},
		},
		VpcId: aws.String(MockVpcId),
	},
	{
		SubnetId: aws.String(MockPublicSubnetId),
		Tags:     []ec2Types.Tag{},
		VpcId:    aws.String(MockVpcId),
	},
}

func NewMockedEC2WithSubnets() *MockedEC2 {
	return &MockedEC2{
		Subnets: mockSubnets,
	}
}

func NewMockedAwsClient() *AWSClient {
	return NewAwsClientWithServiceClients(&MockedEC2{}, &MockedRoute53{})
}

func NewMockedVpceAcceptanceAwsClient() *VpcEndpointAcceptanceAWSClient {
	return NewVpcEndpointAcceptanceAwsClientWithServiceClients(&MockedEC2{})
}

func NewMockedAwsClientWithSubnets() *AWSClient {
	return NewAwsClientWithServiceClients(NewMockedEC2WithSubnets(), &MockedRoute53{})
}

func (m *MockedEC2) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	tagKeys := map[string]bool{}
	for _, filter := range params.Filters {
		for _, tagKey := range filter.Values {
			tagKeys[tagKey] = true
		}
	}

	for _, subnet := range m.Subnets {
		foundTags := 0
		for tagKey := range tagKeys {
			for _, tag := range subnet.Tags {
				if *tag.Key == tagKey {
					foundTags++
					continue
				}
			}
			if foundTags == len(tagKeys) {
				return &ec2.DescribeSubnetsOutput{
					Subnets: []ec2Types.Subnet{*subnet},
				}, nil
			}
		}
	}

	return &ec2.DescribeSubnetsOutput{}, nil
}

func (m *MockedEC2) CreateSecurityGroup(ctx context.Context, params *ec2.CreateSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
	if len(params.TagSpecifications) > 0 {
		return &ec2.CreateSecurityGroupOutput{
			GroupId: aws.String(MockSecurityGroupId),
			Tags:    params.TagSpecifications[0].Tags,
		}, nil
	}

	return &ec2.CreateSecurityGroupOutput{
		GroupId: aws.String(MockSecurityGroupId),
	}, nil
}

func (m *MockedEC2) DeleteSecurityGroup(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
	return &ec2.DeleteSecurityGroupOutput{}, nil
}

func (m *MockedEC2) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	if len(params.GroupIds) > 0 {
		securityGroups := make([]ec2Types.SecurityGroup, len(params.GroupIds))
		for i, groupId := range params.GroupIds {
			securityGroups[i] = ec2Types.SecurityGroup{
				GroupId: aws.String(groupId),
			}
		}
		return &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: securityGroups,
		}, nil
	}

	if len(params.Filters) > 0 {
		for _, filter := range params.Filters {
			if *filter.Name == "tag-key" {
				return &ec2.DescribeSecurityGroupsOutput{
					SecurityGroups: []ec2Types.SecurityGroup{
						{
							GroupId: aws.String(MockSecurityGroupId),
							Tags: []ec2Types.Tag{
								{
									Key:   aws.String(filter.Values[0]),
									Value: nil,
								},
							},
						},
					},
				}, nil
			} else if *filter.Name == "Tag:Name" {
				return &ec2.DescribeSecurityGroupsOutput{
					SecurityGroups: []ec2Types.SecurityGroup{
						{
							GroupId: aws.String(MockSecurityGroupId),
							Tags: []ec2Types.Tag{
								{
									Key:   aws.String("Name"),
									Value: aws.String(filter.Values[0]),
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

func (m *MockedEC2) DescribeSecurityGroupRules(ctx context.Context, params *ec2.DescribeSecurityGroupRulesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupRulesOutput, error) {
	// Mock now contains "pre-existing" rules to ensure SG rules created by customer using IP's over SGs do not cause failures
	// while reconciling security group rules
	return &ec2.DescribeSecurityGroupRulesOutput{
		SecurityGroupRules: []ec2Types.SecurityGroupRule{
			ec2Types.SecurityGroupRule{
				CidrIpv4:            aws.String("0.0.0.0/0"),
				CidrIpv6:            nil,
				Description:         aws.String("bad rule with no source SG"),
				FromPort:            aws.Int32(1),
				GroupId:             nil,
				GroupOwnerId:        nil,
				IpProtocol:          aws.String("tcp"),
				IsEgress:            aws.Bool(false),
				PrefixListId:        nil,
				ReferencedGroupInfo: nil,
				SecurityGroupRuleId: nil,
				Tags:                nil,
				ToPort:              aws.Int32(1),
			},
			ec2Types.SecurityGroupRule{
				CidrIpv4:            aws.String("0.0.0.0/0"),
				CidrIpv6:            nil,
				Description:         aws.String("bad rule with no source SG"),
				FromPort:            aws.Int32(1),
				GroupId:             nil,
				GroupOwnerId:        nil,
				IpProtocol:          aws.String("tcp"),
				IsEgress:            aws.Bool(true),
				PrefixListId:        nil,
				ReferencedGroupInfo: nil,
				SecurityGroupRuleId: nil,
				Tags:                nil,
				ToPort:              aws.Int32(1),
			},
		},
	}, nil
}

func (m *MockedEC2) AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	rules := make([]ec2Types.SecurityGroupRule, len(params.IpPermissions))
	for i, permission := range params.IpPermissions {
		rules[i] = ec2Types.SecurityGroupRule{
			FromPort:   permission.FromPort,
			IpProtocol: permission.IpProtocol,
			ToPort:     permission.ToPort,
		}
	}

	return &ec2.AuthorizeSecurityGroupIngressOutput{
		SecurityGroupRules: rules,
	}, nil
}

func (m *MockedEC2) AuthorizeSecurityGroupEgress(ctx context.Context, params *ec2.AuthorizeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupEgressOutput, error) {
	rules := make([]ec2Types.SecurityGroupRule, len(params.IpPermissions))
	for i, permission := range params.IpPermissions {
		rules[i] = ec2Types.SecurityGroupRule{
			FromPort:   permission.FromPort,
			IpProtocol: permission.IpProtocol,
			ToPort:     permission.ToPort,
		}
	}

	return &ec2.AuthorizeSecurityGroupEgressOutput{
		SecurityGroupRules: rules,
	}, nil
}

func (m *MockedEC2) CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	// TODO: this is a no-op
	return &ec2.CreateTagsOutput{}, nil
}

func (m *MockedEC2) CreateVpcEndpoint(ctx context.Context, params *ec2.CreateVpcEndpointInput, optFns ...func(*ec2.Options)) (*ec2.CreateVpcEndpointOutput, error) {
	return &ec2.CreateVpcEndpointOutput{
		VpcEndpoint: &ec2Types.VpcEndpoint{
			State:         "available",
			VpcEndpointId: aws.String(testutil.MockVpcEndpointId),
		},
	}, nil
}

func (m *MockedEC2) AcceptVpcEndpointConnections(ctx context.Context, params *ec2.AcceptVpcEndpointConnectionsInput, optFns ...func(*ec2.Options)) (*ec2.AcceptVpcEndpointConnectionsOutput, error) {
	if len(params.VpcEndpointIds) == 0 {
		return nil, fmt.Errorf("1 validation error(s) found.\n- missing required field")
	}

	return &ec2.AcceptVpcEndpointConnectionsOutput{}, nil
}

func (m *MockedEC2) DescribeVpcEndpointConnections(ctx context.Context, params *ec2.DescribeVpcEndpointConnectionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointConnectionsOutput, error) {
	// TODO: This is a no-op
	return &ec2.DescribeVpcEndpointConnectionsOutput{}, nil
}

func (m *MockedEC2) DeleteVpcEndpoints(ctx context.Context, params *ec2.DeleteVpcEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVpcEndpointsOutput, error) {
	// TODO: This is a no-op
	return &ec2.DeleteVpcEndpointsOutput{}, nil
}

func (m *MockedEC2) DescribeVpcEndpoints(ctx context.Context, params *ec2.DescribeVpcEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	// Mock a VPC Endpoint if an ID is supplied
	if len(params.VpcEndpointIds) > 0 {
		return &ec2.DescribeVpcEndpointsOutput{
			VpcEndpoints: []ec2Types.VpcEndpoint{
				{
					VpcEndpointId: aws.String(params.VpcEndpointIds[0]),
					DnsEntries: []ec2Types.DnsEntry{
						{
							DnsName: aws.String(testutil.MockVpcEndpointDnsName),
						},
					},
					State: "available",
				},
			},
		}, nil
	}

	// Mock a VPC Endpoint with a specified tag-key
	if len(params.Filters) > 0 {
		for _, filter := range params.Filters {
			if *filter.Name == "tag-key" {
				return &ec2.DescribeVpcEndpointsOutput{
					VpcEndpoints: []ec2Types.VpcEndpoint{
						{
							VpcEndpointId: aws.String(testutil.MockVpcEndpointId),
							DnsEntries: []ec2Types.DnsEntry{
								{
									DnsName: aws.String(testutil.MockVpcEndpointDnsName),
								},
							},
							State: "available",
							Tags: []ec2Types.Tag{
								{
									Key:   aws.String(filter.Values[0]),
									Value: nil,
								},
							},
						},
					},
				}, nil
			}
		}
	}

	return &ec2.DescribeVpcEndpointsOutput{}, nil
}

func (m *MockedEC2) ModifyVpcEndpoint(ctx context.Context, params *ec2.ModifyVpcEndpointInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVpcEndpointOutput, error) {
	// TODO: This is a no-op
	return &ec2.ModifyVpcEndpointOutput{}, nil
}

func (m *MockedRoute53) ListHostedZonesByName(ctx context.Context, params *route53.ListHostedZonesByNameInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesByNameOutput, error) {
	return &route53.ListHostedZonesByNameOutput{
		DNSName:      params.DNSName,
		HostedZoneId: aws.String(MockHostedZoneId),
		HostedZones: []route53Types.HostedZone{
			{
				Id:   aws.String(MockHostedZoneId),
				Name: params.DNSName,
			},
		},
	}, nil
}

func (m *MockedRoute53) ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	return &route53.ListResourceRecordSetsOutput{
		ResourceRecordSets: []route53Types.ResourceRecordSet{*mockResourceRecordSet},
	}, nil
}

func (m *MockedRoute53) ChangeResourceRecordSets(ctx context.Context, params *route53.ChangeResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ChangeResourceRecordSetsOutput, error) {
	return &route53.ChangeResourceRecordSetsOutput{}, nil
}
