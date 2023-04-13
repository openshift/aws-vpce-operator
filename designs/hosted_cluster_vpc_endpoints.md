# Templating VPC Endpoints for Private Hosted Control Planes

Author: @mjlshen @aliceh

Last Updated: 05/11/2023

## Summary

`aws-vpce-operator` (AVO) needs to be able to manage the lifecycle of one VpcEndpoint CR per private HyperShift cluster to enable SRE backplane access.
Since each private HyperShift cluster's hosted control plane has a unique AWS VPC Endpoint Service, and no existing component can create a VpcEndpoint per
HyperShift cluster, a templating functionality is needed.

### Current State

The existing VpcEndpoint CRD and controller can reconcile the existence of a single AWS VPC Endpoint, however there is no existing templating functionality
for many very similar VpcEndpoint CRDs.

## Relevant Stories

- [SDE-1592](https://issues.redhat.com/browse/SDE-1592) Multi-tenant control plane account hierarchy
- [OSD-15666](https://issues.redhat.com/browse/OSD-15666) Create AVO controller to manage backplane VPC Endpoints

## Goals

- Create a new `aws-vpce-operator` controller and CustomResourceDefinition named `VpcEndpointTemplate` to manage the lifecycle of one VpcEndpoint CR per private HyperShift cluster to enable SRE backplane access.

## Non-Goals/Future Work

- Build intelligence into the template - the heavy lifting should be done by the `VpcEndpoint` CR.

## Proposal

- Upon receiving an event for a `hostedcontrolplane.hypershift.openshift.io/v1beta1` change, reconcile an AVO `VpcEndpoint` CR

- How do we figure out if a hostedcluster is private?
  - .spec.platform.aws.endpointAccess: Private on the `hostedcluster` CR
- How do we figure out if a hostedcluster has additionalAllowedPrincipals?
  - .spec.platform.aws.additionalAllowedPrincipals on the `hostedcluster` CR
- How do we figure out the VPC Endpoint Service?
  - .status.endpointServiceName on the `awsendpointservice` CR

### VpcEndpointTemplate CR

The `VpcEndpointTemplate` CR acts like a Deployment does for a Pod - the API semantics are similar as shown in this proposed sample:

```yaml
apiVersion: avo.openshift.io/v1alpha2
kind: VpcEndpointTemplate
metadata:
  name: private-hcp
  namespace: openshift-aws-vpce-operator
spec:
  template:
    type: "HostedControlPlane"
    selector:
      matchLabels:
        key: value
    # VpcEndpoint will need to define a VpcEndpointTemplate akin to PodTemplate for Deployments
    # https://github.com/kubernetes/api/blob/f3a0f2ed177a2ba0eb0b6318ee16222b14872d70/core/v1/types.go#L4240
    template:
      metadata:
        labels:
          key: value
      spec:
        serviceNameRef:
          valueFrom:
            awsEndpointServiceRef:
              name: private-router
        securityGroup:
          ingressRules:
            - fromPort: 443
              toPort: 443
              protocol: tcp
        vpc:
          autoDiscoverSubnets: false
          tags:
            - key: "key1"
              value: "value1"
        customDns:
          route53PrivateHostedZone:
            autoDiscoverPrivateHostedZone: false
            record:
              hostname: "api"
            domainNameRef:
              valueFrom:
                hostedControlPlaneRef:
                  namespaceFieldRef:
                    fieldPath: ".metadata.namespace"
```

## Risks and Mitigations

- Importing the HyperShift Operator API currently is troublesome [HOSTEDCP-336](https://issues.redhat.com/browse/HOSTEDCP-336), which will be required to reconcile off of `hostedcontrolplane` and `awsendpointservice` CRs.
- There can be up to a [15 minute delay (tunable)](https://github.com/openshift/aws-vpce-operator/blob/bf273939e868d1500e05d9439ed7d59495c4931b/controllers/vpcendpoint/vpcendpoint_controller.go#L141-L142) between a `hostedcontrolplane` becoming private and the corresponding VPC Endpoint to become ready. To mitigate this, we will template a VpcEndpoint for public `hostedcontrolplanes` as well so it will be always ready.

## Alternatives

- OCM manages AWS VPC Endpoints when private hosted clusters are created or public hosted clusters become private. We determined this was not a path forward in [SDA-8566](https://issues.redhat.com/browse/SDA-8566)
- The HyperShift operator manages AWS VPC Endpoints. We determined this was not a path forward because it is a specific requirement for ROSA as a managed service and not suitable for upstream.
