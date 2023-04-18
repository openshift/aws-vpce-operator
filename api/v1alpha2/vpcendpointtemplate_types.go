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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VpceTemplateSpec describes the data a VpcEndpoint should have when created from a template
type VpceTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the VpcEndpoint.
	Spec VpcEndpointSpec `json:"spec"`
}

type VpcEndpointTemplateType string

const (
	// HCPVpcEndpointTemplateType classifies this VpcEndpointTemplate as one that applies to HyperShift HostedControlPlanes
	HCPVpcEndpointTemplateType VpcEndpointTemplateType = "HostedControlPlane"
)

// VpcEndpointTemplateSpec defines the desired state of VpcEndpointTemplate
type VpcEndpointTemplateSpec struct {
	// Type allows us to make different VpcEndpointTemplates that watch various kinds
	Type VpcEndpointTemplateType `json:"type"`

	// Select is a label selector for VpcEndpoints. Existing VpcEndpoints with this label
	// will be affected by this VpcEndpointTemplate. It must match the VpcEndpoint
	// template's labels.

	// A label selector is a label query over a set of resources. The result of
	// matchLabels and matchExpressions are ANDed. An empty label selector matches
	// all objects. A null label selector matches no objects.
	Selector metav1.LabelSelector `json:"selector"`

	// Template describes the VpcEndpoints that will be created.
	Template VpceTemplateSpec `json:"template"`
}

// VpcEndpointTemplateStatus defines the observed state of VpcEndpointTemplate
type VpcEndpointTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName={vpcet},scope="Namespaced"
//+kubebuilder:subresource:status

// VpcEndpointTemplate is the Schema for the vpcendpointtemplates API
type VpcEndpointTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VpcEndpointTemplateSpec   `json:"spec,omitempty"`
	Status VpcEndpointTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VpcEndpointTemplateList contains a list of VpcEndpointTemplate
type VpcEndpointTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VpcEndpointTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VpcEndpointTemplate{}, &VpcEndpointTemplateList{})
}
