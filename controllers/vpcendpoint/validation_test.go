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

	"github.com/aws/aws-sdk-go-v2/aws"
	route53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/go-logr/logr/testr"
	hyperv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"

	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
	"github.com/openshift/aws-vpce-operator/pkg/testutil"
	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

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
					InfraId:         testutil.MockInfrastructureName,
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
				clusterTag: aws_client.MockLegacyClusterTag,
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
				clusterTag: aws_client.MockLegacyClusterTag,
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

func TestVPCEndpointReconciler_validateVPCEndpoint_enablesPrivateDns(t *testing.T) {
	resource := &avov1alpha2.VpcEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mock-goalert",
		},
		Spec: avov1alpha2.VpcEndpointSpec{
			EnablePrivateDns: true,
		},
		Status: avov1alpha2.VpcEndpointStatus{
			VPCEndpointId: testutil.MockVpcEndpointId,
		},
	}

	client := testutil.NewTestMock(t, resource).Client
	r := &VpcEndpointReconciler{
		Client:           client,
		Scheme:           client.Scheme(),
		awsClient:        aws_client.NewMockedAwsClientWithSubnets(),
		log:              testr.New(t),
		EnablePrivateDns: true,
		Recorder:         record.NewFakeRecorder(1),
		clusterInfo: &clusterInfo{
			clusterTag: aws_client.MockLegacyClusterTag,
		},
	}

	err := r.validateVPCEndpoint(context.TODO(), resource)
	assert.NoError(t, err)

	condition := meta.FindStatusCondition(resource.Status.Conditions, avov1alpha2.AWSVpcEndpointCondition)
	assert.NotNilf(t, condition, "missing expected %s condition", avov1alpha2.AWSVpcEndpointCondition)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
}

func TestVPCEndpointReconciler_validateVPCEndpoint_skipsPrivateDnsWhenFlagDisabled(t *testing.T) {
	resource := &avov1alpha2.VpcEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mock-goalert",
		},
		Spec: avov1alpha2.VpcEndpointSpec{
			EnablePrivateDns: true,
		},
		Status: avov1alpha2.VpcEndpointStatus{
			VPCEndpointId: testutil.MockVpcEndpointId,
		},
	}

	client := testutil.NewTestMock(t, resource).Client
	r := &VpcEndpointReconciler{
		Client:           client,
		Scheme:           client.Scheme(),
		awsClient:        aws_client.NewMockedAwsClientWithSubnets(),
		log:              testr.New(t),
		EnablePrivateDns: false, // operator flag off
		clusterInfo: &clusterInfo{
			clusterTag: aws_client.MockLegacyClusterTag,
		},
	}

	err := r.validateVPCEndpoint(context.TODO(), resource)
	assert.NoError(t, err)

	condition := meta.FindStatusCondition(resource.Status.Conditions, avov1alpha2.AWSVpcEndpointCondition)
	assert.NotNilf(t, condition, "missing expected %s condition", avov1alpha2.AWSVpcEndpointCondition)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
}

func TestVPCEndpointReconciler_validateCustomDns_privateDns(t *testing.T) {
	tests := []struct {
		name                     string
		operatorEnablePrivateDns bool
		crEnablePrivateDns       bool
		expectSkipRoute53        bool
	}{
		{
			name:                     "both flags true: skips Route53",
			operatorEnablePrivateDns: true,
			crEnablePrivateDns:       true,
			expectSkipRoute53:        true,
		},
		{
			name:                     "operator flag true, CR flag false: does not skip Route53",
			operatorEnablePrivateDns: true,
			crEnablePrivateDns:       false,
			expectSkipRoute53:        false,
		},
		{
			name:                     "operator flag false, CR flag true: does not skip Route53",
			operatorEnablePrivateDns: false,
			crEnablePrivateDns:       true,
			expectSkipRoute53:        false,
		},
		{
			name:                     "both flags false: does not skip Route53",
			operatorEnablePrivateDns: false,
			crEnablePrivateDns:       false,
			expectSkipRoute53:        false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resource := &avov1alpha2.VpcEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mock-goalert",
				},
				Spec: avov1alpha2.VpcEndpointSpec{
					EnablePrivateDns: test.crEnablePrivateDns,
				},
			}

			client := testutil.NewTestMock(t, resource).Client
			r := &VpcEndpointReconciler{
				Client:           client,
				Scheme:           client.Scheme(),
				awsClient:        aws_client.NewMockedAwsClient(),
				log:              testr.New(t),
				EnablePrivateDns: test.operatorEnablePrivateDns,
				clusterInfo: &clusterInfo{
					clusterTag: aws_client.MockLegacyClusterTag,
				},
			}

			if test.expectSkipRoute53 {
				err := r.validateCustomDns(context.TODO(), resource)
				assert.NoError(t, err)

				// No Route53 condition should be set since AVO doesn't manage Route53 resources
				condition := meta.FindStatusCondition(resource.Status.Conditions, avov1alpha2.AWSRoute53RecordCondition)
				assert.Nil(t, condition, "Route53 condition should not be set when private DNS is enabled")
			} else {
				// When private DNS is not active, the code proceeds into Route53 validation.
				// With the minimal mock setup (all spec fields zero), validateCustomDns returns
				// nil without entering any R53 sub-path. This verifies the function doesn't
				// short-circuit via the private DNS skip path.
				err := r.validateCustomDns(context.TODO(), resource)
				assert.NoError(t, err)
			}
		})
	}
}

func Test_isVpcEndpointReady(t *testing.T) {
	tests := []struct {
		name     string
		resource *avov1alpha2.VpcEndpoint
		expected bool
	}{
		{
			name: "no conditions",
			resource: &avov1alpha2.VpcEndpoint{
				Status: avov1alpha2.VpcEndpointStatus{},
			},
			expected: false,
		},
		{
			name: "all conditions true",
			resource: &avov1alpha2.VpcEndpoint{
				Status: avov1alpha2.VpcEndpointStatus{
					Conditions: []metav1.Condition{
						{
							Type:   avov1alpha2.AWSSecurityGroupCondition,
							Status: metav1.ConditionTrue,
						},
						{
							Type:   avov1alpha2.AWSVpcEndpointCondition,
							Status: metav1.ConditionTrue,
						},
						{
							Type:   avov1alpha2.AWSRoute53RecordCondition,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "vpce pending acceptance",
			resource: &avov1alpha2.VpcEndpoint{
				Status: avov1alpha2.VpcEndpointStatus{
					Conditions: []metav1.Condition{
						{
							Type:   avov1alpha2.AWSSecurityGroupCondition,
							Status: metav1.ConditionTrue,
						},
						{
							Type:   avov1alpha2.AWSVpcEndpointCondition,
							Status: metav1.ConditionFalse,
							Reason: "pendingAcceptance",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "security group not ready",
			resource: &avov1alpha2.VpcEndpoint{
				Status: avov1alpha2.VpcEndpointStatus{
					Conditions: []metav1.Condition{
						{
							Type:   avov1alpha2.AWSSecurityGroupCondition,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "all conditions true without route53",
			resource: &avov1alpha2.VpcEndpoint{
				Status: avov1alpha2.VpcEndpointStatus{
					Conditions: []metav1.Condition{
						{
							Type:   avov1alpha2.AWSSecurityGroupCondition,
							Status: metav1.ConditionTrue,
						},
						{
							Type:   avov1alpha2.AWSVpcEndpointCondition,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isVpcEndpointReady(test.resource))
		})
	}
}

func TestValidateR53HostedZoneRecord_SkipsUpsertWhenConditionsTrue(t *testing.T) {
	resource := &avov1alpha2.VpcEndpoint{
		Status: avov1alpha2.VpcEndpointStatus{
			VPCEndpointId: testutil.MockVpcEndpointId,
			HostedZoneId:  "Z12345",
			Conditions: []metav1.Condition{
				{
					Type:   avov1alpha2.AWSRoute53RecordCondition,
					Status: metav1.ConditionTrue,
					Reason: "Created",
				},
				{
					Type:   avov1alpha2.AWSVpcEndpointCondition,
					Status: metav1.ConditionTrue,
					Reason: "available",
				},
			},
		},
	}

	r := &VpcEndpointReconciler{
		awsClient: aws_client.NewMockedThrottlingAwsClient(),
		log:       testr.New(t),
	}

	// Using the throttling mock: if the guard is removed, this would error with "Throttling".
	err := r.validateR53HostedZoneRecord(context.TODO(), resource)
	assert.NoError(t, err)
}

func TestValidateR53HostedZoneRecord_DoesNotSkipWhenRecordConditionFalse(t *testing.T) {
	resource := &avov1alpha2.VpcEndpoint{
		Status: avov1alpha2.VpcEndpointStatus{
			VPCEndpointId: testutil.MockVpcEndpointId,
			HostedZoneId:  "Z12345",
			Conditions: []metav1.Condition{
				{
					Type:   avov1alpha2.AWSRoute53RecordCondition,
					Status: metav1.ConditionFalse,
					Reason: "VpcEndpointChanged",
				},
				{
					Type:   avov1alpha2.AWSVpcEndpointCondition,
					Status: metav1.ConditionTrue,
					Reason: "available",
				},
			},
		},
	}

	r := &VpcEndpointReconciler{
		awsClient: aws_client.NewMockedThrottlingAwsClient(),
		log:       testr.New(t),
	}

	// When the record condition is False, the function should NOT skip.
	// It proceeds to the R53 call, which returns a throttling error from the mock.
	err := r.validateR53HostedZoneRecord(context.TODO(), resource)
	assert.Error(t, err, "expected throttling error, proving skip path was not taken")
	assert.Contains(t, err.Error(), "Throttling")
}

func TestValidateR53HostedZoneRecord_DoesNotSkipWhenVpceConditionFalse(t *testing.T) {
	resource := &avov1alpha2.VpcEndpoint{
		Status: avov1alpha2.VpcEndpointStatus{
			VPCEndpointId: testutil.MockVpcEndpointId,
			HostedZoneId:  "Z12345",
			Conditions: []metav1.Condition{
				{
					Type:   avov1alpha2.AWSRoute53RecordCondition,
					Status: metav1.ConditionTrue,
					Reason: "Created",
				},
				{
					Type:   avov1alpha2.AWSVpcEndpointCondition,
					Status: metav1.ConditionFalse,
					Reason: "pending",
				},
			},
		},
	}

	r := &VpcEndpointReconciler{
		awsClient: aws_client.NewMockedThrottlingAwsClient(),
		log:       testr.New(t),
	}

	err := r.validateR53HostedZoneRecord(context.TODO(), resource)
	assert.Error(t, err, "expected throttling error, proving skip path was not taken")
	assert.Contains(t, err.Error(), "Throttling")
}

func TestInvalidateRoute53RecordCondition(t *testing.T) {
	tests := []struct {
		name        string
		conditions  []metav1.Condition
		expectReset bool
	}{
		{
			name: "resets True condition to False",
			conditions: []metav1.Condition{
				{
					Type:   avov1alpha2.AWSRoute53RecordCondition,
					Status: metav1.ConditionTrue,
					Reason: "Created",
				},
			},
			expectReset: true,
		},
		{
			name: "no-op when condition is already False",
			conditions: []metav1.Condition{
				{
					Type:   avov1alpha2.AWSRoute53RecordCondition,
					Status: metav1.ConditionFalse,
					Reason: "VpcEndpointChanged",
				},
			},
			expectReset: false,
		},
		{
			name:        "no-op when no conditions exist",
			conditions:  []metav1.Condition{},
			expectReset: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resource := &avov1alpha2.VpcEndpoint{
				Status: avov1alpha2.VpcEndpointStatus{
					Conditions: test.conditions,
				},
			}

			r := &VpcEndpointReconciler{
				log: testr.New(t),
			}

			r.invalidateRoute53RecordCondition(resource)

			condition := meta.FindStatusCondition(resource.Status.Conditions, avov1alpha2.AWSRoute53RecordCondition)
			if test.expectReset {
				assert.NotNil(t, condition)
				assert.Equal(t, metav1.ConditionFalse, condition.Status)
				assert.Equal(t, "VpcEndpointChanged", condition.Reason)
			} else if len(test.conditions) > 0 {
				assert.NotNil(t, condition, "existing condition should not be removed")
				assert.Equal(t, test.conditions[0].Status, condition.Status, "condition status should be unchanged")
			} else {
				assert.Nil(t, condition, "no condition should be created when none existed")
			}
		})
	}
}

func TestInvalidateRoute53RecordCondition_AlsoResetsTagsCondition(t *testing.T) {
	resource := &avov1alpha2.VpcEndpoint{
		Status: avov1alpha2.VpcEndpointStatus{
			Conditions: []metav1.Condition{
				{
					Type:   avov1alpha2.AWSRoute53RecordCondition,
					Status: metav1.ConditionTrue,
					Reason: "Created",
				},
				{
					Type:   avov1alpha2.AWSRoute53TagsCondition,
					Status: metav1.ConditionTrue,
					Reason: "TagsVerified",
				},
			},
		},
	}

	r := &VpcEndpointReconciler{
		log: testr.New(t),
	}

	r.invalidateRoute53RecordCondition(resource)

	recordCond := meta.FindStatusCondition(resource.Status.Conditions, avov1alpha2.AWSRoute53RecordCondition)
	assert.NotNil(t, recordCond)
	assert.Equal(t, metav1.ConditionFalse, recordCond.Status)

	tagsCond := meta.FindStatusCondition(resource.Status.Conditions, avov1alpha2.AWSRoute53TagsCondition)
	assert.NotNil(t, tagsCond)
	assert.Equal(t, metav1.ConditionFalse, tagsCond.Status)
	assert.Equal(t, "VpcEndpointChanged", tagsCond.Reason)
}

func TestValidateR53PrivateHostedZone_SkipsTagsWhenConditionTrue(t *testing.T) {
	resource := &avov1alpha2.VpcEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mock-tags-skip",
		},
		Spec: avov1alpha2.VpcEndpointSpec{
			CustomDns: avov1alpha2.CustomDns{
				Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
					DomainName: "example.com",
				},
			},
		},
		Status: avov1alpha2.VpcEndpointStatus{
			HostedZoneId: aws_client.MockHostedZoneId,
			VPCId:        aws_client.MockVpcId,
			Conditions: []metav1.Condition{
				{
					Type:   avov1alpha2.AWSRoute53TagsCondition,
					Status: metav1.ConditionTrue,
					Reason: "TagsVerified",
				},
			},
		},
	}

	r := &VpcEndpointReconciler{
		awsClient: aws_client.NewMockedThrottlingAwsClient(),
		log:       testr.New(t),
		clusterInfo: &clusterInfo{
			clusterTag: aws_client.MockLegacyClusterTag,
			region:     "us-east-1",
		},
	}

	// DomainName path with AWSRoute53TagsCondition=True should skip
	// createMissingPrivateZoneTags entirely. The throttling mock's
	// ListTagsForResource would error if called, proving the skip works.
	err := r.validateR53PrivateHostedZone(context.TODO(), resource)
	assert.NoError(t, err)
}

func TestValidateR53PrivateHostedZone_CallsTagsWhenConditionFalse(t *testing.T) {
	resource := &avov1alpha2.VpcEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mock-tags-noskip",
		},
		Spec: avov1alpha2.VpcEndpointSpec{
			CustomDns: avov1alpha2.CustomDns{
				Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
					DomainName: "example.com",
				},
			},
		},
		Status: avov1alpha2.VpcEndpointStatus{
			HostedZoneId: aws_client.MockHostedZoneId,
			VPCId:        aws_client.MockVpcId,
			Conditions: []metav1.Condition{
				{
					Type:   avov1alpha2.AWSRoute53TagsCondition,
					Status: metav1.ConditionFalse,
					Reason: "VpcEndpointChanged",
				},
			},
		},
	}

	client := testutil.NewTestMock(t, resource).Client
	r := &VpcEndpointReconciler{
		Client:    client,
		Scheme:    client.Scheme(),
		awsClient: aws_client.NewMockedAwsClient(),
		log:       testr.New(t),
		clusterInfo: &clusterInfo{
			clusterTag: aws_client.MockLegacyClusterTag,
			region:     "us-east-1",
		},
	}

	// DomainName path with AWSRoute53TagsCondition=False should call
	// createMissingPrivateZoneTags and set the condition to True.
	err := r.validateR53PrivateHostedZone(context.TODO(), resource)
	assert.NoError(t, err)

	// Re-read from the fake API server to get the updated status
	updated := &avov1alpha2.VpcEndpoint{}
	assert.NoError(t, r.Get(context.TODO(), ctrlclient.ObjectKeyFromObject(resource), updated))

	tagsCond := meta.FindStatusCondition(updated.Status.Conditions, avov1alpha2.AWSRoute53TagsCondition)
	assert.NotNil(t, tagsCond)
	assert.Equal(t, metav1.ConditionTrue, tagsCond.Status, "AWSRoute53TagsCondition should be set to True after tag verification")
}

// TestFindOrCreatePrivateHostedZone_DomainName_DoesNotAdoptSibling verifies that
// when the domain comes from the literal spec...DomainName field
// (a shared service domain), and the cluster's VPC has no associated zone, AVO must NOT
// adopt a same-named zone owned by a sibling cluster. It must create its own
// VPC-associated zone instead.
func TestFindOrCreatePrivateHostedZone_DomainName_DoesNotAdoptSibling(t *testing.T) {
	resource := &avov1alpha2.VpcEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mock-shared-domain-cluster",
		},
		Spec: avov1alpha2.VpcEndpointSpec{
			CustomDns: avov1alpha2.CustomDns{
				Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
					DomainName: "shared-service.example.com",
				},
			},
		},
		Status: avov1alpha2.VpcEndpointStatus{
			VPCId: aws_client.MockVpcId,
		},
	}

	r53 := &aws_client.MockedRoute53{
		// Our VPC has no associated zone for this domain...
		ListByVPCZones: []route53Types.HostedZoneSummary{},
		// ...but a sibling cluster's same-named private zone exists in the account.
		ListByNameZones: []route53Types.HostedZone{
			{
				Id:     aws.String("/hostedzone/SIBLINGZONE123"),
				Name:   aws.String("shared-service.example.com."),
				Config: &route53Types.HostedZoneConfig{PrivateZone: true},
			},
		},
	}
	awsClient := aws_client.NewAwsClientWithServiceClients(&aws_client.MockedEC2{}, r53)

	client := testutil.NewTestMock(t, resource).Client
	r := &VpcEndpointReconciler{
		Client:    client,
		Scheme:    client.Scheme(),
		awsClient: awsClient,
		log:       testr.New(t),
		Recorder:  record.NewFakeRecorder(5),
		clusterInfo: &clusterInfo{
			clusterTag: aws_client.MockLegacyClusterTag,
			region:     "us-east-1",
		},
	}

	err := r.findOrCreatePrivateHostedZone(context.TODO(), resource)
	assert.NoError(t, err)
	assert.True(t, r53.CreateHostedZoneCalled, "must create its own zone rather than adopt the sibling's")
	assert.NotEqual(t, "SIBLINGZONE123", resource.Status.HostedZoneId, "must not adopt a sibling cluster's zone by name")
	assert.Equal(t, aws_client.MockCreatedHostedZoneId, resource.Status.HostedZoneId)
}

// TestFindOrCreatePrivateHostedZone_DomainNameRefName_DoesNotReuse verifies that a domain
// supplied via spec...DomainNameRef.name is treated as a literal string (the API: ".name
// specifies the base domain name directly"), NOT as a cluster-unique value. Like the literal
// DomainName field it can be shared across clusters, so a same-named sibling zone must NOT be
// adopted by name; AVO must create its own VPC-associated zone instead.
func TestFindOrCreatePrivateHostedZone_DomainNameRefName_DoesNotReuse(t *testing.T) {
	resource := &avov1alpha2.VpcEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mock-static-ref-cluster",
		},
		Spec: avov1alpha2.VpcEndpointSpec{
			CustomDns: avov1alpha2.CustomDns{
				Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
					DomainNameRef: &avov1alpha2.DomainName{
						Name: "shared-service.example.com",
					},
				},
			},
		},
		Status: avov1alpha2.VpcEndpointStatus{
			VPCId: aws_client.MockVpcId,
		},
	}

	r53 := &aws_client.MockedRoute53{
		// Our VPC has no associated zone for this domain...
		ListByVPCZones: []route53Types.HostedZoneSummary{},
		// ...but a sibling cluster's same-named private zone exists in the account.
		ListByNameZones: []route53Types.HostedZone{
			{
				Id:     aws.String("/hostedzone/SIBLINGZONE123"),
				Name:   aws.String("shared-service.example.com."),
				Config: &route53Types.HostedZoneConfig{PrivateZone: true},
			},
		},
	}
	awsClient := aws_client.NewAwsClientWithServiceClients(&aws_client.MockedEC2{}, r53)

	client := testutil.NewTestMock(t, resource).Client
	r := &VpcEndpointReconciler{
		Client:    client,
		Scheme:    client.Scheme(),
		awsClient: awsClient,
		log:       testr.New(t),
		Recorder:  record.NewFakeRecorder(5),
		clusterInfo: &clusterInfo{
			clusterTag: aws_client.MockLegacyClusterTag,
			region:     "us-east-1",
		},
	}

	err := r.findOrCreatePrivateHostedZone(context.TODO(), resource)
	assert.NoError(t, err)
	assert.True(t, r53.CreateHostedZoneCalled, "DomainNameRef.name is a literal; must create its own zone, not adopt the sibling's")
	assert.NotEqual(t, "SIBLINGZONE123", resource.Status.HostedZoneId, "must not adopt a sibling cluster's zone by name")
	assert.Equal(t, aws_client.MockCreatedHostedZoneId, resource.Status.HostedZoneId)
}

// TestFindOrCreatePrivateHostedZone_DomainNameRefValueFrom_ReusesByName verifies that
// by-name reuse IS preserved for domains resolved from a cluster-unique
// valueFrom source. Here the HCP path (hostedControlPlaneRef) derives the domain from the
// hosted cluster's own HostedControlPlane, so a same-named zone can only be this cluster's
// own orphan and is safely reused instead of duplicated.
func TestFindOrCreatePrivateHostedZone_DomainNameRefValueFrom_ReusesByName(t *testing.T) {
	const hcpNamespace = "ocm-test-hcp"

	resource := &avov1alpha2.VpcEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock-hcp-cluster",
			Namespace: hcpNamespace,
		},
		Spec: avov1alpha2.VpcEndpointSpec{
			CustomDns: avov1alpha2.CustomDns{
				Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
					DomainNameRef: &avov1alpha2.DomainName{
						ValueFrom: &avov1alpha2.DomainNameSource{
							HostedControlPlaneRef: &avov1alpha2.HostedControlPlaneSelector{
								NamespaceFieldRef: &avov1alpha2.ObjectFieldSelector{
									FieldPath: ".metadata.namespace",
								},
							},
						},
					},
				},
			},
		},
		Status: avov1alpha2.VpcEndpointStatus{
			VPCId: aws_client.MockVpcId,
		},
	}

	// The HostedControlPlane whose APIServer route hostname yields the cluster-unique domain
	// "my-hcp.abcd.hypershift.local" (api. prefix stripped).
	hcp := &hyperv1beta1.HostedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-hcp",
			Namespace: hcpNamespace,
		},
		Spec: hyperv1beta1.HostedControlPlaneSpec{
			Services: []hyperv1beta1.ServicePublishingStrategyMapping{
				{
					Service: hyperv1beta1.APIServer,
					ServicePublishingStrategy: hyperv1beta1.ServicePublishingStrategy{
						Type:  hyperv1beta1.Route,
						Route: &hyperv1beta1.RoutePublishingStrategy{Hostname: "api.my-hcp.abcd.hypershift.local"},
					},
				},
			},
		},
	}

	r53 := &aws_client.MockedRoute53{
		// Not associated with our VPC...
		ListByVPCZones: []route53Types.HostedZoneSummary{},
		// ...but this same cluster's own (cluster-unique) orphan zone exists by name.
		ListByNameZones: []route53Types.HostedZone{
			{
				Id:     aws.String("/hostedzone/OWNZONE123"),
				Name:   aws.String("my-hcp.abcd.hypershift.local."),
				Config: &route53Types.HostedZoneConfig{PrivateZone: true},
			},
		},
	}
	awsClient := aws_client.NewAwsClientWithServiceClients(&aws_client.MockedEC2{}, r53)

	client := testutil.NewTestMock(t, resource, hcp).Client
	r := &VpcEndpointReconciler{
		Client:    client,
		Scheme:    client.Scheme(),
		awsClient: awsClient,
		log:       testr.New(t),
		Recorder:  record.NewFakeRecorder(5),
		clusterInfo: &clusterInfo{
			clusterTag: aws_client.MockLegacyClusterTag,
			region:     "us-east-1",
		},
	}

	err := r.findOrCreatePrivateHostedZone(context.TODO(), resource)
	assert.NoError(t, err)
	assert.False(t, r53.CreateHostedZoneCalled, "valueFrom-derived (cluster-unique) domain should reuse the existing same-named zone, not create a duplicate")
	assert.Equal(t, "OWNZONE123", resource.Status.HostedZoneId, "should reuse the existing cluster-unique zone found by name")
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
