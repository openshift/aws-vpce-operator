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

package vpcendpointtemplate

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-logr/logr/testr"
	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
	hyperv1beta1 "github.com/openshift/hypershift/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCreateVpcEndpoint(t *testing.T) {
	const namespace = "test-ns"

	tests := []struct {
		name      string
		vpcet     *avov1alpha2.VpcEndpointTemplate
		expectErr bool
	}{
		{
			name: "Working",
			vpcet: &avov1alpha2.VpcEndpointTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample",
					Namespace: "aws-vpce-operator",
				},
				Spec: avov1alpha2.VpcEndpointTemplateSpec{
					Type: avov1alpha2.HCPVpcEndpointTemplateType,
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key": "value",
						},
					},
					Template: avov1alpha2.VpceTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"key": "value",
							},
						},
						Spec: avov1alpha2.VpcEndpointSpec{
							ServiceNameRef: &avov1alpha2.ServiceName{
								ValueFrom: &avov1alpha2.ServiceNameSource{
									AwsEndpointServiceRef: &avov1alpha2.AwsEndpointSelector{
										Name: "private-router",
									},
								},
							},
							SecurityGroup: avov1alpha2.SecurityGroup{},
							Region:        "us-east-1",
						},
					},
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := testutil.NewTestMock(t).Client
			r := &VpcEndpointTemplateReconciler{
				Client: c,
				Scheme: c.Scheme(),
				log:    testr.New(t),
			}

			err := r.CreateVpcEndpoint(context.TODO(), test.vpcet, namespace)
			if err != nil {
				if !test.expectErr {
					t.Errorf("expected no err, got %v", err)
				}
				return
			}

			if test.expectErr {
				t.Error("expected err, got nil")
			}

			vpce := new(avov1alpha2.VpcEndpoint)
			if err := r.Get(context.TODO(), client.ObjectKey{
				Namespace: namespace,
				Name:      test.vpcet.Name,
			}, vpce); err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if !reflect.DeepEqual(test.vpcet.Spec.Selector.MatchLabels, vpce.Labels) {
				t.Errorf("mismatched labels, expected %v, got %v", test.vpcet.Spec.Selector.MatchLabels, vpce.Labels)
			}
		})
	}
}

func TestFilterHostedControlPlanes(t *testing.T) {
	tests := []struct {
		name          string
		vpcet         *avov1alpha2.VpcEndpointTemplate
		mockHCPs      []client.Object //*hyperv1beta1.HostedControlPlane
		expectedCount int
		expectErr     bool
	}{
		{
			name: "empty list",
			vpcet: &avov1alpha2.VpcEndpointTemplate{
				Spec: avov1alpha2.VpcEndpointTemplateSpec{
					Type: avov1alpha2.HCPVpcEndpointTemplateType,
				},
			},
			mockHCPs:      nil,
			expectedCount: 0,
			expectErr:     false,
		},
		{
			name: "one HCP",
			vpcet: &avov1alpha2.VpcEndpointTemplate{
				Spec: avov1alpha2.VpcEndpointTemplateSpec{
					Type: avov1alpha2.HCPVpcEndpointTemplateType,
				},
			},
			mockHCPs: []client.Object{
				&hyperv1beta1.HostedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name1",
						Namespace: "namespace1",
					},
				},
				&hyperv1beta1.HostedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name2",
						Namespace: "namespace2",
					},
				},
				&hyperv1beta1.HostedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name3",
						Namespace: "namespace3",
					},
				},
			},
			expectedCount: 3,
			expectErr:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := testutil.NewTestMock(t).Client
			if test.mockHCPs != nil {
				client = testutil.NewTestMock(t, test.mockHCPs...).Client
			}
			r := &VpcEndpointTemplateReconciler{
				Client: client,
				Scheme: client.Scheme(),
				log:    testr.New(t),
			}

			actual, err := r.FilterHostedControlPlanes(context.TODO(), test.vpcet)
			if err != nil {
				if !test.expectErr {
					t.Errorf("expected no err, got %s", err)
				}
			} else {
				if test.expectedCount != len(actual) {
					t.Errorf("expected %v hostedcontrolplanes, got %v", test.expectedCount, len(actual))
				}
			}
		})
	}
}
