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

	"github.com/stretchr/testify/assert"
)

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
		EC2Client: NewMockedEC2WithSubnets(),
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
