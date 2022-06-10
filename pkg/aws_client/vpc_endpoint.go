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

// DescribeSingleVPCEndpointById returns information about a VPC endpoint with a given id.
func (c *AWSClient) DescribeSingleVPCEndpointById(id string) (*ec2.DescribeVpcEndpointsOutput, error) {
	if id == "" {
		// Otherwise, AWS will return all VPC endpoints (interpreting as no specified filter)
		return &ec2.DescribeVpcEndpointsOutput{}, nil
	}

	input := &ec2.DescribeVpcEndpointsInput{
		VpcEndpointIds: []*string{
			aws.String(id),
		},
	}

	resp, err := c.EC2Client.DescribeVpcEndpoints(input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			// Don't return an error if the VPC endpoint with the specified ID doesn't exist
			if awsErr.Code() == "InvalidVpcEndpointId.NotFound" {
				return nil, nil
			}
		}
		return nil, err
	}

	if len(resp.VpcEndpoints) != 1 {
		return nil, fmt.Errorf("expected 1 VPC endpoint, got %d", len(resp.VpcEndpoints))
	}

	return resp, err
}

// FilterVPCEndpointByDefaultTags returns information about a VPC endpoint with the default expected tags.
func (c *AWSClient) FilterVPCEndpointByDefaultTags(clusterTag string) (*ec2.DescribeVpcEndpointsOutput, error) {
	if clusterTag == "" {
		return &ec2.DescribeVpcEndpointsOutput{}, nil
	}

	return c.EC2Client.DescribeVpcEndpoints(&ec2.DescribeVpcEndpointsInput{
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

// CreateDefaultInterfaceVPCEndpoint creates an interface VPC endpoint with
// the default (open to all) VPC Endpoint policy. It attaches no security groups
// nor associates the VPC Endpoint with any subnets.
func (c *AWSClient) CreateDefaultInterfaceVPCEndpoint(name, vpcId, serviceName, tagKey string) (*ec2.CreateVpcEndpointOutput, error) {
	tags, err := util.GenerateAwsTags(name, tagKey)
	if err != nil {
		return nil, err
	}

	input := &ec2.CreateVpcEndpointInput{
		// TODO: Implement ClientToken for idempotency guarantees
		// ClientToken:     "TODO",
		VpcId:           &vpcId,
		ServiceName:     &serviceName,
		VpcEndpointType: aws.String("Interface"),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("vpc-endpoint"),
				Tags:         tags,
			},
		},
	}

	return c.EC2Client.CreateVpcEndpoint(input)
}

// DeleteVPCEndpoint deletes a VPC endpoint with the given id.
func (c *AWSClient) DeleteVPCEndpoint(id string) (*ec2.DeleteVpcEndpointsOutput, error) {
	input := &ec2.DeleteVpcEndpointsInput{
		VpcEndpointIds: []*string{
			aws.String(id),
		},
	}

	return c.EC2Client.DeleteVpcEndpoints(input)
}
