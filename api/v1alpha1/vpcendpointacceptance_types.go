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
)

type VpcEndpointService struct {
	// +kubebuilder:validation:MinLength=0

	// Id is the AWS ID of the VPC Endpoint Service for this controller to poll
	Id string `json:"id"`

	// +kubebuilder:validation:MinLength=0

	// ExpectedVpceName is the name of the VpcEndpoint CR that should be generating acceptance requests
	ExpectedVpceName string `json:"expectedVpceName"`
}

// VpcEndpointAcceptanceSpec defines the desired state of VpcEndpointAcceptance
type VpcEndpointAcceptanceSpec struct {
	// ServiceIds is a slice of VPC Endpoint Service IDs for this controller to poll
	// for VPC Endpoints in a pendingAcceptance state
	VpcEndpointServices []VpcEndpointService `json:"vpcEndpointServices,omitempty"`
}

// VpcEndpointAcceptanceStatus defines the observed state of VpcEndpointAcceptance
type VpcEndpointAcceptanceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VpcEndpointAcceptance is the Schema for the vpcendpointacceptances API
type VpcEndpointAcceptance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VpcEndpointAcceptanceSpec   `json:"spec,omitempty"`
	Status VpcEndpointAcceptanceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VpcEndpointAcceptanceList contains a list of VpcEndpointAcceptance
type VpcEndpointAcceptanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VpcEndpointAcceptance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VpcEndpointAcceptance{}, &VpcEndpointAcceptanceList{})
}
