#!/usr/bin/env bash
#
# Stop Hook: Prek Validation
#
# Runs prek validation when Claude Code stops with smart triggering:
#
# Default mode (CLAUDE_LINT_ON_STOP not set):
#   - Only runs when there are uncommitted changes
#   - Skips validation for read-only queries (fast iteration)
#   - Validates when Claude modifies code (catch issues before commit)
#
# Strict mode (export CLAUDE_LINT_ON_STOP=true):
#   - Always runs validation on every stop
#   - Use when you want maximum quality enforcement
#   - Slower but catches issues immediately
#
# Performance:
#   - Validates changed files only (5-10s typical)
#   - Uses hack/prek.ci.toml (skips network-dependent hooks)
#
set -uo pipefail

# Consume stdin first to avoid pipe deadlock
HOOK_INPUT=$(cat)

# Ensure we're running from the git repository root
REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null)
if [[ -z "$REPO_ROOT" ]]; then
  if command -v jq &> /dev/null; then
    jq -n '{"decision": "block", "reason": "Not in a git repository. Cannot run prek validation."}'
  else
    echo '{"decision": "block", "reason": "Not in a git repository. Cannot run prek validation."}'
  fi
  exit 0
fi
cd "$REPO_ROOT" || exit 1

# Check for jq dependency
if ! command -v jq &> /dev/null; then
  cat <<'EOF'
{"decision": "block", "reason": "jq is not installed — required for hook processing.\n\nInstall it:\n  brew install jq         # macOS\n  apt-get install jq      # Debian/Ubuntu\n  yum install jq          # RHEL/CentOS\n\nRetry the action once installed."}
EOF
  exit 0
fi

# Allow stop on retry to prevent infinite loops
STOP_HOOK_ACTIVE=$(echo "$HOOK_INPUT" | jq -r '.stop_hook_active // false')
if [[ "$STOP_HOOK_ACTIVE" == "true" ]]; then
  exit 0
fi

# Determine if validation should run
FORCE_LINT="${CLAUDE_LINT_ON_STOP:-false}"

if [[ "$FORCE_LINT" != "true" ]]; then
  if git diff-index --quiet HEAD -- 2>/dev/null && [[ -z "$(git ls-files --others --exclude-standard)" ]]; then
    exit 0
  fi
fi

# Check if prek is installed
if ! command -v prek &> /dev/null; then
  jq -n \
    --arg reason "prek is not installed — required for quality checks before stopping.

Install it:
  uv tool install prek      # recommended
  pipx install prek         # alternative
  pip install --user prek   # fallback

Then wire up the git hook: prek install

Retry the action once installed so validation can run." \
    '{"decision": "block", "reason": $reason}'
  exit 0
fi

# Run prek validation (using CI config to skip network-dependent hooks)
# Use null-delimited output to handle filenames with spaces safely
mapfile -d '' -t CHANGED_FILES < <(
  git diff -z --name-only --diff-filter=d HEAD
  git ls-files -z --others --exclude-standard
)
if [[ ${#CHANGED_FILES[@]} -eq 0 ]]; then
  PREK_OUTPUT=$(prek run --all-files --config hack/prek.ci.toml 2>&1)
else
  PREK_OUTPUT=$(prek run --config hack/prek.ci.toml --files "${CHANGED_FILES[@]}" 2>&1)
fi
PREK_EXIT=$?

if [[ $PREK_EXIT -eq 0 ]]; then
  exit 0
fi

jq -n \
  --arg reason "prek validation failed. Fix the issues below, then try again:

$PREK_OUTPUT" \
  '{"decision": "block", "reason": $reason}'
