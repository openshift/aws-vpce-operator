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

package vpcendpoint

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/openshift/aws-vpce-operator/api/v1alpha1"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultAVORateLimiter(t *testing.T) {
	limiter := defaultAVORateLimiter()

	if expected, actual := 1*time.Second, limiter.When("test"); expected != actual {
		t.Errorf("expected %v, got %v", expected, actual)
	}
	if expected, actual := 2*time.Second, limiter.When("test"); expected != actual {
		t.Errorf("expected %v, got %v", expected, actual)
	}
	if expected, actual := 4*time.Second, limiter.When("test"); expected != actual {
		t.Errorf("expected %v, got %v", expected, actual)
	}
	if expected, actual := 8*time.Second, limiter.When("test"); expected != actual {
		t.Errorf("expected %v, got %v", expected, actual)
	}
	if expected, actual := 4, limiter.NumRequeues("test"); expected != actual {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestParseClusterInfo(t *testing.T) {
	mock, err := testutil.NewDefaultMock()
	if err != nil {
		t.Fatal(err)
	}

	r := &VpcEndpointReconciler{
		Client:      mock.Client,
		log:         testr.New(t),
		Scheme:      mock.Client.Scheme(),
		awsClient:   aws_client.NewMockedAwsClientWithSubnets(),
		clusterInfo: nil,
	}

	err = r.parseClusterInfo(context.TODO(), false)
	assert.NoError(t, err)
}

func TestDefaultResourceRecord(t *testing.T) {
	tests := []struct {
		resource  *v1alpha1.VpcEndpoint
		expectErr bool
	}{
		{
			resource: &v1alpha1.VpcEndpoint{
				Status: v1alpha1.VpcEndpointStatus{
					VPCEndpointId: testutil.MockVpcEndpointId,
				},
			},
			expectErr: false,
		},
		{
			resource:  &v1alpha1.VpcEndpoint{},
			expectErr: true,
		},
	}

	mock, err := testutil.NewDefaultMock()
	if err != nil {
		t.Fatal(err)
	}

	r := &VpcEndpointReconciler{
		Client:      mock.Client,
		log:         testr.New(t),
		Scheme:      mock.Client.Scheme(),
		awsClient:   aws_client.NewMockedAwsClientWithSubnets(),
		clusterInfo: nil,
	}

	for _, test := range tests {
		_, err = r.defaultResourceRecord(test.resource)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestServiceForVpce(t *testing.T) {
	var (
		trueBool = true
	)
	tests := []struct {
		resource   *v1alpha1.VpcEndpoint
		domainName string
		expected   *corev1.Service
		expectErr  bool
	}{
		{
			resource: &v1alpha1.VpcEndpoint{
				Spec: v1alpha1.VpcEndpointSpec{
					SubdomainName: "demo",
					ExternalNameService: v1alpha1.ExternalNameServiceSpec{
						Name:      "demo",
						Namespace: "demo-ns",
					},
				},
			},
			domainName: "my.cluster.com",
			expected: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo",
					Namespace: "demo-ns",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "avo.openshift.io/v1alpha1",
							Kind:               "VpcEndpoint",
							Controller:         &trueBool,
							BlockOwnerDeletion: &trueBool,
						},
					},
				},
				Spec: corev1.ServiceSpec{
					Type:         corev1.ServiceTypeExternalName,
					ExternalName: "demo.my.cluster.com",
				},
			},
			expectErr: false,
		},
		{
			resource: &v1alpha1.VpcEndpoint{
				Spec: v1alpha1.VpcEndpointSpec{
					ExternalNameService: v1alpha1.ExternalNameServiceSpec{
						Name:      "demo",
						Namespace: "demo-ns",
					},
				},
			},
			domainName: "my.cluster.com",
			expectErr:  true,
		},
		{
			resource: &v1alpha1.VpcEndpoint{
				Spec: v1alpha1.VpcEndpointSpec{
					SubdomainName: "demo",
					ExternalNameService: v1alpha1.ExternalNameServiceSpec{
						Name:      "demo",
						Namespace: "demo-ns",
					},
				},
			},
			expectErr: true,
		},
	}

	mock, err := testutil.NewDefaultMock()
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		r := &VpcEndpointReconciler{
			Client: mock.Client,
			log:    testr.New(t),
			Scheme: mock.Client.Scheme(),
			clusterInfo: &clusterInfo{
				domainName: test.domainName,
			},
		}

		actual, err := r.expectedServiceForVpce(test.resource)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		}
	}
}
