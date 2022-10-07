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

type AcceptanceCriteria struct {
	// AwsAccountOperatorAccount will accept VPC Endpoint Connections that were requested from an AWS
	// account that matches AWS accounts defined in account.aws.managed.openshift.io custom resources
	AwsAccountOperatorAccount *AAOAccountAcceptanceCriteria `json:"awsAccountOperatorAccount,omitempty"`

	// AlwaysAccept will instruct the controller to accept any VPC Endpoint Connections
	AlwaysAccept bool `json:"alwaysAccept,omitempty"`
}

type AAOAccountAcceptanceCriteria struct {
	Namespace string `json:"namespace"`
}

// VpcEndpointAcceptanceSpec defines the desired state of VpcEndpointAcceptance
type VpcEndpointAcceptanceSpec struct {
	// Id is the AWS ID of the VPC Endpoint Service for this controller to poll
	Id string `json:"id"`

	// AssumeRoleArn is the ARN of an AWS IAM role in the same account as the specified VPC Endpoint Service.
	// This is necessary if the IAM entity available to the controller is not in the same AWS account as the
	// VPC Endpoint Service.
	AssumeRoleArn string `json:"assumeRoleArn,omitempty"`

	// Region is the AWS region that contains the specified VPC Endpoint Service
	Region string `json:"region"`

	// AcceptanceCriteria
	AcceptanceCriteria AcceptanceCriteria `json:"acceptanceCriteria"`
}

// VpcEndpointAcceptanceStatus defines the observed state of VpcEndpointAcceptance
type VpcEndpointAcceptanceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:shortName={vpceacceptance},scope="Namespaced"

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
