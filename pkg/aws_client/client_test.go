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
)

const (
	mockClusterTag             = "kubernetes.io/cluster/mock-12345"
	mockPublicSubnetId         = "subnet-pub12345"
	mockPrivateSubnetId        = "subnet-priv12345"
	mockSecurityGroupId        = "sg-12345"
	mockVpcId                  = "vpc-12345"
	mockVpcEndpointId          = "vpce-12345"
	mockVpcEndpointServiceName = "com.amazonaws.vpce.service.mock-12345"
)

var mockSubnets = []*ec2.Subnet{
	{
		SubnetId: aws.String(mockPrivateSubnetId),
		Tags: []*ec2.Tag{
			{
				Key:   aws.String(privateSubnetTagKey),
				Value: nil,
			},
			{
				Key:   aws.String(mockClusterTag),
				Value: aws.String("shared"),
			},
		},
		VpcId: aws.String(mockVpcId),
	},
	{
		SubnetId: aws.String(mockPublicSubnetId),
		Tags: []*ec2.Tag{
			{
				Key:   aws.String(publicSubnetTagKey),
				Value: nil,
			},
			{
				Key:   aws.String(mockClusterTag),
				Value: aws.String("shared"),
			},
		},
		VpcId: aws.String(mockVpcId),
	},
}

type mockedEC2 struct {
	ec2iface.EC2API

	Subnets []*ec2.Subnet
}

func newMockedEC2() *mockedEC2 {
	return &mockedEC2{
		Subnets: mockSubnets,
	}
}
