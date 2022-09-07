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

	"github.com/stretchr/testify/assert"
)

func TestAWSClient_FilterClusterNodeSecurityGroupsByDefaultTags(t *testing.T) {
	tests := []struct {
		tagKey    string
		expectErr bool
	}{
		{
			tagKey:    MockClusterTag,
			expectErr: false,
		},
	}

	client := NewMockedAwsClient()

	for _, test := range tests {
		_, err := client.FilterClusterNodeSecurityGroupsByDefaultTags(context.TODO(), test.tagKey)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestAWSClient_FilterSecurityGroupByDefaultTags(t *testing.T) {
	tests := []struct {
		tagKey    string
		nameTag   string
		expectErr bool
	}{
		{
			tagKey:    MockClusterTag,
			nameTag:   MockSecurityGroupId,
			expectErr: false,
		},
	}

	client := NewMockedAwsClient()

	for _, test := range tests {
		_, err := client.FilterSecurityGroupByDefaultTags(context.TODO(), test.tagKey, test.nameTag)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestAWSClient_FilterSecurityGroupById(t *testing.T) {
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
		resp, err := client.FilterSecurityGroupById(context.TODO(), test.groupId)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, 1, len(resp.SecurityGroups))
		}
	}
}

func TestAWSClient_CreateDeleteSecurityGroup(t *testing.T) {
	client := NewMockedAwsClient()

	resp, err := client.CreateSecurityGroup(context.TODO(), "name", MockVpcId, MockClusterTag)
	assert.NoError(t, err)

	_, err = client.DeleteSecurityGroup(context.TODO(), *resp.GroupId)
	assert.NoError(t, err)
}
