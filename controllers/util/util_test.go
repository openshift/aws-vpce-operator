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

package util

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultAVORateLimiter(t *testing.T) {
	limiter := DefaultAVORateLimiter()

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

func TestAWSEnvVarReadyzChecker(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		expectErr bool
	}{
		{
			name: "non-STS",
			env: map[string]string{
				"AWS_SECRET_ACCESS_KEY": "A",
				"AWS_ACCESS_KEY_ID":     "B",
			},
			expectErr: false,
		},
		{
			name: "STS",
			env: map[string]string{
				"AWS_WEB_IDENTITY_TOKEN_FILE": "A",
				"AWS_ROLE_ARN":                "B",
			},
			expectErr: false,
		},
		{
			name: "insufficient",
			env: map[string]string{
				"AWS_SECRET_ACCESS_KEY": "A",
				"SOMETHING_ELSE":        "B",
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for k, v := range test.env {
				t.Setenv(k, v)
			}
			actual := AWSEnvVarReadyzChecker(&http.Request{})
			if actual != nil {
				if !test.expectErr {
					t.Fatalf("expected no error, got %s", actual)
				}
			} else {
				if test.expectErr {
					t.Fatalf("expected error, got nil")
				}
			}
		})
	}
}
