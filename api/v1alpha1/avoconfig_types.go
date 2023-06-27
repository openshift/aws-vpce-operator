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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cfg "sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
)

//+kubebuilder:object:root=true

// AvoConfig is the Schema for the avoconfigs API
type AvoConfig struct {
	metav1.TypeMeta `json:",inline"`

	// ControllerManagerConfigurationSpec returns the configurations for controllers
	cfg.ControllerManagerConfigurationSpec `json:",inline"`

	// EnableVpcEndpointController is a feature flag to determine whether the VpcEndpoint controller runs.
	// Defaults to true
	EnableVpcEndpointController *bool `json:"enableVpcEndpointController,omitempty"`

	// EnableVpcEndpointAcceptanceController is a feature flag to determine whether the VpcEndpointAcceptance controller runs
	// Defaults to false
	EnableVpcEndpointAcceptanceController *bool `json:"enableVpcEndpointAcceptanceController,omitempty"`

	// EnableVpcEndpointTemplateController is a feature flag to determine whether the VpcEndpointTemplate controller runs
	// Defaults to false
	EnableVpcEndpointTemplateController *bool `json:"enableVpcEndpointTemplateController,omitempty"`
}

//+kubebuilder:object:root=true

// AvoConfigList contains a list of AvoConfig
type AvoConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AvoConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AvoConfig{})
}
