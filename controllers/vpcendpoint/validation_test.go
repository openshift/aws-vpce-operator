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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
