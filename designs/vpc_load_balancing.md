# Load Balance VPC Endpoints Across a Set of VPCs

Author: @mjlshen

Last Updated: 02/02/2023

## Summary

`aws-vpce-operator` (AVO)'s `VpcEndpoint` CustomResource (CR) sometimes needs to be used at scale to create VPC
Endpoints within a set of VPCs.

### Current State

The current state of the VpcEndpoint API requires that consumers either use the cluster's own subnets/VPC or specify a list of subnets that must
be part of the same VPC. Since AWS has a [quota on the maximum number of VPC Endpoints per VPC](https://docs.aws.amazon.com/vpc/latest/userguide/amazon-vpc-limits.html#vpc-limits-endpoints),
which starts at 50 and is generally easy to raise to 200, when AVO is heavily used at scale it can run into this limit.

```go
// Vpc represents the configuration for the AWS VPC to create the VPC Endpoint in
type Vpc struct {
    // +kubebuilder:validation:Optional

    // AutoDiscoverSubnets will instruct the controller to use the subnets associated with this ROSA cluster if true.
    AutoDiscoverSubnets bool `json:"autoDiscoverSubnets,omitempty"`

    // SubnetIds is a list of subnet ids to associate with the VPC Endpoint, which must all be in the same VPC.
    // If more than one is specified, each subnet must be in a different Availability Zone.
    // Ref: https://docs.aws.amazon.com/vpc/latest/privatelink/create-interface-endpoint.html
    SubnetIds []string `json:"subnetIds,omitempty"`
}
```

## Motivation

This change is important because:

- It will prevent the need of a higher level orchestrator to stamp out `VpcEndpoint` CRs load balanced across a set of VPCs

## Relevant Stories

- [OSD-13906](https://issues.redhat.com/browse/OSD-13906) Creating VPC Endpoints in the OSD Transit Account
- [OSD-15012](https://issues.redhat.com/browse/OSD-15012) Load Balancing VPC Endpoints across VPCs

## Goals

- Add an optional set of VPC IDs specified in `.spec.vpc.ids`
- Pick out private subnet IDs from the provided list of VPCs with the aid of `kubernetes.io/role/internal-elb`
  - Filter out relevant subnets that are in Availability Zones that the VPC Endpoint Service supports
- Load balance VPC Endpoints with a "least connection" strategy into relevant VPCs to take advantage of existing monitoring and alerting

## Non-Goals/Future Work

- Have AVO dynamically create and/or manage VPCs directly
- Monitor and/or alert on the quota of VPC Endpoints

Existing alerting is defined as: `the number of VPC Endpoints in a region`/(`number of VPCs per region` * `VPC Endpoint quota per region`) exceeds 0.9.

```
sum(aws_resources_exporter_vpc_interfacevpcendpointspervpc_usage{job="{{{ job_stage }}}"}) by (aws_region, quota_code, service_code) /
sum(count(aws_resources_exporter_vpc_interfacevpcendpointspervpc_usage{job="{{{ job_stage }}}"}) by (aws_region, quota_code, service_code) *
on (aws_region, quota_code, service_code) aws_resources_exporter_vpc_interfacevpcendpointspervpc_quota{job="{{{ job_stage }}}"}) by (aws_region, quota_code, service_code) >= 0.9
```

## Proposal

A new `ids` field in `.spec.vpc`:

```go
// Vpc represents the configuration for the AWS VPC to create the VPC Endpoint in
type Vpc struct {
    // +kubebuilder:validation:Optional

    // AutoDiscoverSubnets will instruct the controller to use the subnets associated with this ROSA cluster if true.
    AutoDiscoverSubnets bool `json:"autoDiscoverSubnets,omitempty"`

    // SubnetIds is a list of subnet ids to associate with the VPC Endpoint, which must all be in the same VPC.
    // If more than one is specified, each subnet must be in a different Availability Zone.
    // Ref: https://docs.aws.amazon.com/vpc/latest/privatelink/create-interface-endpoint.html
    SubnetIds []string `json:"subnetIds,omitempty"`
	
    // Ids is a list of VPC ids that aws-vpce-operator can choose from to load balance in a "least used" 
    // fashion to evenly spread quota usage across provided VPCs. All provided VPCs must be in the
    // same region as the specified VPC Endpoint Service (.spec.serviceName).
    Ids []string `json:"ids,omitempty"`
}
```

## Risks and Mitigations

- None known currently

## Alternatives

- A different piece of automation load balancing VPC Endpoint CRs and AVO just creating VPC Endpoints as specified.
