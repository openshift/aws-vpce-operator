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
)

// GetVpcEndpointServiceAZs returns a slice of strings indicating which AZs the specified VPC Endpoint Service supports
func (c *AWSClient) GetVpcEndpointServiceAZs(ctx context.Context, serviceName string) ([]string, error) {
	if serviceName == "" {
		return nil, errors.New("GetVpcEndpointServiceAZs: serviceName must be specified")
	}

	input := &ec2.DescribeVpcEndpointServicesInput{
		ServiceNames: []string{serviceName},
	}

	resp, err := c.ec2Client.DescribeVpcEndpointServices(ctx, input)
	if err != nil {
		return nil, err
	}

	if len(resp.ServiceDetails) != 1 {
		return nil, fmt.Errorf("expected one VPC Endpoint Service with name %s, got %d", serviceName, len(resp.ServiceDetails))
	}

	return resp.ServiceDetails[0].AvailabilityZones, nil
}

// GetVpcEndpointConnectionsPendingAcceptance returns information about a VPC endpoint with a given id.
func (c *VpcEndpointAcceptanceAWSClient) GetVpcEndpointConnectionsPendingAcceptance(ctx context.Context, id string) (*ec2.DescribeVpcEndpointConnectionsOutput, error) {
	if id == "" {
		// Otherwise, AWS will return all VPC endpoints (interpreting as no specified filter)
		return &ec2.DescribeVpcEndpointConnectionsOutput{VpcEndpointConnections: []types.VpcEndpointConnection{}}, nil
	}

	input := &ec2.DescribeVpcEndpointConnectionsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("service-id"),
				Values: []string{id},
			},
			{
				Name:   aws.String("vpc-endpoint-state"),
				Values: []string{"pendingAcceptance"},
			},
		},
	}

	return c.ec2Client.DescribeVpcEndpointConnections(ctx, input)
}

// AcceptVpcEndpointConnections is a wrapper around ec2:AcceptVpcEndpointConnections for a give VPC Endpoint serviceId
// and a slice of vpcEndpointIds
func (c *VpcEndpointAcceptanceAWSClient) AcceptVpcEndpointConnections(ctx context.Context, serviceId string, vpcEndpointIds ...string) (*ec2.AcceptVpcEndpointConnectionsOutput, error) {
	if len(vpcEndpointIds) == 0 {
		return &ec2.AcceptVpcEndpointConnectionsOutput{}, nil
	}

	input := &ec2.AcceptVpcEndpointConnectionsInput{
		ServiceId:      aws.String(serviceId),
		VpcEndpointIds: vpcEndpointIds,
	}

	return c.ec2Client.AcceptVpcEndpointConnections(ctx, input)
}
