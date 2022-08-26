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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/go-logr/logr/testr"
	avov1alpha1 "github.com/openshift/aws-vpce-operator/api/v1alpha1"
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

func TestVpcEndpointReconciler_parseClusterInfo(t *testing.T) {
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

func TestVpcEndpointReconciler_findOrCreateSecurityGroup(t *testing.T) {
	tests := []struct {
		name        string
		resource    *avov1alpha1.VpcEndpoint
		clusterInfo *clusterInfo
		expectErr   bool
	}{
		{
			name: "SecurityGroupID populated",
			resource: &avov1alpha1.VpcEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mock1",
				},
				Status: avov1alpha1.VpcEndpointStatus{
					SecurityGroupId: aws_client.MockSecurityGroupId,
				},
			},
			expectErr: false,
		},
		{
			name: "SecurityGroupID missing",
			resource: &avov1alpha1.VpcEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mock2",
				},
				Status: avov1alpha1.VpcEndpointStatus{},
			},
			clusterInfo: &clusterInfo{
				infraName: testutil.MockInfrastructureName,
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		r := &VpcEndpointReconciler{
			Client:      testutil.NewTestMock(t, test.resource).Client,
			Scheme:      testutil.NewTestMock(t).Client.Scheme(),
			log:         testr.New(t),
			awsClient:   aws_client.NewMockedAwsClient(),
			clusterInfo: test.clusterInfo,
		}
		t.Run(test.name, func(t *testing.T) {
			_, err := r.findOrCreateSecurityGroup(context.TODO(), test.resource)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVpcEndpointReconciler_findOrCreateVpcEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		resource    *avov1alpha1.VpcEndpoint
		clusterInfo *clusterInfo
		expectErr   bool
	}{
		{
			name: "VPCEndpointID populated",
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
		{
			name: "VPCEndpointID missing",
			resource: &avov1alpha1.VpcEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mock2",
				},
				Status: avov1alpha1.VpcEndpointStatus{},
			},
			clusterInfo: &clusterInfo{
				clusterTag: aws_client.MockClusterTag,
				infraName:  testutil.MockInfrastructureName,
				vpcId:      aws_client.MockVpcId,
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		r := &VpcEndpointReconciler{
			Client:      testutil.NewTestMock(t, test.resource).Client,
			Scheme:      testutil.NewTestMock(t).Client.Scheme(),
			log:         testr.New(t),
			awsClient:   aws_client.NewMockedAwsClient(),
			clusterInfo: test.clusterInfo,
		}
		t.Run(test.name, func(t *testing.T) {
			_, err := r.findOrCreateVpcEndpoint(context.TODO(), test.resource)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equalf(t, "available", test.resource.Status.Status, "expected state to be %s, got %s", "available", test.resource.Status.Status)
			}
		})
	}
}

func TestVpcEndpointReconciler_diffVpcEndpointSubnets(t *testing.T) {
	tests := []struct {
		name                string
		vpce                *ec2.VpcEndpoint
		clusterTag          string
		expectedNumToAdd    int
		expectedNumToRemove int
		expectErr           bool
	}{
		{
			name:      "nil",
			vpce:      nil,
			expectErr: true,
		},
		{
			name:       "exact match",
			clusterTag: aws_client.MockClusterTag,
			vpce: &ec2.VpcEndpoint{
				SubnetIds: []*string{aws.String(aws_client.MockPrivateSubnetId)},
			},
			expectedNumToAdd:    0,
			expectedNumToRemove: 0,
			expectErr:           false,
		},
		{
			name:                "subnet addition needed",
			clusterTag:          aws_client.MockClusterTag,
			vpce:                &ec2.VpcEndpoint{},
			expectedNumToAdd:    1,
			expectedNumToRemove: 0,
			expectErr:           false,
		},
		{
			name:       "subnet removal needed",
			clusterTag: "some/random/tag",
			vpce: &ec2.VpcEndpoint{
				SubnetIds: []*string{aws.String(aws_client.MockPrivateSubnetId)},
			},
			expectedNumToAdd:    0,
			expectedNumToRemove: 1,
			expectErr:           false,
		},
	}

	for _, test := range tests {
		r := &VpcEndpointReconciler{
			awsClient:   aws_client.NewMockedAwsClientWithSubnets(),
			log:         testr.New(t),
			clusterInfo: &clusterInfo{clusterTag: test.clusterTag},
		}
		t.Run(test.name, func(t *testing.T) {
			actualToAdd, actualToRemove, err := r.diffVpcEndpointSubnets(test.vpce)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equalf(t, test.expectedNumToAdd, len(actualToAdd), "expected %d to add, got %d", test.expectedNumToAdd, len(actualToAdd))
				assert.Equalf(t, test.expectedNumToRemove, len(actualToRemove), "expected %d to remove, got %d", test.expectedNumToRemove, len(actualToRemove))
			}
		})
	}
}

func TestVpcEndpointReconciler_diffVpcEndpointSecurityGroups(t *testing.T) {
	tests := []struct {
		name                string
		resource            *avov1alpha1.VpcEndpoint
		vpce                *ec2.VpcEndpoint
		expectedNumToAdd    int
		expectedNumToRemove int
	}{
		{
			name: "exact match",
			resource: &avov1alpha1.VpcEndpoint{
				Status: avov1alpha1.VpcEndpointStatus{
					SecurityGroupId: aws_client.MockSecurityGroupId,
				},
			},
			vpce: &ec2.VpcEndpoint{
				Groups: []*ec2.SecurityGroupIdentifier{
					{
						GroupId: aws.String(aws_client.MockSecurityGroupId),
					},
				},
			},
			expectedNumToAdd:    0,
			expectedNumToRemove: 0,
		},
		{
			name: "need to add and remove",
			resource: &avov1alpha1.VpcEndpoint{
				Status: avov1alpha1.VpcEndpointStatus{
					SecurityGroupId: aws_client.MockSecurityGroupId,
				},
			},
			vpce: &ec2.VpcEndpoint{
				Groups: []*ec2.SecurityGroupIdentifier{
					{
						GroupId: aws.String("sg-extra-to-remove"),
					},
				},
			},
			expectedNumToAdd:    1,
			expectedNumToRemove: 1,
		},
	}

	for _, test := range tests {
		r := &VpcEndpointReconciler{
			Client:      nil,
			Scheme:      nil,
			log:         testr.New(t),
			awsClient:   nil,
			clusterInfo: nil,
		}
		t.Run(test.name, func(t *testing.T) {
			actualToAdd, actualToRemove, err := r.diffVpcEndpointSecurityGroups(test.vpce, test.resource)

			assert.NoError(t, err)
			assert.Equalf(t, test.expectedNumToAdd, len(actualToAdd), "expected to add %d, got %d", test.expectedNumToAdd, len(actualToAdd))
			assert.Equalf(t, test.expectedNumToRemove, len(actualToRemove), "expected to remove %d, got %d", test.expectedNumToRemove, len(actualToRemove))
		})
	}
}

func TestVpcEndpointReconciler_generateRoute53Record(t *testing.T) {
	tests := []struct {
		resource  *avov1alpha1.VpcEndpoint
		expectErr bool
	}{
		{
			resource: &avov1alpha1.VpcEndpoint{
				Status: avov1alpha1.VpcEndpointStatus{
					VPCEndpointId: testutil.MockVpcEndpointId,
				},
			},
			expectErr: false,
		},
		{
			resource:  &avov1alpha1.VpcEndpoint{},
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
		_, err = r.generateRoute53Record(test.resource)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestVpcEndpointReconciler_generateExternalNameService(t *testing.T) {
	var (
		trueBool = true
	)
	tests := []struct {
		resource   *avov1alpha1.VpcEndpoint
		domainName string
		expected   *corev1.Service
		expectErr  bool
	}{
		{
			resource: &avov1alpha1.VpcEndpoint{
				Spec: avov1alpha1.VpcEndpointSpec{
					SubdomainName: "demo",
					ExternalNameService: avov1alpha1.ExternalNameServiceSpec{
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
			resource: &avov1alpha1.VpcEndpoint{
				Spec: avov1alpha1.VpcEndpointSpec{
					ExternalNameService: avov1alpha1.ExternalNameServiceSpec{
						Name:      "demo",
						Namespace: "demo-ns",
					},
				},
			},
			domainName: "my.cluster.com",
			expectErr:  true,
		},
		{
			resource: &avov1alpha1.VpcEndpoint{
				Spec: avov1alpha1.VpcEndpointSpec{
					SubdomainName: "demo",
					ExternalNameService: avov1alpha1.ExternalNameServiceSpec{
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

		actual, err := r.generateExternalNameService(test.resource)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		}
	}
}

func TestTagsContains(t *testing.T) {
	tests := []struct {
		name        string
		tags        []*ec2.Tag
		tagsToCheck map[string]string
		expected    bool
	}{
		{
			name:        "empty set",
			tagsToCheck: map[string]string{},
			expected:    true,
		},
		{
			name: "contains subset",
			tags: []*ec2.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("val1"),
				},
				{
					Key:   aws.String("key2"),
					Value: aws.String("val2"),
				},
			},
			tagsToCheck: map[string]string{
				"key1": "val1",
			},
			expected: true,
		},
		{
			name: "missing",
			tags: []*ec2.Tag{
				{
					Key:   aws.String("key1"),
					Value: aws.String("val1"),
				},
				{
					Key:   aws.String("key2"),
					Value: aws.String("val2"),
				},
			},
			tagsToCheck: map[string]string{
				"key1": "val1",
				"key3": "val3",
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := tagsContains(test.tags, test.tagsToCheck)
			assert.Equal(t, test.expected, actual)
		})
	}
}
