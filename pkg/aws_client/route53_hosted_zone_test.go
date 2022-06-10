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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/stretchr/testify/assert"
)

func (m *mockedRoute53) ListHostedZonesByName(input *route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error) {
	return &route53.ListHostedZonesByNameOutput{
		DNSName:      input.DNSName,
		HostedZoneId: aws.String(mockHostedZoneId),
		HostedZones: []*route53.HostedZone{
			{
				Id:   aws.String(mockHostedZoneId),
				Name: input.DNSName,
			},
		},
	}, nil
}

func (m *mockedRoute53) ListResourceRecordSets(input *route53.ListResourceRecordSetsInput) (*route53.ListResourceRecordSetsOutput, error) {
	return &route53.ListResourceRecordSetsOutput{
		ResourceRecordSets: []*route53.ResourceRecordSet{mockResourceRecordSet},
	}, nil
}

func (m *mockedRoute53) ChangeResourceRecordSets(input *route53.ChangeResourceRecordSetsInput) (*route53.ChangeResourceRecordSetsOutput, error) {
	return &route53.ChangeResourceRecordSetsOutput{}, nil
}

func TestAWSClient_GetDefaultPrivateHostedZoneId(t *testing.T) {
	tests := []struct {
		domainName string
		expectErr  bool
	}{
		{
			domainName: mockDomainName,
			expectErr:  false,
		},
	}

	client := &AWSClient{
		Route53Client: &mockedRoute53{},
	}

	for _, test := range tests {
		_, err := client.GetDefaultPrivateHostedZoneId(test.domainName)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestAWSClient_ListResourceRecordSets(t *testing.T) {
	client := &AWSClient{
		Route53Client: &mockedRoute53{},
	}

	_, err := client.ListResourceRecordSets(mockHostedZoneId)
	assert.NoError(t, err)
}

func TestAWSClient_UpsertDeleteResourceRecordSet(t *testing.T) {
	client := &AWSClient{
		Route53Client: &mockedRoute53{},
	}

	_, err := client.UpsertResourceRecordSet(mockResourceRecordSet, mockHostedZoneId)
	assert.NoError(t, err)

	_, err = client.DeleteResourceRecordSet(mockResourceRecordSet, mockHostedZoneId)
	assert.NoError(t, err)
}
