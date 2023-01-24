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
)

func TestAWSClient_DescribeSubnets(t *testing.T) {
	tests := []struct {
		clusterTag string
		expectErr  bool
	}{
		{
			clusterTag: MockClusterTag,
			expectErr:  false,
		},
		{
			clusterTag: "doesn't exist",
			expectErr:  true,
		},
	}

	client := NewMockedAwsClientWithSubnets()

	for _, test := range tests {
		t.Run(test.clusterTag, func(t *testing.T) {
			_, err := client.AutodiscoverPrivateSubnets(context.TODO(), test.clusterTag)
			if err != nil {
				if !test.expectErr {
					t.Errorf("expected no error, got %s", err)
				}
			} else {
				if test.expectErr {
					t.Error("expected error, got nil")
				}
			}
		})
	}
}
