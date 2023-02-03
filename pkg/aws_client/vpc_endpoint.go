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
	"math"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/openshift/aws-vpce-operator/pkg/util"
)

// SelectVPCForVPCEndpoint uses a "least connection" strategy to place a VPC Endpoint in the provided VPC ID with the
// fewest existing VPC Endpoints in it to balance out quota usage.
// https://docs.aws.amazon.com/vpc/latest/userguide/amazon-vpc-limits.html#vpc-limits-endpoints
func (c *AWSClient) SelectVPCForVPCEndpoint(ctx context.Context, ids ...string) (string, error) {
	if len(ids) == 0 {
		return "", errors.New("must specify vpc id when counting VPC Endpoints per VPC")
	}

	input := &ec2.DescribeVpcEndpointsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: ids,
			},
		},
	}

	minVpcId := ""
	minVpceConsumed := math.MaxInt
	vpcePerVpc := map[string]int{}

	paginator := ec2.NewDescribeVpcEndpointsPaginator(c.ec2Client, input)
	for paginator.HasMorePages() {
		resp, err := paginator.NextPage(ctx)
		if err != nil {
			return "", err
		}

		for _, vpce := range resp.VpcEndpoints {
			vpcePerVpc[*vpce.VpcId]++
		}
	}

	for vpcId, vpceCount := range vpcePerVpc {
		if vpceCount < minVpceConsumed {
			minVpceConsumed = vpceCount
		}
		minVpcId = vpcId
	}

	return minVpcId, nil
}

// DescribeSingleVPCEndpointById returns information about a VPC endpoint with a given id.
func (c *AWSClient) DescribeSingleVPCEndpointById(ctx context.Context, id string) (*ec2.DescribeVpcEndpointsOutput, error) {
	if id == "" {
		// Otherwise, AWS will return all VPC endpoints (interpreting as no specified filter)
		return &ec2.DescribeVpcEndpointsOutput{}, nil
	}

	input := &ec2.DescribeVpcEndpointsInput{
		VpcEndpointIds: []string{id},
	}

	resp, err := c.ec2Client.DescribeVpcEndpoints(ctx, input)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			// Don't return an error if the VPC endpoint with the specified ID doesn't exist
			if ae.ErrorCode() == "InvalidVpcEndpointId.NotFound" {
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
func (c *AWSClient) FilterVPCEndpointByDefaultTags(ctx context.Context, clusterTag, vpceNameTag string) (*ec2.DescribeVpcEndpointsOutput, error) {
	if clusterTag == "" {
		return &ec2.DescribeVpcEndpointsOutput{}, nil
	}

	return c.ec2Client.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{vpceNameTag},
			},
			{
				Name:   aws.String("tag-key"),
				Values: []string{clusterTag},
			},
			{
				Name:   aws.String("tag:" + util.OperatorTagKey),
				Values: []string{util.OperatorTagValue},
			},
		},
	})
}

// CreateDefaultInterfaceVPCEndpoint creates an interface VPC endpoint with
// the default (open to all) VPC Endpoint policy. It attaches no security groups
// nor associates the VPC Endpoint with any subnets.
func (c *AWSClient) CreateDefaultInterfaceVPCEndpoint(ctx context.Context, name, vpcId, serviceName, tagKey string) (*ec2.CreateVpcEndpointOutput, error) {
	tags, err := util.GenerateAwsTags(name, tagKey)
	if err != nil {
		return nil, err
	}

	input := &ec2.CreateVpcEndpointInput{
		// TODO: Implement ClientToken for idempotency guarantees
		// ClientToken:     "token",
		VpcId:           &vpcId,
		ServiceName:     &serviceName,
		VpcEndpointType: types.VpcEndpointTypeInterface,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpcEndpoint,
				Tags:         tags,
			},
		},
	}

	return c.ec2Client.CreateVpcEndpoint(ctx, input)
}

// DeleteVPCEndpoint deletes a VPC endpoint with the given id.
func (c *AWSClient) DeleteVPCEndpoint(ctx context.Context, id string) (*ec2.DeleteVpcEndpointsOutput, error) {
	input := &ec2.DeleteVpcEndpointsInput{
		VpcEndpointIds: []string{id},
	}

	return c.ec2Client.DeleteVpcEndpoints(ctx, input)
}

// ModifyVpcEndpoint modifies a VPC endpoint
func (c *AWSClient) ModifyVpcEndpoint(ctx context.Context, input *ec2.ModifyVpcEndpointInput) (*ec2.ModifyVpcEndpointOutput, error) {
	return c.ec2Client.ModifyVpcEndpoint(ctx, input)
}
