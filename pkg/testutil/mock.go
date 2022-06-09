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
	psov1alpha1 "github.com/openshift/aws-vpce-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type MockKubeClient struct {
	Client client.Client
}

func NewMock(t *testing.T, objs ...client.Object) *MockKubeClient {
	s := runtime.NewScheme()
	if err := configv1.Install(s); err != nil {
		t.Fatal(err)
	}

	if err := psov1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	return &MockKubeClient{
		Client: fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build(),
	}
}
