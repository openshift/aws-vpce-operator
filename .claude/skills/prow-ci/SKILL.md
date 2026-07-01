---
name: prow-ci
description: Fetch and analyze OpenShift Prow CI job failures with automated artifact download and failure pattern detection
trigger: prow, prow-ci, /prow-ci, ci results, check ci, analyze ci failure
---

# Prow CI Analysis for AWS VPCE Operator

This skill fetches Prow CI job artifacts from Google Cloud Storage and provides automated failure analysis.

## Prerequisites

Before using this skill, verify gcloud CLI is installed:
```bash
which gcloud
```

If not installed, provide instructions from: https://cloud.google.com/sdk/docs/install

**Note**: The `test-platform-results` GCS bucket is publicly accessible - no authentication required.

## Quick Start

```bash
# Check PR status and get Prow job URLs
gh pr checks <PR_NUMBER>

# Analyze a failed job
/prow-ci <prow-job-url>

# Or ask naturally:
"Analyze the lint failure in PR <NUMBER>"
"Check why the validate job failed"
"Show me what broke in the coverage job"
```

## Implementation

When invoked, this skill:

1. **Fetches artifacts** using `fetch_prow_artifacts.py`:
   - Downloads **prowjob.json** (job metadata)
   - Downloads **build-log.txt** (complete build output with all errors)
   - Saves to `.work/prow-artifacts/<build-id>/`

2. **Analyzes failures** using `analyze_failure.py`:
   - Parses build-log.txt for error patterns
   - Detects common failure patterns (lint, build, timeout, OOM)
   - Extracts error messages and stack traces
   - Identifies compilation errors and test failures

3. **Generates report**:
   - Markdown format with failure summary
   - Pattern detection (compilation errors, lint failures, timeouts)
   - Top error messages and failures
   - Actionable failure details

## Usage Instructions

### Step 1: Get Prow Job URL

```bash
gh pr checks <PR_NUMBER>

# Or get detailed status
gh pr view <PR_NUMBER> --json statusCheckRollup --jq '.statusCheckRollup[] | select(.state == "FAILURE")'
```

### Step 2: Fetch and Analyze

Run the fetch script from repository root:
```bash
python3 .claude/skills/prow-ci/fetch_prow_artifacts.py "<prow-job-url>" -o .work/prow-artifacts
```

### Step 3: Analyze Failures

```bash
python3 .claude/skills/prow-ci/analyze_failure.py .work/prow-artifacts/<build-id> -f markdown
```

### Step 4: Present Findings

Create a clear summary for the user with:
- Root cause identification
- Detected patterns (lint, build, timeout, etc.)
- Key error messages
- Actionable next steps to fix the issue

## Common Job Names

**Prow CI Jobs** (configured in openshift/release):
- `pull-ci-openshift-aws-vpce-operator-master-e2e-binary-build-success`
- `pull-ci-openshift-aws-vpce-operator-master-coverage`
- `pull-ci-openshift-aws-vpce-operator-master-lint`
- `pull-ci-openshift-aws-vpce-operator-master-test`
- `pull-ci-openshift-aws-vpce-operator-master-validate`

**Tekton Pipelines** (configured in `.tekton/`):
- `aws-vpce-operator-pull-request` - Main PR pipeline
- `aws-vpce-operator-e2e-pull-request` - E2E testing pipeline
- `aws-vpce-operator-pko-pull-request` - PKO pipeline
- Corresponding `-push` pipelines for merged commits

## Debugging CI Failures

### Reproduce Locally
```bash
# For unit tests
make go-test

# For linting
make go-check
# OR use prek
prek run --all-files

# For validation
make validate

# For container builds (Tekton pipelines)
make docker-build
```

## Prow Resources

**Main Dashboard**: https://prow.ci.openshift.org/
**CI Search**: https://github.com/openshift/ci-search
**Job History**: https://prow.ci.openshift.org/?repo=openshift%2Faws-vpce-operator

## References

- [Prow Dashboard](https://prow.ci.openshift.org/)
- [CI Search Tool](https://github.com/openshift/ci-search)
- [OpenShift CI Documentation](https://docs.ci.openshift.org/)
