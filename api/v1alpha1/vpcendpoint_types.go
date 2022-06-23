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
	corev1 "k8s.io/api/core/v1"
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

	// SubdomainName is the name of the Route53 Hosted Zone CNAME rule to create in the cluster's
	// Private Route53 Hosted Zone
	SubdomainName string `json:"subdomainName"`

	// ExternalServiceName is the name of the Kubernetes Service supporting the VPC Endpoint Service
	ExternalNameService ExternalNameService `json:"externalNameService,omitempty"`
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

	// Whether the Route53 CNAME record has been created
	CNAMERecordCreated bool `json:"hostedZoneRecordCreated,omitempty"`

	// ExternalServiceNameStatus is the status of the ExternalName service
	ExternalServiceNameStatus metav1.Status `json:"externalNameServiceStatus,omitempty"`
}

type ExternalNameService struct {
	//Name  is the name of the externalName service
	Name string `json:"name"`

	// Namespace is the namespace of the externalName service
	Namespace string `json:"namespace"`

	// type is the type of the Kubernetes Service supporting the VPC Endpoint Service
	ServiceType corev1.ServiceType `json:"type"`

	// ExternalName is the DNS record of the Kubernetes Service supporting the VPC Endpoint Service
	ExternalName string `json:"externalName,omitempty"`
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
