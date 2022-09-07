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

// StringSliceTwoWayDiff returns the objects in expected that are not in current (to be used for creation)
// and the objects in current that are not in expected (to be used for deletion).
func StringSliceTwoWayDiff(current, expected []string) ([]string, []string) {
	currentMap := map[string]bool{}
	for _, val := range current {
		currentMap[val] = true
	}

	expectedMap := map[string]bool{}
	for _, val := range expected {
		expectedMap[val] = true
	}

	var (
		toCreate []string
		toDelete []string
	)
	for k := range currentMap {
		if !expectedMap[k] {
			toDelete = append(toDelete, k)
		}
	}

	for k := range expectedMap {
		if !currentMap[k] {
			toCreate = append(toCreate, k)
		}
	}
	return toCreate, toDelete
}
