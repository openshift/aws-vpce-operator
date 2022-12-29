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

package testutil

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	MockAWSRegion          = "us-gov-west-1"
	MockDomainName         = "mock-domain.com"
	MockInfrastructureName = "mock-12345"
	MockVpcEndpointId      = "vpce-12345"
	MockVpcEndpointDnsName = "vpce-12345.amazonaws.com"
)

type MockKubeClient struct {
	Client client.Client
}

var mockDnses = &configv1.DNS{
	ObjectMeta: metav1.ObjectMeta{
		Name: "cluster",
	},
	Spec: configv1.DNSSpec{
		BaseDomain: MockDomainName,
	},
}

var mockInfrastructure = &configv1.Infrastructure{
	ObjectMeta: metav1.ObjectMeta{
		Name: "cluster",
	},
	Spec: configv1.InfrastructureSpec{
		PlatformSpec: configv1.PlatformSpec{
			Type: "AWS",
		},
	},
	Status: configv1.InfrastructureStatus{
		InfrastructureName: MockInfrastructureName,
		PlatformStatus: &configv1.PlatformStatus{
			Type: "AWS",
			AWS: &configv1.AWSPlatformStatus{
				Region: MockAWSRegion,
			},
		},
	},
}

func NewDefaultMock() (*MockKubeClient, error) {
	return NewMock(mockDnses, mockInfrastructure)
}

func NewTestMock(t *testing.T, objs ...client.Object) *MockKubeClient {
	mock, err := NewMock(objs...)
	if err != nil {
		t.Fatal(err)
	}

	return mock
}

func NewMock(obs ...client.Object) (*MockKubeClient, error) {
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		return nil, err
	}

	if err := configv1.Install(s); err != nil {
		return nil, err
	}

	if err := avov1alpha2.AddToScheme(s); err != nil {
		return nil, err
	}

	return &MockKubeClient{
		Client: fake.NewClientBuilder().WithScheme(s).WithObjects(obs...).Build(),
	}, nil
}
