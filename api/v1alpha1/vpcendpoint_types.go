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
	FromPort int32 `json:"fromPort,omitempty"`

	// FromPort and ToPort are the start and end of the port range to allow
	// To allow a single port, set both to the same value.
	ToPort int32 `json:"toPort,omitempty"`

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
	// +kubebuilder:validation:MinLength=0

	// ServiceName is the name of the VPC Endpoint Service to connect to
	ServiceName string `json:"serviceName"`

	// SecurityGroup contains the configuration of the security group attached to the VPC Endpoint
	SecurityGroup SecurityGroup `json:"securityGroup"`

	// +kubebuilder:validation:Pattern=[a-z0-9]([-a-z0-9]*[a-z0-9])?
	// SubdomainName is the name of the Route53 Hosted Zone CNAME rule to create in the cluster's
	// Private Route53 Hosted Zone
	SubdomainName string `json:"subdomainName"`

	// ExternalNameService configures the name and namespace of the created Kubernetes ExternalName Service
	ExternalNameService ExternalNameServiceSpec `json:"externalNameService"`

	// AddtlHostedZoneName is an optional FQDN to support supplemental VPCE routing via Route53 Private Hosted Zone
	// +optional
	AddtlHostedZoneName string `json:"addtlHostedZoneName,omitempty"`
}

const (
	AWSVpcEndpointCondition      = "AWSVpcEndpointReady"
	AWSSecurityGroupCondition    = "AWSSecurityGroupReady"
	AWSRoute53RecordCondition    = "AWSRoute53RecordReady"
	ExternalNameServiceCondition = "ExternalNameServiceReady"
)

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

	// The status conditions of the AWS and K8s resources managed by this controller
	// +optional
	Conditions []metav1.Condition `json:"conditions"`
}

type ExternalNameServiceSpec struct {
	// Name of the ExternalName service to create in the same namespace as the VPCE CR
	Name string `json:"name"`
}

// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName={vpce},scope="Namespaced"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="ID",type=string,JSONPath=`.status.vpcEndpointId`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

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
