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
	corev1 "k8s.io/api/core/v1"
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
	// +kubebuilder:validation:Optional

	// AutoDiscoverSubnets will instruct the controller to use the subnets associated with this ROSA cluster if true.
	AutoDiscoverSubnets bool `json:"autoDiscoverSubnets,omitempty"`

	// SubnetIds is a list of subnet ids to associate with the VPC Endpoint, which must all be in the same VPC.
	// If more than one is specified, each subnet must be in a different Availability Zone.
	// Ref: https://docs.aws.amazon.com/vpc/latest/privatelink/create-interface-endpoint.html
	SubnetIds []string `json:"subnetIds,omitempty"`

	// +kubebuilder:validation:Optional

	// Ids is a list of VPC ids that aws-vpce-operator can choose from to load balance in a "least used"
	// fashion to evenly spread quota usage across provided VPCs. All provided VPCs must be in the
	// same region as the specified VPC Endpoint Service (.spec.serviceName) and must use subnet auto-discovery
	// (.spec.vpc.autoDiscoverSubnets true) based on the "kubernetes.io/role/internal-elb" tag key
	Ids []string `json:"ids,omitempty"`
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
	Hostname string `json:"hostname"`

	// +kubebuilder:validation:Optional

	ExternalNameService ExternalNameService `json:"externalNameService,omitempty"`
}

// Route53PrivateHostedZone is the configuration of an AWS Route 53 Private Hosted Zone to create a custom domain
// the resolves to the regional endpoint of the created VPCE.
type Route53PrivateHostedZone struct {
	// +kubebuilder:validation:Optional

	// AutoDiscover will use the existing ROSA cluster's Route 53 Private Hosted Zone
	AutoDiscover bool `json:"autoDiscoverPrivateHostedZone,omitempty"`

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
	// ServiceName is the name of the VPC Endpoint Service to connect to
	ServiceName string `json:"serviceName"`

	// SecurityGroup contains the configuration of the security group attached to the VPC Endpoint
	SecurityGroup SecurityGroup `json:"securityGroup"`

	// +kubebuilder:validation:Optional

	// AssumeRoleArn will allow AVO to use sts:AssumeRole to create VPC Endpoints in separate AWS Accounts
	// TODO: Implement
	AssumeRoleArn string `json:"assumeRoleArn,omitempty"`

	// +kubebuilder:validation:Optional

	// AWSCredentialOverride is a Kubernetes secret containing AWS credentials for the operator to use for reconciling
	// this specific vpcendpoint Custom Resource
	AWSCredentialOverrideRef *corev1.SecretReference `json:"awsCredentialOverrideRef,omitempty"`

	// +kubebuilder:validation:Optional

	// Region will allow AVO to create VPC Endpoints and other AWS infrastructure in a specific region
	// Defaults to the same region as the cluster AVO is running on
	Region string `json:"region,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional

	// EnablePrivateDns will allow AVO to create VPC Endpoints with private DNS names specified by a VPC Endpoint Service
	// https://docs.aws.amazon.com/vpc/latest/privatelink/manage-dns-names.html (defaults to false)
	// TODO: Implement
	EnablePrivateDns bool `json:"enablePrivateDns,omitempty"`

	// +kubebuilder:validation:Optional

	// Vpc will allow AVO to use a specific VPC or use the same VPC as the ROSA cluster it's running on
	Vpc Vpc `json:"vpc,omitempty"`

	// +kubebuilder:validation:Optional

	// CustomDns will define configurations for all other custom DNS setups, such as a separate Route 53 Private Hosted
	// Zone or an `ExternalName` Kubernetes service.
	CustomDns CustomDns `json:"customDns,omitempty"`
}

const (
	AWSVpcEndpointCondition      = "AWSVpcEndpointReady"
	AWSSecurityGroupCondition    = "AWSSecurityGroupReady"
	ExternalNameServiceCondition = "ExternalNameServiceReady"
	AWSRoute53RecordCondition    = "AWSRoute53RecordReady"
)

// VpcEndpointStatus defines the observed state of VpcEndpoint
type VpcEndpointStatus struct {
	// Status of the VPC Endpoint
	Status string `json:"status,omitempty"`

	// The AWS ID of the managed security group
	// +kubebuilder:validation:Optional
	SecurityGroupId string `json:"securityGroupId,omitempty"`

	// The AWS ID of the managed VPC Endpoint
	// +kubebuilder:validation:Optional
	VPCEndpointId string `json:"vpcEndpointId,omitempty"`

	// The AWS ID of the Route 53 Private Hosted Zone being used
	// +kubebuilder:validation:Optional
	HostedZoneId string `json:"hostedZoneId,omitempty"`

	// The FQDN of a Route 53 Hosted Zone record that has been created
	// +kubebuilder:validation:Optional
	ResourceRecordSet string `json:"resourceRecordSet,omitempty"`

	// The status conditions of the AWS and K8s resources managed by this controller
	// +kubebuilder:validation:Optional
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
