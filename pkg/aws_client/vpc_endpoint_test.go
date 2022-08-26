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
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
)

func (m *MockedEC2) CreateVpcEndpoint(input *ec2.CreateVpcEndpointInput) (*ec2.CreateVpcEndpointOutput, error) {
	return &ec2.CreateVpcEndpointOutput{
		VpcEndpoint: &ec2.VpcEndpoint{
			State:         aws.String("available"),
			VpcEndpointId: aws.String(testutil.MockVpcEndpointId),
		},
	}, nil
}

func TestAWSClient_DescribeSingleVPCEndpointById(t *testing.T) {
	client := NewMockedAwsClient()

	resp, err := client.DescribeSingleVPCEndpointById(testutil.MockVpcEndpointId)
	assert.NoError(t, err)
	assert.Equal(t, testutil.MockVpcEndpointId, *resp.VpcEndpoints[0].VpcEndpointId)
}

func TestAWSClient_FilterVPCEndpointByDefaultTags(t *testing.T) {
	client := NewMockedAwsClient()

	_, err := client.FilterVPCEndpointByDefaultTags(MockClusterTag)
	assert.NoError(t, err)
}

func TestCreateDeleteVPCEndpoint(t *testing.T) {
	client := NewMockedAwsClient()

	resp, err := client.CreateDefaultInterfaceVPCEndpoint("name", MockVpcId, MockVpcEndpointServiceName, MockClusterTag)
	assert.NoError(t, err)

	_, err = client.DeleteVPCEndpoint(*resp.VpcEndpoint.VpcEndpointId)
	assert.NoError(t, err)
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
		_, err := client.ModifyVpcEndpoint(test.input)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}
