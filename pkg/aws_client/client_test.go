package aws_client

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

type mockAvoEC2API struct {
	describeVpcEndpointServicesResp *ec2.DescribeVpcEndpointServicesOutput
}

func (m mockAvoEC2API) AuthorizeSecurityGroupEgress(ctx context.Context, params *ec2.AuthorizeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupEgressOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAvoEC2API) AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAvoEC2API) CreateSecurityGroup(ctx context.Context, params *ec2.CreateSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAvoEC2API) DeleteSecurityGroup(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAvoEC2API) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAvoEC2API) DescribeSecurityGroupRules(ctx context.Context, params *ec2.DescribeSecurityGroupRulesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupRulesOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAvoEC2API) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAvoEC2API) CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAvoEC2API) CreateVpcEndpoint(ctx context.Context, params *ec2.CreateVpcEndpointInput, optFns ...func(*ec2.Options)) (*ec2.CreateVpcEndpointOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAvoEC2API) DeleteVpcEndpoints(ctx context.Context, params *ec2.DeleteVpcEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVpcEndpointsOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAvoEC2API) DescribeVpcEndpoints(ctx context.Context, params *ec2.DescribeVpcEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAvoEC2API) ModifyVpcEndpoint(ctx context.Context, params *ec2.ModifyVpcEndpointInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVpcEndpointOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAvoEC2API) DescribeVpcEndpointServices(ctx context.Context, params *ec2.DescribeVpcEndpointServicesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointServicesOutput, error) {
	return m.describeVpcEndpointServicesResp, nil
}
