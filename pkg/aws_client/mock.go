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
)

const (
	MockClusterTag             = "kubernetes.io/cluster/mock-12345"
	MockDomainName             = "mock-domain.com"
	MockHostedZoneId           = "R53HZ12345"
	MockPublicSubnetId         = "subnet-pub12345"
	MockPrivateSubnetId        = "subnet-priv12345"
	MockSecurityGroupId        = "sg-12345"
	MockVpcId                  = "vpc-12345"
	MockVpcEndpointId          = "vpce-12345"
	MockVpcEndpointDnsName     = "vpce-12345.amazonaws.com"
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
			Value: aws.String(MockVpcEndpointDnsName),
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

func newMockedEC2WithSubnets() *MockedEC2 {
	return &MockedEC2{
		Subnets: mockSubnets,
	}
}
