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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

// AvoEC2API defines the subset of the AWS EC2 API that AVO needs to interact with
type AvoEC2API interface {
	AuthorizeSecurityGroupEgress(ctx context.Context, params *ec2.AuthorizeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupEgressOutput, error)
	AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	CreateSecurityGroup(ctx context.Context, params *ec2.CreateSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error)
	DeleteSecurityGroup(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error)
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeSecurityGroupRules(ctx context.Context, params *ec2.DescribeSecurityGroupRulesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupRulesOutput, error)

	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)

	CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)

	CreateVpcEndpoint(ctx context.Context, params *ec2.CreateVpcEndpointInput, optFns ...func(*ec2.Options)) (*ec2.CreateVpcEndpointOutput, error)
	DeleteVpcEndpoints(ctx context.Context, params *ec2.DeleteVpcEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVpcEndpointsOutput, error)
	DescribeVpcEndpoints(ctx context.Context, params *ec2.DescribeVpcEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error)
	ModifyVpcEndpoint(ctx context.Context, params *ec2.ModifyVpcEndpointInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVpcEndpointOutput, error)

	DescribeVpcEndpointServices(ctx context.Context, params *ec2.DescribeVpcEndpointServicesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointServicesOutput, error)
}

// AvoRoute53API defines the subset of the AWS Route53 API that AVO needs to interact with
type AvoRoute53API interface {
	ChangeResourceRecordSets(ctx context.Context, params *route53.ChangeResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ChangeResourceRecordSetsOutput, error)
	ChangeTagsForResource(ctx context.Context, input *route53.ChangeTagsForResourceInput, optFns ...func(*route53.Options)) (*route53.ChangeTagsForResourceOutput, error)
	CreateHostedZone(ctx context.Context, params *route53.CreateHostedZoneInput, optFns ...func(*route53.Options)) (*route53.CreateHostedZoneOutput, error)
	DeleteHostedZone(ctx context.Context, params *route53.DeleteHostedZoneInput, optFns ...func(*route53.Options)) (*route53.DeleteHostedZoneOutput, error)
	GetHostedZone(ctx context.Context, params *route53.GetHostedZoneInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error)
	ListHostedZonesByVPC(ctx context.Context, params *route53.ListHostedZonesByVPCInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesByVPCOutput, error)
	ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
	ListTagsForResource(ctx context.Context, params *route53.ListTagsForResourceInput, optFns ...func(*route53.Options)) (*route53.ListTagsForResourceOutput, error)
}

type AWSClient struct {
	ec2Client     AvoEC2API
	route53Client AvoRoute53API
}

type AvoVpcEndpointAcceptanceEc2Api interface {
	AcceptVpcEndpointConnections(ctx context.Context, params *ec2.AcceptVpcEndpointConnectionsInput, optFns ...func(*ec2.Options)) (*ec2.AcceptVpcEndpointConnectionsOutput, error)
	DescribeVpcEndpointConnections(ctx context.Context, params *ec2.DescribeVpcEndpointConnectionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointConnectionsOutput, error)
}

type VpcEndpointAcceptanceAWSClient struct {
	ec2Client AvoVpcEndpointAcceptanceEc2Api
}

// NewAwsClient returns an AWSClient with the provided session
func NewAwsClient(cfg aws.Config) *AWSClient {
	return NewAwsClientWithServiceClients(ec2.NewFromConfig(cfg), route53.NewFromConfig(cfg))
}

// NewAwsClientWithServiceClients returns an AWSClient with the provided EC2 and Route53 clients.
// Typically, not used directly except for building a mock for testing.
func NewAwsClientWithServiceClients(ec2 AvoEC2API, r53 AvoRoute53API) *AWSClient {
	return &AWSClient{
		ec2Client:     ec2,
		route53Client: r53,
	}
}

// NewVpcEndpointAcceptanceAwsClient returns an VpcEndpointAcceptanceAWSClient with the provided session
func NewVpcEndpointAcceptanceAwsClient(cfg aws.Config) *VpcEndpointAcceptanceAWSClient {
	return &VpcEndpointAcceptanceAWSClient{
		ec2Client: ec2.NewFromConfig(cfg),
	}
}

// NewVpcEndpointAcceptanceAwsClientWithServiceClients returns a VpcEndpointAcceptanceAWSClient with the provided
// EC2 client. Typically, not used directly except for building a mock for testing.
func NewVpcEndpointAcceptanceAwsClientWithServiceClients(ec2 AvoVpcEndpointAcceptanceEc2Api) *VpcEndpointAcceptanceAWSClient {
	return &VpcEndpointAcceptanceAWSClient{
		ec2Client: ec2,
	}
}
