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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringSliceTwoWayDiff(t *testing.T) {
	tests := []struct {
		firstSlice       []string
		secondSlice      []string
		expectedToCreate []string
		expectedToDelete []string
	}{
		{
			firstSlice:       []string{"a"},
			secondSlice:      []string{"a"},
			expectedToCreate: []string{},
			expectedToDelete: []string{},
		},
		{
			firstSlice:       []string{"a"},
			secondSlice:      []string{},
			expectedToCreate: []string{},
			expectedToDelete: []string{"a"},
		},
		{
			firstSlice:       []string{},
			secondSlice:      []string{"a"},
			expectedToCreate: []string{"a"},
			expectedToDelete: []string{},
		},
		{
			firstSlice:       []string{"a"},
			secondSlice:      []string{"b"},
			expectedToCreate: []string{"b"},
			expectedToDelete: []string{"a"},
		},
	}

	for _, test := range tests {
		toCreate, toDelete := StringSliceTwoWayDiff(test.firstSlice, test.secondSlice)
		assert.Equal(t, len(test.expectedToCreate), len(toCreate))
		for _, val := range test.expectedToCreate {
			contains := false
			for _, val2 := range toCreate {
				if val == val2 {
					contains = true
					break
				}
			}
			if !contains {
				t.Errorf("Expected to create %v, but did not", val)
			}
		}
		assert.Equal(t, len(test.expectedToDelete), len(toDelete))
		for _, val := range test.expectedToDelete {
			contains := false
			for _, val2 := range toDelete {
				if val == val2 {
					contains = true
					break
				}
			}
			if !contains {
				t.Errorf("Expected to delete %v, but did not", val)
			}
		}
	}
}
