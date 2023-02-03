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
	"k8s.io/client-go/tools/record"
	"testing"

	"github.com/go-logr/logr/testr"
	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateVpcEndpointCR(t *testing.T) {
	tests := []struct {
		name      string
		vpce      *avov1alpha2.VpcEndpoint
		expectErr bool
	}{
		{
			name: "Override region + Autodiscovery",
			vpce: &avov1alpha2.VpcEndpoint{
				Spec: avov1alpha2.VpcEndpointSpec{
					Region: "us-east-1",
					Vpc: avov1alpha2.Vpc{
						AutoDiscoverSubnets: true,
					},
					CustomDns: avov1alpha2.CustomDns{
						Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
							AutoDiscover: true,
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "VPC Load Balancing + Subnet IDs",
			vpce: &avov1alpha2.VpcEndpoint{
				Spec: avov1alpha2.VpcEndpointSpec{
					Vpc: avov1alpha2.Vpc{
						AutoDiscoverSubnets: true,
						SubnetIds:           []string{"subnet-a", "subnet-b", "subnet-c"},
						Ids:                 []string{"vpc-a", "vpc-b", "vpc-c"},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Specifying Route53 HZ ID + Route53 HZ Domain Name",
			vpce: &avov1alpha2.VpcEndpoint{
				Spec: avov1alpha2.VpcEndpointSpec{
					CustomDns: avov1alpha2.CustomDns{
						Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
							DomainName: "example.com",
							Id:         "ABCDEFG",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Valid example 1",
			vpce: &avov1alpha2.VpcEndpoint{
				Spec: avov1alpha2.VpcEndpointSpec{
					Vpc: avov1alpha2.Vpc{
						AutoDiscoverSubnets: true,
						Ids:                 []string{"vpc-a", "vpc-b", "vpc-c"},
					},
					CustomDns: avov1alpha2.CustomDns{
						Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
							Id: "ABCDEFG",
						},
					},
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateVpcEndpointCR(test.vpce)
			if err != nil {
				if !test.expectErr {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if test.expectErr {
					t.Error("expected error, got nil")
				}
			}
		})
	}
}

func TestVPCEndpointReconciler_validateSecurityGroup(t *testing.T) {
	tests := []struct {
		name      string
		resource  *avov1alpha2.VpcEndpoint
		expectErr bool
	}{
		{
			name:      "Nil resource",
			resource:  nil,
			expectErr: true,
		},
		{
			name: "minimum viable",
			resource: &avov1alpha2.VpcEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mock1",
				},
				Spec: avov1alpha2.VpcEndpointSpec{
					SecurityGroup: avov1alpha2.SecurityGroup{
						EgressRules: []avov1alpha2.SecurityGroupRule{
							{
								FromPort: 0,
								ToPort:   0,
								Protocol: "tcp",
							},
						},
						IngressRules: []avov1alpha2.SecurityGroupRule{
							{
								FromPort: 0,
								ToPort:   0,
								Protocol: "tcp",
							},
						},
					},
				},
				Status: avov1alpha2.VpcEndpointStatus{
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
			Recorder: record.NewFakeRecorder(1),
		}

		t.Run(test.name, func(t *testing.T) {
			err := r.validateSecurityGroup(context.TODO(), test.resource)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				condition := meta.FindStatusCondition(test.resource.Status.Conditions, avov1alpha2.AWSSecurityGroupCondition)
				assert.NotNilf(t, condition, "missing expected %s condition", avov1alpha2.AWSVpcEndpointCondition)
				assert.Equal(t, metav1.ConditionTrue, condition.Status)
			}
		})
	}
}

func TestVPCEndpointReconciler_validateVPCEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		resource  *avov1alpha2.VpcEndpoint
		expectErr bool
	}{
		{
			name:      "Nil resource",
			resource:  nil,
			expectErr: true,
		},
		{
			name: "minimum viable",
			resource: &avov1alpha2.VpcEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mock1",
				},
				Status: avov1alpha2.VpcEndpointStatus{
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

				condition := meta.FindStatusCondition(test.resource.Status.Conditions, avov1alpha2.AWSVpcEndpointCondition)
				assert.NotNilf(t, condition, "missing expected %s condition", avov1alpha2.AWSVpcEndpointCondition)
				assert.Equal(t, metav1.ConditionTrue, condition.Status)
			}
		})
	}
}

//func TestVPCEndpointReconciler_validateR53HostedZoneRecord(t *testing.T) {
//	tests := []struct {
//		name       string
//		domainName string
//		resource   *avov1alpha2.VpcEndpoint
//		expectErr  bool
//	}{
//		{
//			name:      "Nil resource",
//			resource:  nil,
//			expectErr: true,
//		},
//		{
//			name:       "minimum viable",
//			domainName: "example.com",
//			resource: &avov1alpha2.VpcEndpoint{
//				Status: avov1alpha2.VpcEndpointStatus{
//					VPCEndpointId: testutil.MockVpcEndpointId,
//				},
//			},
//			expectErr: false,
//		},
//	}
//
//	for _, test := range tests {
//		r := &VpcEndpointReconciler{
//			awsClient: aws_client.NewMockedAwsClient(),
//			log:       testr.New(t),
//			clusterInfo: &clusterInfo{
//				domainName: test.domainName,
//			},
//		}
//
//		t.Run(test.name, func(t *testing.T) {
//			err := r.validateR53HostedZoneRecord(context.TODO(), test.resource)
//			if test.expectErr {
//				assert.Error(t, err)
//			} else {
//				assert.NoError(t, err)
//
//				condition := meta.FindStatusCondition(test.resource.Status.Conditions, avov1alpha2.AWSRoute53RecordCondition)
//				assert.NotNilf(t, condition, "missing expected %s condition", avov1alpha2.AWSRoute53RecordCondition)
//				assert.Equal(t, metav1.ConditionTrue, condition.Status)
//			}
//		})
//	}
//}

//func TestVPcEndpointReconciler_validateExternalNameService(t *testing.T) {
//	tests := []struct {
//		name                    string
//		resource                *avov1alpha2.VpcEndpoint
//		existingSvc             client.Object
//		expectedConditionStatus metav1.ConditionStatus
//		expectedConditionReason string
//		expectErr               bool
//	}{
//		{
//			name:      "nil",
//			resource:  nil,
//			expectErr: true,
//		},
//		{
//			name: "need to create",
//			resource: &avov1alpha2.VpcEndpoint{
//				ObjectMeta: metav1.ObjectMeta{
//					Name:      "mock-vpce",
//					Namespace: "mockns",
//				},
//				Spec: avov1alpha2.VpcEndpointSpec{
//					ExternalNameService: avov1alpha2.ExternalNameServiceSpec{
//						Name: "mock",
//					},
//					SubdomainName: "mocksubdomain",
//				},
//			},
//			expectedConditionStatus: metav1.ConditionTrue,
//			expectedConditionReason: "Created",
//			expectErr:               true,
//		},
//		{
//			name: "need to modify",
//			resource: &avov1alpha2.VpcEndpoint{
//				ObjectMeta: metav1.ObjectMeta{
//					Name:      "mock-vpce",
//					Namespace: "mockns",
//				},
//				Spec: avov1alpha2.VpcEndpointSpec{
//					ExternalNameService: avov1alpha2.ExternalNameServiceSpec{
//						Name: "mock",
//					},
//					SubdomainName: "mocksubdomain",
//				},
//			},
//			existingSvc: &corev1.Service{
//				ObjectMeta: metav1.ObjectMeta{
//					Name:      "mock",
//					Namespace: "mockns",
//				},
//				Spec: corev1.ServiceSpec{
//					ExternalName: "example.com",
//				},
//			},
//			expectedConditionStatus: metav1.ConditionTrue,
//			expectedConditionReason: "Reconciled",
//			expectErr:               false,
//		},
//	}
//
//	for _, test := range tests {
//		mock := testutil.NewTestMock(t)
//		if test.existingSvc != nil {
//			mock = testutil.NewTestMock(t, test.existingSvc)
//		}
//		r := &VpcEndpointReconciler{
//			Client: mock.Client,
//			Scheme: mock.Client.Scheme(),
//			log:    testr.New(t),
//			clusterInfo: &clusterInfo{
//				domainName: testutil.MockDomainName,
//			},
//		}
//		t.Run(test.name, func(t *testing.T) {
//			err := r.validateExternalNameService(context.TODO(), test.resource)
//			if test.expectErr {
//				assert.Error(t, err)
//			} else {
//				assert.NoError(t, err)
//
//				condition := meta.FindStatusCondition(test.resource.Status.Conditions, avov1alpha2.ExternalNameServiceCondition)
//				assert.NotNilf(t, condition, "missing expected %s condition", avov1alpha2.ExternalNameServiceCondition)
//				assert.Equal(t, test.expectedConditionStatus, condition.Status)
//				assert.Equal(t, test.expectedConditionReason, condition.Reason)
//			}
//		})
//	}
//}
