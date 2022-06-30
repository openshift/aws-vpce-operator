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

func (m *MockedEC2) CreateTags(input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	// TODO: this is a no-op
	return &ec2.CreateTagsOutput{}, nil
}

func TestAWSClient_CreateTags(t *testing.T) {
	tests := []struct {
		input     *ec2.CreateTagsInput
		expectErr bool
	}{
		{
			input:     &ec2.CreateTagsInput{},
			expectErr: false,
		},
	}

	client := NewMockedAwsClient()

	for _, test := range tests {
		_, err := client.CreateTags(test.input)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}
