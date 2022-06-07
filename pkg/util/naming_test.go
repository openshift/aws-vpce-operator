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
	"fmt"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestGenerateAwsTags(t *testing.T) {
	tests := []struct {
		name          string
		clusterTagKey string
		expectErr     bool
	}{
		{
			name:          "",
			clusterTagKey: "",
			expectErr:     true,
		},
		{
			name:          "cluster",
			clusterTagKey: "kubernetes.io/cluster/infra",
			expectErr:     false,
		},
	}

	for _, test := range tests {
		_, err := GenerateAwsTags(test.name, test.clusterTagKey)
		if test.expectErr {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
		}
	}
}

func TestGetClusterTagKey(t *testing.T) {
	tests := []struct {
		infraName string
		expected  string
		expectErr bool
	}{
		{
			infraName: "",
			expectErr: true,
		},
		{
			infraName: "infra",
			expected:  "kubernetes.io/cluster/infra",
			expectErr: false,
		},
	}

	for _, test := range tests {
		actual, err := GetClusterTagKey(test.infraName)
		if test.expectErr {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.expected, actual)
		}
	}
}

func TestGenerateSecurityGroupName(t *testing.T) {
	tests := []struct {
		clusterName string
		purpose     string
		expected    string
		expectErr   bool
	}{
		{
			clusterName: "cluster",
			purpose:     "test",
			expected:    "cluster-test-sg",
			expectErr:   false,
		},
		{
			clusterName: "cluster",
			purpose:     strings.Repeat("a", 255),
			expected:    fmt.Sprintf("cluster-%s-sg", strings.Repeat("a", 244)),
			expectErr:   false,
		},
	}

	for _, test := range tests {
		actual, err := GenerateSecurityGroupName(test.clusterName, test.purpose)
		if test.expectErr {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.expected, actual)
		}
	}
}

func TestGenerateVPCEndpointName(t *testing.T) {
	tests := []struct {
		clusterName string
		purpose     string
		expected    string
		expectErr   bool
	}{
		{
			clusterName: "cluster",
			purpose:     "test",
			expected:    "cluster-test-vpce",
			expectErr:   false,
		},
		{
			clusterName: "cluster",
			purpose:     strings.Repeat("a", 255),
			expected:    fmt.Sprintf("cluster-%s-vpce", strings.Repeat("a", 242)),
			expectErr:   false,
		},
	}

	for _, test := range tests {
		actual, err := GenerateVPCEndpointName(test.clusterName, test.purpose)
		if test.expectErr {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.expected, actual)
		}
	}
}
