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

package secrets

import (
	"context"
	"testing"

	"github.com/openshift/aws-vpce-operator/pkg/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	mockAWSAccessKeyId     = "mock_access_key_id"     //#nosec G101
	mockAWSSecretAccessKey = "mock_secret_access_key" //#nosec G101
)

func TestParseAWSCredentialOverride(t *testing.T) {
	tests := []struct {
		secret    *corev1.Secret
		expectErr bool
	}{
		{
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "override",
					Namespace: "override-ns",
				},
				Data: map[string][]byte{
					defaultAWSAccessKeyId:     []byte(mockAWSAccessKeyId),
					defaultAWSSecretAccessKey: []byte(mockAWSSecretAccessKey),
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			mock := testutil.NewTestMock(t, test.secret)
			ref := &corev1.SecretReference{
				Name:      test.secret.Name,
				Namespace: test.secret.Namespace,
			}
			cfg, err := ParseAWSCredentialOverride(context.TODO(), mock.Client, "us-east-1", ref)
			if test.expectErr {
				if err == nil {
					t.Errorf("expected err, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no err, got %v", err)
				}

				creds, err := cfg.Credentials.Retrieve(context.TODO())
				if err != nil {
					t.Errorf("unexpected err: %v", err)
				}

				if creds.AccessKeyID != mockAWSAccessKeyId {
					t.Errorf("expected %s, got %s", mockAWSAccessKeyId, creds.AccessKeyID)
				}

				if creds.SecretAccessKey != mockAWSSecretAccessKey {
					t.Errorf("expected %s, got %s", mockAWSSecretAccessKey, creds.SecretAccessKey)
				}
			}
		})
	}
}
