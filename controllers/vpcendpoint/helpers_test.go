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
		Client: mock.Client,
		log:    testr.New(t),
		Scheme: mock.Client.Scheme(),
		awsClient: &aws_client.AWSClient{
			EC2Client: aws_client.NewMockedEC2WithSubnets(),
		},
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
		Client: mock.Client,
		log:    testr.New(t),
		Scheme: mock.Client.Scheme(),
		awsClient: &aws_client.AWSClient{
			EC2Client:     aws_client.NewMockedEC2WithSubnets(),
			Route53Client: aws_client.MockedRoute53{},
		},
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
