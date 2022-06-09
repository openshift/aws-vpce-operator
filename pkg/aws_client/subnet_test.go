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
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
)

func (m *MockedEC2) DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	tagKeys := map[string]bool{}
	for _, filter := range input.Filters {
		for _, tagKey := range filter.Values {
			tagKeys[*tagKey] = true
		}
	}

	for _, subnet := range m.Subnets {
		foundTags := 0
		for tagKey := range tagKeys {
			for _, tag := range subnet.Tags {
				if *tag.Key == tagKey {
					foundTags++
					break
				}
			}
			if foundTags == len(tagKeys) {
				return &ec2.DescribeSubnetsOutput{
					Subnets: []*ec2.Subnet{subnet},
				}, nil
			}
		}
	}

	return &ec2.DescribeSubnetsOutput{}, nil
}

func TestAWSClient_DescribeSubnets(t *testing.T) {
	tests := []struct {
		clusterTag        string
		expectedPrivateId string
		expectedPublicId  string
		expectedVpcId     string
		expectErr         bool
	}{
		{
			clusterTag:        MockClusterTag,
			expectedPrivateId: MockPrivateSubnetId,
			expectedPublicId:  MockPublicSubnetId,
			expectedVpcId:     MockVpcId,
			expectErr:         false,
		},
	}

	client := &AWSClient{
		EC2Client: newMockedEC2WithSubnets(),
	}

	for _, test := range tests {
		actualPrivate, err := client.DescribePrivateSubnets(test.clusterTag)
		if test.expectErr {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, len(actualPrivate.Subnets), 1)
			assert.Equal(t, test.expectedPrivateId, *actualPrivate.Subnets[0].SubnetId)
		}

		actualPublic, err := client.DescribePublicSubnets(test.clusterTag)
		if test.expectErr {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, len(actualPublic.Subnets), 1)
			assert.Equal(t, test.expectedPublicId, *actualPublic.Subnets[0].SubnetId)
		}

		actualVpcId, err := client.GetVPCId(test.clusterTag)
		if test.expectErr {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.expectedVpcId, actualVpcId)
		}
	}
}
