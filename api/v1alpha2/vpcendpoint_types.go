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

// SecurityGroupRule is based on required inputs for `aws authorize-security-group-ingress/egress`
type SecurityGroupRule struct {
	// FromPort and ToPort are the start and end of the port range to allow.
	// In the case of a single port, set both to the same value.
	FromPort int32 `json:"fromPort,omitempty"`

	// FromPort and ToPort are the start and end of the port range to allow.
	// In the case of a single port, set both to the same value.
	ToPort int32 `json:"toPort,omitempty"`

	// Protocol is the IP protocol, tcp | udp | icmp | all
	Protocol string `json:"protocol,omitempty"`
}

// SecurityGroup is represents the configuration of a security group associated with the VPC Endpoint created by this CR
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

// Vpc represents the configuration for the AWS VPC to create the VPC Endpoint in
type Vpc struct {
	// +kubebuilder:default=true

	// AutoDiscoverSubnets will instruct the controller to use the subnets associated with this ROSA cluster if true.
	AutoDiscoverSubnets bool `json:"autoDiscoverSubnets"`

	// SubnetIds is a list of subnet ids to associate with the VPC Endpoint. Each subnet must be in a different
	// Availability Zone.
	SubnetIds []string `json:"subnetIds,omitempty"`
}

// ExternalNameService is the configuration of a Kubernetes ExternalName Service pointing to a CustomDns
// Route53PrivateHostedZone Record for the VPC Endpoint.
type ExternalNameService struct {
	// Name of the ExternalName service to create in the same namespace as the VPCE Custom Resource
	Name string `json:"name"`
}

// Route53HostedZoneRecord is the configuration of an AWS Route 53 Hosted Zone Record pointing to the created VPCE.
type Route53HostedZoneRecord struct {
	// Hostname is the hostname of the record.
	Hostname            string              `json:"hostname"`
	ExternalNameService ExternalNameService `json:"externalNameService,omitempty"`
}

// Route53PrivateHostedZone is the configuration of an AWS Route 53 Private Hosted Zone to create a custom domain
// the resolves to the regional endpoint of the created VPCE.
type Route53PrivateHostedZone struct {
	// +kubebuilder:default=true

	// AutoDiscover will use the existing ROSA cluster's Route 53 Private Hosted Zone
	AutoDiscover bool `json:"autoDiscoverPrivateHostedZone"`

	// DomainName specifies the domain name of a Route 53 Private Hosted Zone to create
	DomainName string `json:"domainName,omitempty"`

	// Id specifies the AWS ID of an existing Route 53 Private Hosted Zone to use
	Id string `json:"id,omitempty"`

	// Record is the configuration of a record within the selected Route 53 Private Hosted Zone
	Record Route53HostedZoneRecord `json:"record,omitempty"`
}

// CustomDns is the configuration of customized DNS routing external to a standalone AWS VPC Endpoint
type CustomDns struct {
	// Route53PrivateHostedZone configures an AWS Route 53 Private Hosted Zone with a route to the created VPCE.
	Route53PrivateHostedZone Route53PrivateHostedZone `json:"route53PrivateHostedZone,omitempty"`
}

// VpcEndpointSpec defines the desired state of VpcEndpoint
type VpcEndpointSpec struct {
	// AssumeRoleArn will allow AVO to use sts:AssumeRole to create VPC Endpoints in separate AWS Accounts
	AssumeRoleArn string `json:"assumeRoleArn,omitempty"`

	// Region will allow AVO to create VPC Endpoints and other AWS infrastructure in a specific region
	// Defaults to the same region as the cluster AVO is running on
	Region string `json:"region,omitempty"`

	// +kubebuilder:default=false
	// EnablePrivateDns will allow AVO to create VPC Endpoints with private DNS names specified by a VPC Endpoint Service
	// https://docs.aws.amazon.com/vpc/latest/privatelink/manage-dns-names.html (defaults to false)
	EnablePrivateDns bool `json:"enablePrivateDns"`

	// ServiceName is the name of the VPC Endpoint Service to connect to
	ServiceName string `json:"serviceName"`

	// SecurityGroup contains the configuration of the security group attached to the VPC Endpoint
	SecurityGroup SecurityGroup `json:"securityGroup"`

	// Vpc will allow AVO to use a specific VPC or use the same VPC as the ROSA cluster it's running on
	Vpc Vpc `json:"vpc"`

	// CustomDns will define configurations for all other custom DNS setups, such as a separate Route 53 Private Hosted
	// Zone or an `ExternalName` Kubernetes service.
	CustomDns CustomDns `json:"customDns,omitempty"`
}

const (
	AWSVpcEndpointCondition   = "AWSVpcEndpointReady"
	AWSSecurityGroupCondition = "AWSSecurityGroupReady"
	AWSCustomDnsCondition     = "AWSCustomDnsReady"
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

// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName={vpce},scope="Namespaced"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="ID",type=string,JSONPath=`.status.vpcEndpointId`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:storageversion

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
