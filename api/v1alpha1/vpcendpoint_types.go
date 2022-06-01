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

// SecurityGroupRule is based on required inputs for
// aws authorize-security-group-ingress/egress
type SecurityGroupRule struct {
	// FromPort and ToPort are the start and end of the port range to allow.
	// To allow a single port, set both to the same value.
	FromPort int64 `json:"fromPort,omitempty"`

	// FromPort and ToPort are the start and end of the port range to allow
	// To allow a single port, set both to the same value.
	ToPort int64 `json:"toPort,omitempty"`

	// Protocol is the IP protocol, tcp | udp | icmp | all
	Protocol string `json:"protocol,omitempty"`
}

type SecurityGroup struct {
	// IngressRules is a list of security group ingress rules.
	// They will be allowed for the master and worker security groups.
	// +optional
	IngressRules []SecurityGroupRule `json:"ingressRules,omitempty"`

	// EgressRules is a list of security group egress rules
	// They will be allowed for the master and worker security groups.
	// +optional
	EgressRules []SecurityGroupRule `json:"egressRules,omitempty"`
}

// VpcEndpointSpec defines the desired state of VpcEndpoint
type VpcEndpointSpec struct {
	//+kubebuilder:validation:MinLength=0

	// ServiceName is the name of the VPC Endpoint Service to connect to
	ServiceName string `json:"serviceName"`

	// SecurityGroup contains the configuration of the security group attached to the VPC Endpoint
	SecurityGroup SecurityGroup `json:"securityGroup"`
}

// VpcEndpointStatus defines the observed state of VpcEndpoint
type VpcEndpointStatus struct {
	// Status of the VPC Endpoint
	Status string `json:"status,omitempty"`

	// The AWS ID of the managed security group
	// +optional
	SecurityGroupId string `json:"securityGroupId,omitempty"`

	// The AWS ID of the managed VPC Endpoint
	// +optional
	VPCEndpointId string `json:"vpcEndpointId,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VpcEndpoint is the Schema for the vpcendpoints API
type VpcEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VpcEndpointSpec   `json:"spec,omitempty"`
	Status VpcEndpointStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VpcEndpointList contains a list of VpcEndpoint
type VpcEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VpcEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VpcEndpoint{}, &VpcEndpointList{})
}
