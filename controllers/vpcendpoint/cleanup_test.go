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

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
)

func TestVpcEndpointReconciler_cleanupAwsResources(t *testing.T) {
	tests := []struct {
		name      string
		resource  *avov1alpha2.VpcEndpoint
		expectErr bool
	}{
		{
			name: "all resources needing cleanup",
			resource: &avov1alpha2.VpcEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mock1",
				},
				Status: avov1alpha2.VpcEndpointStatus{
					SecurityGroupId: aws_client.MockSecurityGroupId,
					VPCEndpointId:   testutil.MockVpcEndpointId,
					Conditions: []metav1.Condition{
						{
							Type:   avov1alpha2.ExternalNameServiceCondition,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		client := testutil.NewTestMock(t).Client
		if test.resource != nil {
			client = testutil.NewTestMock(t, test.resource).Client
		}
		r := &VpcEndpointReconciler{
			Client:      client,
			Scheme:      client.Scheme(),
			awsClient:   aws_client.NewMockedAwsClientWithSubnets(),
			log:         testr.New(t),
			clusterInfo: &clusterInfo{},
		}

		err := r.cleanupAwsResources(context.TODO(), test.resource)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestDomainCount(t *testing.T) {
	tests := []struct {
		Name       string
		TestClient client.Client
		//Resources     []client.Object
		//Resources     []client.Object
		ExpectedCount int
		ExpectError   bool
	}{
		{
			Name: "one VPC with custom domain",
			TestClient: testutil.NewTestMock(t,
				&avov1alpha2.VpcEndpoint{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mock1",
						Namespace: "ns1",
					},
					Spec: avov1alpha2.VpcEndpointSpec{
						CustomDns: avov1alpha2.CustomDns{
							Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
								DomainName: "test.tld",
							},
						},
					},
				},
			).Client,
			ExpectedCount: 1,
			ExpectError:   false,
		},
		{
			Name: "two VPCs with same custom domain",
			TestClient: testutil.NewTestMock(t,
				&avov1alpha2.VpcEndpoint{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mock1",
						Namespace: "ns1",
					},
					Spec: avov1alpha2.VpcEndpointSpec{
						CustomDns: avov1alpha2.CustomDns{
							Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
								DomainName: "test.tld",
							},
						},
					},
				},
				&avov1alpha2.VpcEndpoint{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mock2",
						Namespace: "ns2",
					},
					Spec: avov1alpha2.VpcEndpointSpec{
						CustomDns: avov1alpha2.CustomDns{
							Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
								DomainName: "test.tld",
							},
						},
					},
				},
			).Client,
			ExpectedCount: 2,
			ExpectError:   false,
		},
	}

	for _, test := range tests {
		r := &VpcEndpointReconciler{
			Client:      test.TestClient,
			Scheme:      test.TestClient.Scheme(),
			awsClient:   aws_client.NewMockedAwsClientWithSubnets(),
			log:         testr.New(t),
			clusterInfo: &clusterInfo{},
		}

		count, err := r.domainCount("test.tld")
		if count != test.ExpectedCount {
			t.Errorf("TestDomainCount() %s: expected %d, got %d\n", test.Name, test.ExpectedCount, count)
		}
		if test.ExpectError == (err == nil) {
			t.Errorf("TestDomainCount() %s: ExpectError: %t, actual error: %s\n", test.Name, test.ExpectError, err)
		}
	}
}
