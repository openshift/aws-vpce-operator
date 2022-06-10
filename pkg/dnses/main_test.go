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

package dnses

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const mockBaseDomain = "mock.base.domain"

func TestGetPrivateHostedZoneDomainName(t *testing.T) {
	tests := []struct {
		dns       *configv1.DNS
		expected  string
		expectErr bool
	}{
		{
			dns: &configv1.DNS{
				ObjectMeta: metav1.ObjectMeta{
					Name: defaultDnsesName,
				},
				Spec: configv1.DNSSpec{
					BaseDomain: mockBaseDomain,
				},
			},
			expected:  mockBaseDomain,
			expectErr: false,
		},
		{
			dns: &configv1.DNS{
				ObjectMeta: metav1.ObjectMeta{
					Name: "non-default-dnses-name",
				},
				Spec: configv1.DNSSpec{
					BaseDomain: mockBaseDomain,
				},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		mock := testutil.NewTestMock(t, test.dns)
		actual, err := GetPrivateHostedZoneDomainName(context.TODO(), mock.Client)
		if test.expectErr {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.expected, actual)
		}
	}
}
