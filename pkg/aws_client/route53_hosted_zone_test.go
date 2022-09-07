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

	"github.com/openshift/aws-vpce-operator/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestAWSClient_GetDefaultPrivateHostedZoneId(t *testing.T) {
	tests := []struct {
		domainName string
		expectErr  bool
	}{
		{
			domainName: testutil.MockDomainName,
			expectErr:  false,
		},
	}

	client := NewMockedAwsClient()

	for _, test := range tests {
		_, err := client.GetDefaultPrivateHostedZoneId(context.TODO(), test.domainName)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestAWSClient_ListResourceRecordSets(t *testing.T) {
	client := NewMockedAwsClient()

	_, err := client.ListResourceRecordSets(context.TODO(), MockHostedZoneId)
	assert.NoError(t, err)
}

func TestAWSClient_UpsertDeleteResourceRecordSet(t *testing.T) {
	client := NewMockedAwsClient()

	_, err := client.UpsertResourceRecordSet(context.TODO(), mockResourceRecordSet, MockHostedZoneId)
	assert.NoError(t, err)

	_, err = client.DeleteResourceRecordSet(context.TODO(), mockResourceRecordSet, MockHostedZoneId)
	assert.NoError(t, err)
}
