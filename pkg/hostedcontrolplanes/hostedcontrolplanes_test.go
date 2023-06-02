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

package hostedcontrolplanes

import (
	"context"
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestGetInfraId(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		hcp       *hyperv1beta1.HostedControlPlane
		expected  string
		expectErr bool
	}{
		{
			name:      "working",
			namespace: "example",
			hcp: &hyperv1beta1.HostedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "example",
				},
				Spec: hyperv1beta1.HostedControlPlaneSpec{
					InfraID: "mycluster",
				},
			},
			expected:  "mycluster",
			expectErr: false,
		},
		{
			name:      "empty infraId",
			namespace: "example",
			hcp: &hyperv1beta1.HostedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "example",
				},
				Spec: hyperv1beta1.HostedControlPlaneSpec{},
			},
			expectErr: true,
		},
		{
			name:      "no hostedcontrolplane",
			namespace: "example",
			hcp: &hyperv1beta1.HostedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "example2",
				},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mock := testutil.NewTestMock(t, test.hcp)
			actual, err := GetInfraId(context.TODO(), mock.Client, test.namespace)
			if test.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, test.expected, actual)
			}
		})
	}
}

func TestGetPrivateHostedZoneDomainName(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		hcp       *hyperv1beta1.HostedControlPlane
		expected  string
		expectErr bool
	}{
		{
			name:      "working",
			namespace: "example",
			hcp: &hyperv1beta1.HostedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "example",
				},
				Spec: hyperv1beta1.HostedControlPlaneSpec{
					Services: []hyperv1beta1.ServicePublishingStrategyMapping{
						{
							Service: hyperv1beta1.APIServer,
							ServicePublishingStrategy: hyperv1beta1.ServicePublishingStrategy{
								Type: hyperv1beta1.Route,
								Route: &hyperv1beta1.RoutePublishingStrategy{
									Hostname: "api.example.com",
								},
							},
						},
					},
				},
			},
			expected:  "example.com",
			expectErr: false,
		},
		{
			name:      "no APIServer URL",
			namespace: "example",
			hcp: &hyperv1beta1.HostedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "example",
				},
				Spec: hyperv1beta1.HostedControlPlaneSpec{
					Services: []hyperv1beta1.ServicePublishingStrategyMapping{
						{
							Service: hyperv1beta1.APIServer,
							ServicePublishingStrategy: hyperv1beta1.ServicePublishingStrategy{
								Type: hyperv1beta1.NodePort,
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name:      "no hostedcontrolplane",
			namespace: "example",
			hcp: &hyperv1beta1.HostedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "example2",
				},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mock := testutil.NewTestMock(t, test.hcp)
			actual, err := GetPrivateHostedZoneDomainName(context.TODO(), mock.Client, test.namespace)
			if test.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, test.expected, actual)
			}
		})
	}
}
