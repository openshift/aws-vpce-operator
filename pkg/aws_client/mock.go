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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
)

const (
	MockClusterTag             = "kubernetes.io/cluster/mock-12345"
	MockHostedZoneId           = "R53HZ12345"
	MockPublicSubnetId         = "subnet-pub12345"
	MockPrivateSubnetId        = "subnet-priv12345"
	MockSecurityGroupId        = "sg-12345"
	MockVpcId                  = "vpc-12345"
	MockVpcEndpointServiceName = "com.amazonaws.vpce.service.mock-12345"
)

type MockedEC2 struct {
	ec2iface.EC2API

	Subnets []*ec2.Subnet
}

type MockedRoute53 struct {
	route53iface.Route53API
}

var mockResourceRecordSet = &route53.ResourceRecordSet{
	Name: aws.String("mock"),
	ResourceRecords: []*route53.ResourceRecord{
		{
			Value: aws.String(testutil.MockVpcEndpointDnsName),
		},
	},
	TTL:  aws.Int64(300),
	Type: aws.String("CNAME"),
}

var mockSubnets = []*ec2.Subnet{
	{
		SubnetId: aws.String(MockPrivateSubnetId),
		Tags: []*ec2.Tag{
			{
				Key:   aws.String(privateSubnetTagKey),
				Value: nil,
			},
			{
				Key:   aws.String(MockClusterTag),
				Value: aws.String("shared"),
			},
		},
		VpcId: aws.String(MockVpcId),
	},
	{
		SubnetId: aws.String(MockPublicSubnetId),
		Tags: []*ec2.Tag{
			{
				Key:   aws.String(publicSubnetTagKey),
				Value: nil,
			},
			{
				Key:   aws.String(MockClusterTag),
				Value: aws.String("shared"),
			},
		},
		VpcId: aws.String(MockVpcId),
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

func NewMockedAwsClientWithSubnets() *AWSClient {
	return NewAwsClientWithServiceClients(NewMockedEC2WithSubnets(), &MockedRoute53{})
}

func (m *MockedEC2) DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	tagKeys := map[string]bool{}
	for _, filter := range input.Filters {
		for _, tagKey := range filter.Values {
			tagKeys[*tagKey] = true
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
					Subnets: []*ec2.Subnet{subnet},
				}, nil
			}
		}
	}

	return &ec2.DescribeSubnetsOutput{}, nil
}

func (m *MockedEC2) DescribeVpcEndpoints(input *ec2.DescribeVpcEndpointsInput) (*ec2.DescribeVpcEndpointsOutput, error) {
	// Mock a VPC Endpoint if an ID is supplied
	if len(input.VpcEndpointIds) > 0 {
		return &ec2.DescribeVpcEndpointsOutput{
			VpcEndpoints: []*ec2.VpcEndpoint{
				{
					VpcEndpointId: input.VpcEndpointIds[0],
					DnsEntries: []*ec2.DnsEntry{
						{
							DnsName: aws.String(testutil.MockVpcEndpointDnsName),
						},
					},
					State: aws.String("available"),
				},
			},
		}, nil
	}

	// Mock a VPC Endpoint with a specified tag-key
	if len(input.Filters) > 0 {
		for _, filter := range input.Filters {
			if *filter.Name == "tag-key" {
				return &ec2.DescribeVpcEndpointsOutput{
					VpcEndpoints: []*ec2.VpcEndpoint{
						{
							DnsEntries: []*ec2.DnsEntry{
								{
									DnsName: aws.String(testutil.MockVpcEndpointDnsName),
								},
							},
							State: aws.String("available"),
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

	return &ec2.DescribeVpcEndpointsOutput{}, nil
}

func (m *MockedEC2) ModifyVpcEndpoint(input *ec2.ModifyVpcEndpointInput) (*ec2.ModifyVpcEndpointOutput, error) {
	// TODO: This is a no-op
	return &ec2.ModifyVpcEndpointOutput{}, nil
}

func (m *MockedRoute53) ListHostedZonesByName(input *route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error) {
	return &route53.ListHostedZonesByNameOutput{
		DNSName:      input.DNSName,
		HostedZoneId: aws.String(MockHostedZoneId),
		HostedZones: []*route53.HostedZone{
			{
				Id:   aws.String(MockHostedZoneId),
				Name: input.DNSName,
			},
		},
	}, nil
}

func (m *MockedRoute53) ChangeResourceRecordSets(input *route53.ChangeResourceRecordSetsInput) (*route53.ChangeResourceRecordSetsOutput, error) {
	return &route53.ChangeResourceRecordSetsOutput{}, nil
}
