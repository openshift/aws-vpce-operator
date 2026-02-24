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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestAWSClient_SelectVPCForVPCEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		ids        []string
		resp       *ec2.DescribeVpcEndpointsOutput
		expectedId string
		expectErr  bool
	}{
		{
			name:      "No ids",
			expectErr: true,
		},
		{
			name:       "No VPC Endpoints",
			ids:        []string{"vpc-01"},
			resp:       &ec2.DescribeVpcEndpointsOutput{},
			expectedId: "vpc-01",
			expectErr:  false,
		},
		{
			name: "vpc-02 is more empty",
			ids:  []string{"vpc-01", "vpc-02"},
			resp: &ec2.DescribeVpcEndpointsOutput{
				VpcEndpoints: []types.VpcEndpoint{
					{VpcId: aws.String("vpc-01")},
					{VpcId: aws.String("vpc-01")},
					{VpcId: aws.String("vpc-01")},
					{VpcId: aws.String("vpc-02")},
				},
			},
			expectedId: "vpc-02",
			expectErr:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := AWSClient{ec2Client: mockAvoEC2API{describeVpcEndpointResp: test.resp}}
			actualId, err := client.SelectVPCForVPCEndpoint(context.TODO(), test.ids...)
			if err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equalf(t, test.expectedId, actualId, "expected %s, got %s", test.expectedId, actualId)
			}
		})
	}
}

func TestAWSClient_DescribeSingleVPCEndpointById(t *testing.T) {
	client := NewMockedAwsClient()

	resp, err := client.DescribeSingleVPCEndpointById(context.TODO(), testutil.MockVpcEndpointId)
	assert.NoError(t, err)
	assert.Equal(t, testutil.MockVpcEndpointId, *resp.VpcEndpoints[0].VpcEndpointId)
}

func TestAWSClient_FilterVPCEndpointByDefaultTags(t *testing.T) {
	client := NewMockedAwsClient()

	_, err := client.FilterVPCEndpointByDefaultTags(context.TODO(), MockLegacyClusterTag, MockClusterNameTag)
	assert.NoError(t, err)
}

func TestCreateDeleteVPCEndpoint(t *testing.T) {
	client := NewMockedAwsClient()

	resp, err := client.CreateDefaultInterfaceVPCEndpoint(context.TODO(), "name", MockVpcId, MockVpcEndpointServiceName, MockLegacyClusterTag)
	assert.NoError(t, err)

	_, err = client.DeleteVPCEndpoint(context.TODO(), *resp.VpcEndpoint.VpcEndpointId)
	assert.NoError(t, err)
}

func TestAWSClient_GetVpcCidrBlock(t *testing.T) {
	client := NewMockedAwsClient()

	cidr, err := client.GetVpcCidrBlock(context.TODO(), MockVpcId)
	assert.NoError(t, err)
	assert.Equal(t, MockVpcCidr, cidr)

	_, err = client.GetVpcCidrBlock(context.TODO(), "")
	assert.Error(t, err)
}

func TestAWSClient_ModifyVPCEndpoint(t *testing.T) {
	tests := []struct {
		input     *ec2.ModifyVpcEndpointInput
		expectErr bool
	}{
		{
			input:     &ec2.ModifyVpcEndpointInput{},
			expectErr: false,
		},
	}

	client := NewMockedAwsClient()

	for _, test := range tests {
		_, err := client.ModifyVpcEndpoint(context.TODO(), test.input)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}
