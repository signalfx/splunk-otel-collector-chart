#!/bin/bash
# Purpose: Validates the presence of a changelog entry based on file changes.
# Notes:
#   - Should be executed via the `make chlog-validate` command.
#   - Designed to be a local or CI/CD pipeline check.
#   - Checks for changes in specific directories and files:
#       1. Helm chart templates in 'helm-charts/splunk-otel-collector/templates/*'
#       2. Rendered manifests in 'examples/*/rendered_manifests/*'
#   - Requires a '.chloggen' file if any of the above conditions is met.

# Include the base utility functions for setting and debugging variables
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/base_util.sh"

# ---- Git Fetch Main ----
if ! git show-ref --verify --quiet refs/heads/main; then
  echo "The main branch is not available. Fetching..."
  git fetch origin main:main
fi

# ---- Initialize Variables ----
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
LAST_COMMON_COMMIT=$(git merge-base $CURRENT_BRANCH main)
HELM_CHART_UPDATED=0
RENDERED_MANIFESTS_UPDATED=0
CHLOGGEN_FILE_PRESENT=0

# Get a list of all changed files since the last common commit with main
CHANGED_FILES=$(git diff --name-only $LAST_COMMON_COMMIT HEAD)
# Include uncommitted changes
UNCOMMITTED_CHANGED_FILES=$(git diff --name-only)

# Check if either COMMITTED_CHANGED_FILES or UNCOMMITTED_CHANGED_FILES is non-empty
if [[ -n "$UNCOMMITTED_CHANGED_FILES" ]]; then
    CHANGED_FILES="$CHANGED_FILES $UNCOMMITTED_CHANGED_FILES"
fi

# ---- File Change Analysis ----
for file in $CHANGED_FILES; do
  case "$file" in
    helm-charts/*/templates*)
        HELM_CHART_UPDATED=1
        ;;
    examples/*/rendered_manifests*)
        RENDERED_MANIFESTS_UPDATED=1
        ;;
    .chloggen*)
        CHLOGGEN_FILE_PRESENT=1
        ;;
  esac
done

# ---- Changelog Entry Validation ----
if { [[ $HELM_CHART_UPDATED -eq 1 ]] || [[ $RENDERED_MANIFESTS_UPDATED -eq 1 ]]; } && [[ $CHLOGGEN_FILE_PRESENT -eq 0 ]]; then
    printf "Changed Files:\n${CHANGED_FILES}\n"
    echo "FAIL: A changelog entry (.chloggen) is required for this commit due to:"
    [[ $HELM_CHART_UPDATED -eq 1 ]] && echo "- Updates to files under 'helm-charts/*/templates*'"
    [[ $RENDERED_MANIFESTS_UPDATED -eq 1 ]] && echo "- Updates to files under 'examples/*/rendered_manifests*'"
    exit 1
fi

echo "PASS: all changelog entries required for PR are valid"
exit 0
