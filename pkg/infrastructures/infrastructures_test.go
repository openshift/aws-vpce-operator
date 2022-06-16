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

package infrastructures

import (
	"context"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	mockAWSRegion          = "us-gov-west-1"
	mockInfrastructureName = "rosa-cluster-abcd1"
)

var mockInfrastructure = &configv1.Infrastructure{
	ObjectMeta: metav1.ObjectMeta{
		Name: defaultInfrastructuresName,
	},
	Status: configv1.InfrastructureStatus{
		InfrastructureName: mockInfrastructureName,
		PlatformStatus: &configv1.PlatformStatus{
			Type: "AWS",
			AWS: &configv1.AWSPlatformStatus{
				Region: mockAWSRegion,
			},
		},
	},
}

func TestGetInfrastructureName(t *testing.T) {
	tests := []struct {
		infra     *configv1.Infrastructure
		expected  string
		expectErr bool
	}{
		{
			infra:     mockInfrastructure,
			expected:  mockInfrastructureName,
			expectErr: false,
		},
		{
			infra: &configv1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Name: "non-default-infra-name",
				},
				Status: configv1.InfrastructureStatus{
					InfrastructureName: mockInfrastructureName,
				},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		mock := testutil.NewTestMock(t, test.infra)
		actual, err := GetInfrastructureName(context.TODO(), mock.Client)
		if test.expectErr {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.expected, actual)
		}
	}
}

func TestGetAWSRegion(t *testing.T) {
	tests := []struct {
		infra     *configv1.Infrastructure
		expected  string
		expectErr bool
	}{
		{
			infra:     mockInfrastructure,
			expected:  mockAWSRegion,
			expectErr: false,
		},
		{
			infra: &configv1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Name: "non-default-infra-name",
				},
				Status: configv1.InfrastructureStatus{
					InfrastructureName: mockInfrastructureName,
				},
			},
			expectErr: true,
		},
		{
			infra: &configv1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Name: defaultInfrastructuresName,
				},
				Status: configv1.InfrastructureStatus{
					PlatformStatus: &configv1.PlatformStatus{
						Type: "GCP",
						GCP:  &configv1.GCPPlatformStatus{},
					},
				},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		mock := testutil.NewTestMock(t, test.infra)
		actual, err := GetAWSRegion(context.TODO(), mock.Client)
		if test.expectErr {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.expected, actual)
		}
	}
}
