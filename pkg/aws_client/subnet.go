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
	"github.com/openshift/aws-vpce-operator/api/v1alpha2"
)

// privateSubnetTagKey is labelled by Hive on a non-BYOVPC cluster's subnets at install time
const privateSubnetTagKey = "kubernetes.io/role/internal-elb"

// GetVPCId returns the VPC ID of the provided subnetIds. Returns an error if the subnets are not in the same VPC.
func (c *AWSClient) GetVPCId(ctx context.Context, subnetIds []string) (string, error) {
	if len(subnetIds) == 0 {
		return "", errors.New("no subnets provided")
	}

	input := &ec2.DescribeSubnetsInput{
		SubnetIds: subnetIds,
	}

	resp, err := c.ec2Client.DescribeSubnets(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to describe subnets: %w", err)
	}

	if len(resp.Subnets) == 0 {
		return "", fmt.Errorf("no subnets found with ids: %v", subnetIds)
	}

	vpcId := *resp.Subnets[0].VpcId
	for _, s := range resp.Subnets {
		if *s.VpcId != vpcId {
			return "", fmt.Errorf("subnets %v are a part of multiple VPCs", subnetIds)
		}
	}

	return vpcId, nil
}

// AutodiscoverPrivateSubnets attempts to automatically return a slice of ROSA cluster private subnet ids.
// A ROSA cluster's subnets are tagged with a tag key in AWS: "kubernetes.io/cluster/<cluster-name>".
// Private subnets for non-BYOVPC clusters also have the `kubernetes.io/role/internal-elb` tag key.
func (c *AWSClient) AutodiscoverPrivateSubnets(ctx context.Context, clusterTag string, tags ...v1alpha2.Tag) ([]types.Subnet, error) {
	// For non-BYOVPC clusters, resp will contain only the private subnets.
	nonByovpc, err := c.DescribeSubnetsByTags(ctx, append(tags, v1alpha2.Tag{Key: clusterTag}, v1alpha2.Tag{Key: privateSubnetTagKey})...)
	if err != nil {
		return nil, err
	}

	if len(nonByovpc.Subnets) != 0 {
		return nonByovpc.Subnets, nil
	}

	// For BYOVPC+PrivateLink clusters, resp will contain only the private subnets.
	// TODO: Make this work for BYOVPC non-PrivateLink clusters
	byovpc, err := c.DescribeSubnetsByTags(ctx, append(tags, v1alpha2.Tag{Key: clusterTag})...)
	if err != nil {
		return nil, err
	}

	if len(byovpc.Subnets) != 0 {
		return byovpc.Subnets, nil
	}

	return nil, fmt.Errorf("failed to find subnets with tag key: %s", clusterTag)
}

// DescribeSubnetsByTags returns a list of subnets filtered by the provided tags
// If there is no value in the provided tag, filtering is done by tag-key only
func (c *AWSClient) DescribeSubnetsByTags(ctx context.Context, tags ...v1alpha2.Tag) (*ec2.DescribeSubnetsOutput, error) {
	filters := []types.Filter{}
	for _, t := range tags {
		// If a tag-key is empty, don't filter by it as it will exclude all subnets i.e. treat it as bad input.
		if t.Key != "" {
			switch {
			case t.Value == "":
				// If a tag value is empty, filter by tag-key
				filters = append(filters, types.Filter{
					Name: aws.String("tag-key"),
					// Values are OR-ed
					Values: []string{t.Key},
				})
			default:
				filters = append(filters, types.Filter{
					Name:   aws.String(fmt.Sprintf("tag:%s", t.Key)),
					Values: []string{t.Value},
				})
			}
		}
	}

	input := &ec2.DescribeSubnetsInput{
		Filters: filters,
	}

	return c.ec2Client.DescribeSubnets(ctx, input)
}
