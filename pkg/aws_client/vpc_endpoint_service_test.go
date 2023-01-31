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
)

func TestAWSClient_GetVpcEndpointServiceAZs(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		resp        *ec2.DescribeVpcEndpointServicesOutput
		expectErr   bool
	}{
		{
			name:      "serviceName not specified",
			expectErr: true,
		},
		{
			name:        "service not found",
			serviceName: "mock",
			resp: &ec2.DescribeVpcEndpointServicesOutput{
				ServiceDetails: []types.ServiceDetail{},
			},
			expectErr: true,
		},
		{
			name:        "service found",
			serviceName: "mock",
			resp: &ec2.DescribeVpcEndpointServicesOutput{
				ServiceDetails: []types.ServiceDetail{
					{
						AvailabilityZones: []string{"us-east-1a"},
						ServiceName:       aws.String("mock"),
					},
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := AWSClient{ec2Client: mockAvoEC2API{describeVpcEndpointServicesResp: test.resp}}
			_, err := client.GetVpcEndpointServiceAZs(context.TODO(), test.serviceName)
			if err != nil {
				if !test.expectErr {
					t.Errorf("expected no err, got %v", err)
				}
			} else {
				if test.expectErr {
					t.Error("expected err, got nil")
				}
			}
		})
	}
}

func TestVpcEndpointAcceptanceAWSClient_AcceptVpcEndpointConnections(t *testing.T) {
	tests := []struct {
		name      string
		vpceIds   []string
		expectErr bool
	}{
		{
			name:      "nothing to accept",
			vpceIds:   []string{},
			expectErr: false,
		},
		{
			name:      "something to accept",
			vpceIds:   []string{"vpce-12345"},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := NewMockedVpceAcceptanceAwsClient()
			_, err := client.AcceptVpcEndpointConnections(context.TODO(), MockVpcEndpointServiceId, test.vpceIds...)
			if err != nil {
				if !test.expectErr {
					t.Fatalf("expected no error, but got %s", err)
				}
			} else {
				if test.expectErr {
					t.Fatal("expected error, but got none")
				}
			}
		})
	}
}
