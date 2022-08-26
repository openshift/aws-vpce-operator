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
	avov1alpha1 "github.com/openshift/aws-vpce-operator/api/v1alpha1"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestVPCEndpointReconciler_validateSecurityGroup(t *testing.T) {
	tests := []struct {
		name      string
		resource  *avov1alpha1.VpcEndpoint
		expectErr bool
	}{
		{
			name:      "Nil resource",
			resource:  nil,
			expectErr: true,
		},
		{
			name: "minimum viable",
			resource: &avov1alpha1.VpcEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mock1",
				},
				Spec: avov1alpha1.VpcEndpointSpec{
					SecurityGroup: avov1alpha1.SecurityGroup{
						EgressRules: []avov1alpha1.SecurityGroupRule{
							{
								FromPort: 0,
								ToPort:   0,
								Protocol: "tcp",
							},
						},
						IngressRules: []avov1alpha1.SecurityGroupRule{
							{
								FromPort: 0,
								ToPort:   0,
								Protocol: "tcp",
							},
						},
					},
				},
				Status: avov1alpha1.VpcEndpointStatus{
					SecurityGroupId: aws_client.MockSecurityGroupId,
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
			Client:    client,
			Scheme:    client.Scheme(),
			awsClient: aws_client.NewMockedAwsClientWithSubnets(),
			log:       testr.New(t),
			clusterInfo: &clusterInfo{
				clusterTag: aws_client.MockClusterTag,
				infraName:  testutil.MockInfrastructureName,
			},
		}

		t.Run(test.name, func(t *testing.T) {
			err := r.validateSecurityGroup(context.TODO(), test.resource)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				condition := meta.FindStatusCondition(test.resource.Status.Conditions, avov1alpha1.AWSSecurityGroupCondition)
				assert.NotNilf(t, condition, "missing expected %s condition", avov1alpha1.AWSVpcEndpointCondition)
				assert.Equal(t, metav1.ConditionTrue, condition.Status)
			}
		})
	}
}

func TestVPCEndpointReconciler_validateVPCEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		resource  *avov1alpha1.VpcEndpoint
		expectErr bool
	}{
		{
			name:      "Nil resource",
			resource:  nil,
			expectErr: true,
		},
		{
			name: "minimum viable",
			resource: &avov1alpha1.VpcEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mock1",
				},
				Status: avov1alpha1.VpcEndpointStatus{
					VPCEndpointId: testutil.MockVpcEndpointId,
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
			Client:    client,
			Scheme:    client.Scheme(),
			awsClient: aws_client.NewMockedAwsClientWithSubnets(),
			log:       testr.New(t),
			clusterInfo: &clusterInfo{
				clusterTag: aws_client.MockClusterTag,
			},
		}

		t.Run(test.name, func(t *testing.T) {
			err := r.validateVPCEndpoint(context.TODO(), test.resource)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				condition := meta.FindStatusCondition(test.resource.Status.Conditions, avov1alpha1.AWSVpcEndpointCondition)
				assert.NotNilf(t, condition, "missing expected %s condition", avov1alpha1.AWSVpcEndpointCondition)
				assert.Equal(t, metav1.ConditionTrue, condition.Status)
			}
		})
	}
}

func TestVPCEndpointReconciler_validateR53HostedZoneRecord(t *testing.T) {
	tests := []struct {
		name       string
		domainName string
		resource   *avov1alpha1.VpcEndpoint
		expectErr  bool
	}{
		{
			name:      "Nil resource",
			resource:  nil,
			expectErr: true,
		},
		{
			name:       "minimum viable",
			domainName: "example.com",
			resource: &avov1alpha1.VpcEndpoint{
				Status: avov1alpha1.VpcEndpointStatus{
					VPCEndpointId: testutil.MockVpcEndpointId,
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		r := &VpcEndpointReconciler{
			awsClient: aws_client.NewMockedAwsClient(),
			log:       testr.New(t),
			clusterInfo: &clusterInfo{
				domainName: test.domainName,
			},
		}

		t.Run(test.name, func(t *testing.T) {
			err := r.validateR53HostedZoneRecord(context.TODO(), test.resource)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				condition := meta.FindStatusCondition(test.resource.Status.Conditions, avov1alpha1.AWSRoute53RecordCondition)
				assert.NotNilf(t, condition, "missing expected %s condition", avov1alpha1.AWSRoute53RecordCondition)
				assert.Equal(t, metav1.ConditionTrue, condition.Status)
			}
		})
	}
}

func TestVPcEndpointReconciler_validateExternalNameService(t *testing.T) {
	tests := []struct {
		name                    string
		resource                *avov1alpha1.VpcEndpoint
		existingSvc             client.Object
		expectedConditionStatus metav1.ConditionStatus
		expectedConditionReason string
		expectErr               bool
	}{
		{
			name:      "nil",
			resource:  nil,
			expectErr: true,
		},
		{
			name: "need to create",
			resource: &avov1alpha1.VpcEndpoint{
				Spec: avov1alpha1.VpcEndpointSpec{
					ExternalNameService: avov1alpha1.ExternalNameServiceSpec{
						Name:      "mock",
						Namespace: "mockns",
					},
					SubdomainName: "mocksubdomain",
				},
			},
			expectedConditionStatus: metav1.ConditionTrue,
			expectedConditionReason: "Created",
			expectErr:               true,
		},
		{
			name: "need to modify",
			resource: &avov1alpha1.VpcEndpoint{
				Spec: avov1alpha1.VpcEndpointSpec{
					ExternalNameService: avov1alpha1.ExternalNameServiceSpec{
						Name:      "mock",
						Namespace: "mockns",
					},
					SubdomainName: "mocksubdomain",
				},
			},
			existingSvc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mock",
					Namespace: "mockns",
				},
				Spec: corev1.ServiceSpec{
					ExternalName: "example.com",
				},
			},
			expectedConditionStatus: metav1.ConditionTrue,
			expectedConditionReason: "Reconciled",
			expectErr:               false,
		},
	}

	for _, test := range tests {
		mock := testutil.NewTestMock(t)
		if test.existingSvc != nil {
			mock = testutil.NewTestMock(t, test.existingSvc)
		}
		r := &VpcEndpointReconciler{
			Client: mock.Client,
			Scheme: mock.Client.Scheme(),
			log:    testr.New(t),
			clusterInfo: &clusterInfo{
				domainName: testutil.MockDomainName,
			},
		}
		t.Run(test.name, func(t *testing.T) {
			err := r.validateExternalNameService(context.TODO(), test.resource)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				condition := meta.FindStatusCondition(test.resource.Status.Conditions, avov1alpha1.ExternalNameServiceCondition)
				assert.NotNilf(t, condition, "missing expected %s condition", avov1alpha1.ExternalNameServiceCondition)
				assert.Equal(t, test.expectedConditionStatus, condition.Status)
				assert.Equal(t, test.expectedConditionReason, condition.Reason)
			}
		})
	}
}
