# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

aws-vpce-operator (AVO) is a Kubernetes operator for OpenShift clusters that manages AWS VPC Endpoint connectivity. It creates and manages AWS VPC Endpoints, security groups, Route53 private hosted zones, and Kubernetes ExternalName services to enable private network connectivity to AWS VPC Endpoint Services.

## Development Commands

### Building and Testing
- `make run` - Run the operator locally (requires AWS credentials)
- `make run ARGS="--zap-log-level=debug"` - Run locally with debug logging
- `make install` - Install CRDs into the cluster via `oc apply -f ./deploy/crds/`
- `make uninstall` - Remove CRDs from the cluster
- `./boilerplate/_lib/container-make generate` - Generate CRDs (only needed if modifying `./api/`)
- `make boilerplate-update` - Update boilerplate files
- `make osde2e` - Build e2e tests

### Container Operations
The project uses the boilerplate system which provides:
- `make build` - Build container image
- `make push` - Push container image
- `make docker-build` - Alternative build command

### Deployment
- `oc apply -f deploy/` - Deploy all operator resources to cluster
- `oc create namespace openshift-aws-vpce-operator` - Create operator namespace

## Architecture Overview

### Core Components

**Main Controllers** (main.go:75):
- **VpcEndpointReconciler** - Primary controller for VpcEndpoint CRs (enabled by default)
- **VpcEndpointAcceptanceReconciler** - Handles VPC endpoint service acceptance (disabled by default)
- **VpcEndpointTemplateReconciler** - Template-based endpoint creation (disabled by default)

**API Versions**:
- `api/v1alpha1/` - Contains AvoConfig and VpcEndpointAcceptance types
- `api/v1alpha2/` - Contains VpcEndpoint type (primary CR)

**AWS Integration** (pkg/aws_client/):
- EC2 client for VPC endpoints and security groups
- Route53 client for DNS management
- STS client for role assumption
- Supports both direct AWS credentials and STS token assume role patterns

**Helper Packages**:
- `pkg/util/` - Common utilities including AWS resource tagging helpers
- `pkg/infrastructures/` - OpenShift Infrastructure CR utilities
- `pkg/dnses/` - DNS configuration helpers
- `controllers/util/` - Controller-specific utilities

### Custom Resources

**VpcEndpoint** (v1alpha2) - Primary resource:
- Creates AWS VPC Endpoint for specified service name
- Manages security group with ingress/egress rules
- Optionally creates Route53 private hosted zone and DNS records
- Optionally creates Kubernetes ExternalName service

**VpcEndpointAcceptance** (v1alpha1):
- Automates acceptance of VPC endpoint connections
- Supports cross-account role assumption for service provider accounts

### Development Environment Requirements

**For AWS STS ROSA clusters**:
- Must have `infrastructures.config.openshift.io/default` CR
- Must have `dnses.config.openshift.io/default` CR
- Specific AWS IAM permissions for EC2, Route53 operations
- Proper AWS resource tagging for Hive cleanup integration

**Local Development**:
- AWS credentials via environment variables or role assumption
- Access to OpenShift/Kubernetes cluster with appropriate CRDs installed
- Go 1.24+ (from go.mod)

## Important Notes

- All AWS resources created by the operator must include proper tags for Hive cleanup (use helpers in pkg/util/naming.go)
- The operator is designed specifically for AWS OpenShift clusters and leverages OpenShift-specific CRs
- FIPS mode is enabled in the build configuration
- Uses controller-runtime framework with event recording and health checks
- Boilerplate system provides standardized build, test, and deployment workflows