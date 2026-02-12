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
	// +kubebuilder:validation:Format=cidr

	// CidrIp is the IPv4 address range, in CIDR format, to allow.
	// If not specified, the cluster's master and worker security group are allowed instead.
	CidrIp string `json:"cidrIp,omitempty"`

	// FromPort and ToPort are the start and end of the port range to allow.
	// In the case of a single port, set both to the same value.
	FromPort int32 `json:"fromPort,omitempty"`

	// FromPort and ToPort are the start and end of the port range to allow.
	// In the case of a single port, set both to the same value.
	ToPort int32 `json:"toPort,omitempty"`

	// Protocol is the IP protocol, tcp | udp | icmp | all
	Protocol string `json:"protocol,omitempty"`
}

// SecurityGroup represents the configuration of a security group associated with the VPC Endpoint created by this CR
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

// Tag represents a key-value pair to filter AWS resources by
type Tag struct {
	// Key of an AWS tag
	Key string `json:"key"`

	// Value of an AWS tag
	Value string `json:"value"`
}

// Vpc represents the configuration for the AWS VPC to create the VPC Endpoint in
type Vpc struct {
	// +kubebuilder:validation:Optional

	// AutoDiscoverSubnets will instruct the controller to use the subnets associated with this ROSA cluster if true
	// using the tag-key: "kubernetes.io/cluster/${infraName}". If .spec.vpc.ids or spec.vpc.tags is specified, the
	// tag-key "kubernetes.io/role/internal-elb" will be used instead.
	AutoDiscoverSubnets bool `json:"autoDiscoverSubnets,omitempty"`

	// +kubebuilder:validation:Optional

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

	// +kubebuilder:validation:Optional

	// Tags is a list of AWS tag key-value pairs to find VPCs with. This is mutually exclusive with
	// .spec.vpc.ids and can only be specified with .spec.vpc.autoDiscoverSubnets = true.
	Tags []Tag `json:"tags,omitempty"`

	// +kubebuilder:validation:Optional

	// SubnetTags is a list of AWS tag key-value pairs to additionally filter private-subnets with. The main tags used
	// when filtering subnets is controlled by .spec.vpc.autoDiscoverSubnets
	SubnetTags []Tag `json:"subnetTags,omitempty"`
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

// DomainName represents the base domain name of a Route 53 Private Hosted Zone
// Similar to: https://github.com/kubernetes/api/blob/7a87286591e433a1d034a768032b5fd4abb072b3/core/v1/types.go#L2100-L2110
type DomainName struct {
	// Name specifies the base domain name directly
	Name string `json:"name,omitempty"`

	// ValueFrom allows the base domain name to be read from a source
	ValueFrom *DomainNameSource `json:"valueFrom,omitempty"`
}

type DomainNameSource struct {
	// A reference to a config.openshift.io/v1 DNS custom resource
	DnsRef *DnsSelector `json:"dnsRef,omitempty"`

	// A reference to a hypershift.openshift.io/v1beta1 HostedControlPlane custom resource
	HostedControlPlaneRef *HostedControlPlaneSelector `json:"hostedControlPlaneRef,omitempty"`
}

// DnsSelector represents a selector for a config.openshift.io/v1 DNS custom resource
type DnsSelector struct {
	// Name of the config.openshift.io/v1 DNS custom resource to select
	Name string `json:"name"`
}

// HostedControlPlaneSelector represents a selector for a hypershift.openshift.io/v1beta1 HostedControlPlane
// custom resource
type HostedControlPlaneSelector struct {
	// Path of the field containing the namespace of the hostedcontrolplane, typically ".metadata.namespace" to select
	// the same namespace as the VpcEndpoint itself
	NamespaceFieldRef *ObjectFieldSelector `json:"namespaceFieldRef,omitempty"`
}

// ObjectFieldSelector selects a field of a VpcEndpoint.
// https://github.com/kubernetes/api/blob/f3a0f2ed177a2ba0eb0b6318ee16222b14872d70/core/v1/types.go#L2054
type ObjectFieldSelector struct {
	// +kubebuilder:validation:Enum=`.metadata.namespace`

	// Path of the field to select
	FieldPath string `json:"fieldPath"`
}

// AssociatedVpc represents configuration for associating the created Route53 Private Hosted Zone to an additional VPC.
// Ref: https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/hosted-zone-private-associate-vpcs-different-accounts.html
type AssociatedVpc struct {
	// CredentialsSecretRef references a Kubernetes secret with the keys: "aws_access_key_id" and
	// "aws_secret_access_key" which has the permissions to perform route53:AssociateVpcWithHostedZone and
	// ec2:DescribeVpcs
	CredentialsSecretRef *corev1.SecretReference `json:"credentialsSecretRef"`
	// VpcId is the ID of the VPC to associate to the Route 53 Private Hosted Zone
	VpcId string `json:"vpcId"`
	// Region is the AWS Region the VPC exists in
	Region string `json:"region"`
}

// Route53PrivateHostedZone is the configuration of an AWS Route 53 Private Hosted Zone to create a custom domain
// the resolves to the regional endpoint of the created VPCE.
type Route53PrivateHostedZone struct {
	// +kubebuilder:validation:Optional

	// AutoDiscover will use the existing ROSA cluster's Route 53 Private Hosted Zone
	AutoDiscover bool `json:"autoDiscoverPrivateHostedZone,omitempty"`

	// AssociatedVpc represents configuration for associating the created Route53 Private Hosted Zone to additional VPCs
	// Ref: https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/hosted-zone-private-associate-vpcs-different-accounts.html
	AssociatedVpcs []AssociatedVpc `json:"associatedVpcs,omitempty"`

	// DomainName specifies the domain name of a Route 53 Private Hosted Zone to create
	DomainName string `json:"domainName,omitempty"`

	// DomainNameRef is an alternative to DomainName when the domain name of a Route 53 Private Hosted Zone is read from
	// another source
	DomainNameRef *DomainName `json:"domainNameRef,omitempty"`

	// Id specifies the AWS ID of an existing Route 53 Private Hosted Zone to use
	Id string `json:"id,omitempty"`

	// +kubebuilder:validation:XValidation:message=cannot create an ExternalName service without a Route53 Hosted Zone record,rule=!(self.hostname == "" && self.externalNameService.name != "")

	// Record is the configuration of a record within the selected Route 53 Private Hosted Zone
	Record Route53HostedZoneRecord `json:"record,omitempty"`
}

// CustomDns is the configuration of customized DNS routing external to a standalone AWS VPC Endpoint
type CustomDns struct {
	// +kubebuilder:validation:XValidation:message=cannot set both a Route53 Hosted Zone ID and domain name,rule=!(has(self.id) && (has(self.domainName) || has(self.domainNameRef)))

	// Route53PrivateHostedZone configures an AWS Route 53 Private Hosted Zone with a route to the created VPCE.
	Route53PrivateHostedZone Route53PrivateHostedZone `json:"route53PrivateHostedZone,omitempty"`
}

type ServiceName struct {
	Name      string             `json:"name,omitempty"`
	ValueFrom *ServiceNameSource `json:"valueFrom,omitempty"`
}

// ServiceNameSource represents the source of a VPC Endpoint Service Name
// Similar to: https://github.com/kubernetes/api/blob/7a87286591e433a1d034a768032b5fd4abb072b3/core/v1/types.go#L2100-L2110
type ServiceNameSource struct {
	AwsEndpointServiceRef *AwsEndpointSelector `json:"awsEndpointServiceRef,omitempty"`
}

type AwsEndpointSelector struct {
	Name string `json:"name"`
}

// +kubebuilder:validation:XValidation:message=.spec.vpc.autoDiscoverSubnets is not supported with .spec.region,rule=!(has(self.region) && self.vpc.autoDiscoverSubnets)
// +kubebuilder:validation:XValidation:message=.spec.customDns.route53PrivateHostedZone.autoDiscoverPrivateHostedZone is not supported with .spec.region,rule=!(has(self.region) && self.customDns.route53PrivateHostedZone.autoDiscoverPrivateHostedZone)
// +kubebuilder:validation:XValidation:message="one of .spec.serviceName, .spec.serviceNameRef.name, or .spec.serviceNameRef.valueFrom.awsEndpointServiceRef.name must be specified",rule=has(self.serviceName) || (has(self.serviceNameRef) && has(self.serviceNameRef.name)) || (!has(self.serviceNameRef.valueFrom) || !has(self.serviceNameRef.valueFrom.awsEndpointServiceRef) || has(self.serviceNameRef.valueFrom.awsEndpointServiceRef.name))

// VpcEndpointSpec defines the desired state of VpcEndpoint
type VpcEndpointSpec struct {
	// ServiceName is the name of the VPC Endpoint Service to connect to
	ServiceName string `json:"serviceName,omitempty"`

	// ServiceNameRef refers to a group and resource that contains the name of the VPC Endpoint Service
	ServiceNameRef *ServiceName `json:"serviceNameRef,omitempty"`

	// SecurityGroup contains the configuration of the security group attached to the VPC Endpoint
	SecurityGroup SecurityGroup `json:"securityGroup"`

	// +kubebuilder:validation:Optional

	// AssumeRoleArn will allow AVO to use sts:AssumeRole to create VPC Endpoints in separate AWS Accounts
	// TODO: Implement
	AssumeRoleArn string `json:"assumeRoleArn,omitempty"`

	// +kubebuilder:validation:Optional

	// AWSCredentialOverride is a Kubernetes secret containing AWS credentials for the operator to use for reconciling
	// this specific vpcendpoint Custom Resource.
	// The secret should have data keys for either:
	// * role_arn: The operator will attempt to assume this role
	// * aws_access_key_id and aws_secret_access_key: The operator will simply use these IAM User credentials
	AWSCredentialOverrideRef *corev1.SecretReference `json:"awsCredentialOverrideRef,omitempty"`

	// +kubebuilder:validation:Optional

	// Region will allow AVO to create VPC Endpoints and other AWS infrastructure in a specific region
	// Defaults to the same region as the cluster AVO is running on
	Region string `json:"region,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional

	// EnablePrivateDns will allow AVO to create VPC Endpoints with private DNS names specified by a VPC Endpoint Service.
	// When true, DNS resolution is handled at the VPC Endpoint Service level via Domain Ownership Verification,
	// and AVO will skip Route53 hosted zone and record management for this endpoint.
	// https://docs.aws.amazon.com/vpc/latest/privatelink/manage-dns-names.html (defaults to false)
	EnablePrivateDns bool `json:"enablePrivateDns,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:message=.spec.vpc.autoDiscoverSubnets must be true when specifying tags to search for VPCs,rule=!(size(self.tags) > 0 && !self.autoDiscoverSubnets)
	// +kubebuilder:validation:XValidation:message=.spec.vpc.autoDiscoverSubnets must be true when specifying VPCs to load balance,rule=!(size(self.ids) > 0 && !self.autoDiscoverSubnets)
	// +kubebuilder:validation:XValidation:message=.spec.vpc.subnetIds is not supported when specifying VPCs to load balance,rule=!(size(self.ids) > 0 && has(self.subnetIds) && size(self.subnetIds) > 0)

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

	// The AWS ID of the VPC to create resources in
	// +kubebuilder:validation:Optional
	VPCId string `json:"vpcId,omitempty"`

	// The AWS ID of the managed VPC Endpoint
	// +kubebuilder:validation:Optional
	VPCEndpointId string `json:"vpcEndpointId,omitempty"`

	// The name of the VPC Endpoint Service the VPC Endpoint connects to
	// +kubebuilder:validation:Optional
	VPCEndpointServiceName string `json:"vpcEndpointServiceName,omitempty"`

	// The AWS ID of the Route 53 Private Hosted Zone being used
	// +kubebuilder:validation:Optional
	HostedZoneId string `json:"hostedZoneId,omitempty"`

	// The FQDN of a Route 53 Hosted Zone record that has been created
	// +kubebuilder:validation:Optional
	ResourceRecordSet string `json:"resourceRecordSet,omitempty"`

	// The Infra Id of the cluster, used for naming and tagging purposes
	// +kubebuilder:validation:Optional
	InfraId string `json:"infraId,omitempty"`

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
