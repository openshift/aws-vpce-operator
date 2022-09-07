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
	"github.com/stretchr/testify/assert"
)

func TestAWSClient_DescribeSecurityGroupRules(t *testing.T) {
	tests := []struct {
		groupId   string
		expectErr bool
	}{
		{
			groupId:   MockSecurityGroupId,
			expectErr: false,
		},
	}

	client := NewMockedAwsClient()

	for _, test := range tests {
		_, err := client.DescribeSecurityGroupRules(context.TODO(), test.groupId)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestAWSClient_AuthorizeSecurityGroupRules(t *testing.T) {
	tests := []struct {
		ingress          *ec2.AuthorizeSecurityGroupIngressInput
		egress           *ec2.AuthorizeSecurityGroupEgressInput
		expectedNumRules int
		expectErr        bool
	}{
		{
			ingress: &ec2.AuthorizeSecurityGroupIngressInput{
				GroupId: aws.String(MockSecurityGroupId),
				IpPermissions: []types.IpPermission{
					{
						FromPort:   aws.Int32(80),
						IpProtocol: aws.String("tcp"),
						ToPort:     aws.Int32(80),
					},
				},
			},
			egress: &ec2.AuthorizeSecurityGroupEgressInput{
				GroupId: aws.String(MockSecurityGroupId),
				IpPermissions: []types.IpPermission{
					{
						FromPort:   aws.Int32(80),
						IpProtocol: aws.String("tcp"),
						ToPort:     aws.Int32(80),
					},
				},
			},
			expectedNumRules: 2,
			expectErr:        false,
		},
	}

	client := NewMockedAwsClient()

	for _, test := range tests {
		resp, err := client.AuthorizeSecurityGroupRules(context.TODO(), test.ingress, test.egress)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, test.expectedNumRules, len(resp))
		}
	}
}
