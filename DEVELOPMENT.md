# Development Guide

Quick reference for developing the AWS VPCE Operator.

## Prerequisites

- **Go**: 1.24 or later (see `go.mod`)
- **operator-sdk**: v1.21.0
- **kubectl**: For cluster interaction
- **prek**: `uv tool install prek` (recommended) or `pipx install prek`

## Initial Setup

```bash
# Clone repository
git clone https://github.com/openshift/aws-vpce-operator.git
cd aws-vpce-operator

# Install prek hooks
prek install
```

## Common Commands

### Build
```bash
make go-build                 # Build operator binary
make docker-build             # Build container image
```

### Test
```bash
make go-test                  # Run all unit tests
go test ./controllers/vpcendpoint/...  # Test specific package
ginkgo -r ./controllers/      # Run controller tests with Ginkgo
```

### Lint
```bash
make go-check                 # Full linting (golangci-lint)
prek run --all-files          # Run all prek hooks
prek run golangci-lint        # Lint only
```

### Code Generation
```bash
# After modifying API types (api/v1alpha1/*.go)
# or interfaces requiring mocks
boilerplate/_lib/container-make generate

# What this generates:
# - Deepcopy methods (zz_generated.deepcopy.go)
# - OpenAPI schemas
# - Mock interfaces for testing
```

### Run Locally
```bash
# Run against cluster in ~/.kube/config
make run

# Run with verbose logging
make run ARGS="--zap-log-level=debug"
```

### Container-based Build
```bash
# Run make targets inside boilerplate container
# (ensures consistent environment with CI)
boilerplate/_lib/container-make
boilerplate/_lib/container-make go-test
boilerplate/_lib/container-make generate
```

## Fast Local Iteration

**Minimal validation loop:**
```bash
# After code changes
go build ./...                # Fast compile check (~5s)
go test ./pkg/aws_client/     # Run affected tests
prek run                      # Lint staged files
```

**Full validation (pre-PR):**
```bash
prek run --all-files          # All hooks (~15-30s)
make go-test                  # Full test suite
```

## Targeted Testing

```bash
# Run specific test
ginkgo -focus="NetworkPolicy" ./controllers/vpcendpoint/

# Run tests for one package
go test -v ./controllers/vpcendpoint/

# Skip slow tests during development
ginkgo -skip="E2E" -r ./...
```

## Debugging

```bash
# Verbose operator logs
make run ARGS="--zap-log-level=debug"

# Print specific package logs
go test -v ./pkg/... 2>&1 | grep "MyFunction"

# Ginkgo verbose output
ginkgo -v ./...
```

## Dependency Management

```bash
# Add new dependency
go get github.com/some/package@v1.2.3

# Update dependency
go get -u github.com/some/package

# Tidy (removes unused, adds missing)
go mod tidy

# Verify checksums
go mod verify
```

**Note**: `go.sum` changes automatically trigger validation in prek.

## Architecture Pointers

- **API Types**: `api/v1alpha1/` and `api/v1alpha2/` - CRD definitions
- **Controllers**: `controllers/{vpcendpoint,vpcendpointacceptance,vpcendpointtemplate}/` - Reconciliation logic
- **AWS Client**: `pkg/aws_client/` - EC2, Route53, STS operations
- **Tests**: `*_test.go` alongside source, `*_suite_test.go` for Ginkgo
- **Mocks**: `pkg/aws_client/mock.go`, `pkg/testutil/mock.go`
- **Design Docs**: `designs/` - Architecture and feature designs

## CI Parity

Local prek hooks mirror Tekton CI checks:
- **go-check** <-> Tekton lint job
- **go-build** <-> Compilation in CI
- **go-test** <-> Unit test job
- **gitleaks** <-> Security scanning

Run `prek run --all-files` before pushing to catch CI failures early.

## Boilerplate Integration

This repo uses Red Hat's standardized boilerplate:
- Centralized Makefiles: `boilerplate/openshift/golang-osd-operator/`
- Standard targets: `go-build`, `go-check`, `go-test`
- Container builds: `boilerplate/_lib/container-make`
- Update boilerplate: `make boilerplate-update`

## Troubleshooting

**Mock generation fails:**
```bash
boilerplate/_lib/container-make generate
```

**Prek hook timeout:**
```bash
# macOS: Install GNU timeout
brew install coreutils

# Linux: timeout is built-in
```

**go.sum checksum mismatch:**
```bash
export GOPROXY="https://proxy.golang.org"
go mod tidy
```

**Tests fail locally but pass in CI:**
```bash
boilerplate/_lib/container-make go-test
```

## Further Reading

- [Testing Guide](./TESTING.md)
- [Design Documentation](./designs/)
- [Operator SDK Docs](https://sdk.operatorframework.io/)
