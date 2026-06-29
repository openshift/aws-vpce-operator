# Claude Code Hooks

Security and validation hooks for AWS VPCE Operator development.

## Overview

This repository uses **prek** (git hook manager) for quality checks and validation. Claude Code hooks integrate with prek to provide immediate feedback during development.

## Architecture

```text
+-------------------------------------+
|   Developer / Claude Code Agent     |
+---------------+---------------------+
                |
                v
+-------------------------------------+
|   Stop Hook (conditional)           |
|   - Default: runs only with changes |
|   - Strict: runs every turn         |
|   - Blocks if issues found          |
|   - Claude fixes automatically      |
+---------------+---------------------+
                |
                v
+-------------------------------------+
|   Prek Hooks (CI config)            |
|   - golangci-lint (static analysis) |
|   - RBAC wildcard check             |
|   - go build validation             |
|   - go mod tidy check               |
|   - file hygiene (trailing space)   |
+---------------+---------------------+
                |
+---------------v---------------------+
|   Prek Hooks (full config)          |
|   + rh-pre-commit (InfoSec)         |
|   + gitleaks (secret scanning)      |
+---------------+---------------------+
                |
                v
+-------------------------------------+
|   Git Commit                        |
+---------------+---------------------+
                |
                v
+-------------------------------------+
|   CI/CD (Tekton Pipelines)          |
+-------------------------------------+
```

## Available Hooks

### [stop-prek-validation.sh](./stop-prek-validation.sh)
**Purpose**: Run prek validation when Claude makes changes (or always, if configured)

**Triggers**: On Claude Code session stop (Stop hook)

**Behavior**:

**Default mode** (recommended):
- Only runs if there are uncommitted changes (staged, unstaged, or untracked files)
- Skips validation for read-only queries (fast iteration)
- Validates when Claude modifies code (before commit)

**Strict mode** (opt-in):
- Set environment variable: `export CLAUDE_LINT_ON_STOP=true`
- Always runs validation on every stop, regardless of changes

**Common behavior**:
- Runs `prek run --config hack/prek.ci.toml` on changed files
- Uses CI-compatible config (skips network-dependent hooks like rh-pre-commit, gitleaks)
- Blocks Claude from stopping if issues found
- Feeds errors back to Claude for automatic fixes
- Includes infinite loop guard (allows stop on retry)

**Performance**:
- Default mode, clean working directory: 0s (skipped)
- Default mode, with changes: 5-10s typical (changed files only)
- Strict mode (CLAUDE_LINT_ON_STOP=true): 5-10s every stop

**Installation**: Configured in `.claude/settings.json`

**Enable strict mode**:
```bash
# In your shell profile (~/.zshrc, ~/.bashrc)
export CLAUDE_LINT_ON_STOP=true

# Or for single session
CLAUDE_LINT_ON_STOP=true claude
```

---

### [pre-edit.sh](./pre-edit.sh)
**Purpose**: Prevent editing generated files and warn about high-risk changes

**Status**: Available for standalone use (not configured as Claude Code hook)

**Checks**:
- Generated files (`zz_generated.*.go`)
- Generated mocks (`**/generated/mock_*.go`)
- Vendored code (`vendor/`)
- Boilerplate files (managed upstream)
- High-risk security files (RBAC, auth, NetworkPolicy)
- CI/CD pipelines (`.tekton/*.yaml`)
- Dockerfiles

**Manual Usage**:
```bash
.claude/hooks/pre-edit.sh path/to/file.go
```

---

## Prek Configuration

This repository uses **prek** as the hook runner with two configurations:

### 1. `.pre-commit-config.yaml` (Full validation)
Used for local development with internal network access. This file is boilerplate-managed.

**Hooks**:
- File hygiene (trailing whitespace, EOF, syntax checks)
- **rh-pre-commit**: Red Hat InfoSec security checks (requires `gitlab.cee.redhat.com` access)
- **gitleaks**: Secret detection (configured via `.gitleaks.toml`)
- **golangci-lint**: Static analysis
- **go-build**: Compile check
- **go-mod-tidy**: Dependency drift detection
- **rbac-wildcard-check**: RBAC validation

**Usage**:
```bash
prek run  # Uses .pre-commit-config.yaml by default
```

### 2. **hack/prek.ci.toml** (CI-compatible)
Used by Claude Code stop hook and CI environments without internal network access.

**Excludes**:
- `rh-pre-commit` (requires Red Hat internal network)
- `gitleaks` (may not be available in all CI environments)

**Usage**:
```bash
hack/ci.sh
# or
prek run --config hack/prek.ci.toml --all-files
```

**Why two configs?**
The CI-compatible config allows Claude Code and external CI systems to run quality checks without requiring access to Red Hat's internal GitLab instance.

## Setup

### Prerequisites
```bash
# Install prek (choose one)
uv tool install prek      # recommended
pipx install prek         # alternative
pip install --user prek   # fallback
```

### Install Git Hooks
```bash
prek install
```

This sets up pre-commit hooks that run validation automatically.

## Usage

### Automatic Validation
Prek runs automatically:
- **Stop hook (default mode)**: Runs `prek run --config hack/prek.ci.toml` only when changes are present
- **Stop hook (strict mode)**: Set `CLAUDE_LINT_ON_STOP=true` to run on every turn
- **On commit**: Pre-commit hook runs relevant checks

### Manual Validation
```bash
# Run all checks
prek run --all-files

# Run CI-compatible subset
prek run --config hack/prek.ci.toml

# Run specific check
prek run gitleaks
prek run golangci-lint
prek run rbac-wildcard-check
```

## Security Guardrails

### Secret Prevention
**Implementation**: gitleaks via prek

**Configuration**: `.gitleaks.toml`

**Action**: BLOCK commit

### InfoSec Scanning
**Implementation**: rh-pre-commit via prek

**Source**: Red Hat InfoSec Developer Workbench

**Action**: BLOCK commit on violations

### RBAC Validation
**Implementation**: rbac-wildcard-check via prek

**Action**: BLOCK commit

### Hook Failures (DO NOT Bypass)

**NEVER bypass hooks:**
```bash
# FORBIDDEN - bypasses all validation
git commit --no-verify

# FORBIDDEN - bypasses specific hooks
SKIP=hook-id git commit
```

**If hooks are blocking your commit:**
1. **Investigate and fix the root issue** - hooks catch real problems
2. **If the hook or config is broken:**
   - Fix the hook/config first
   - Open an issue documenting the problem
   - Request reviewer approval before merge
3. **Re-run full validation:**
   - `prek run --config hack/prek.ci.toml` locally
   - Ensure all required CI checks pass
   - Get explicit code review approval

**Security hooks (gitleaks, rh-pre-commit) must NEVER be bypassed under any circumstances.**

## Version Management

### Prek Version
Pinned in `.prek-version` for CI consistency:
```bash
cat .prek-version  # v0.4.1
```

### Hook Dependencies
Defined in `.pre-commit-config.yaml` with immutable refs:
- `rh-pre-commit-2.3.0`
- `v8.18.0` (gitleaks)
- `v2.0.2` (golangci-lint)

## References

- [Prek Documentation](https://prek.j178.dev/)
- [Gitleaks](https://github.com/gitleaks/gitleaks)
- [RH InfoSec Tools](https://gitlab.cee.redhat.com/infosec-public/developer-workbench/tools)
- [golangci-lint](https://golangci-lint.run/)
- [CLAUDE.md](../../CLAUDE.md) - Development guidelines
