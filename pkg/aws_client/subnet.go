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
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	privateSubnetTagKey = "kubernetes.io/role/internal-elb"
	publicSubnetTagKey  = "kubernetes.io/role/elb"
)

// GetVPCId returns the VPC ID which contains subnets with the specified tag key
// Returns an error if there are no subnets with the specified tag key or
// subnets with the specified tag key are not all in the same VPC
func (c *AWSClient) GetVPCId(ctx context.Context, tagKey string) (string, error) {
	subnets, err := c.DescribePrivateSubnets(ctx, tagKey)
	if err != nil {
		return "", fmt.Errorf("unable to DescribeSubnets: %w", err)
	}

	if len(subnets.Subnets) == 0 {
		return "", fmt.Errorf("no subnets with tag key: `%s`", tagKey)
	}

	vpcId := subnets.Subnets[0].VpcId
	for _, subnet := range subnets.Subnets {
		if *subnet.VpcId != *vpcId {
			return "", fmt.Errorf("subnets found with tag key: `%s` are a part of mulitple VPCs", tagKey)
		}
	}

	return *vpcId, nil
}

// DescribePrivateSubnets returns a list of private ROSA subnets that have the
// specified cluster tag key, typically "kubernetes.io/cluster/<cluster-name>".
// Private subnets are differentiated by also having the `kubernetes.io/role/internal-elb` tag key.
func (c *AWSClient) DescribePrivateSubnets(ctx context.Context, clusterTag string) (*ec2.DescribeSubnetsOutput, error) {
	return c.DescribeSubnetsByTagKey(ctx, clusterTag, privateSubnetTagKey)
}

// DescribePublicSubnets returns a list of public ROSA subnets that have the
// specified cluster tag key, typically "kubernetes.io/cluster/<cluster-name>".
// Public subnets are differentiated by also having the `kubernetes.io/role/elb` tag key.
func (c *AWSClient) DescribePublicSubnets(ctx context.Context, clusterTag string) (*ec2.DescribeSubnetsOutput, error) {
	return c.DescribeSubnetsByTagKey(ctx, clusterTag, publicSubnetTagKey)
}

// DescribeSubnetsByTagKey returns a list of subnets that have all the specified tag key(s).
func (c *AWSClient) DescribeSubnetsByTagKey(ctx context.Context, tagKey ...string) (*ec2.DescribeSubnetsOutput, error) {
	filters := make([]types.Filter, len(tagKey))
	for i := range tagKey {
		filters[i] = types.Filter{
			Name:   aws.String("tag-key"),
			Values: []string{tagKey[i]},
		}
	}

	input := &ec2.DescribeSubnetsInput{
		Filters: filters,
	}

	return c.ec2Client.DescribeSubnets(ctx, input)
}
