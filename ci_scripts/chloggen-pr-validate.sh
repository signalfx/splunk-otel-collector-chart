#!/bin/bash
# Purpose: Validates the presence of a changelog entry based on file changes.
# Notes:
#   - Should be executed via the `make chlog-validate` command.
#   - Designed to be a local or CI/CD pipeline check.
#   - Checks for changes in specific files:
#       1. Helm chart templates in 'helm-charts/*/templates/*'
#       2. Helm chart configurations in 'helm-charts/*/Chart.yaml'
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
CHLOGGEN_CONTENT_UPDATED=0

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
    # Helm chart template was updated
    helm-charts/*/templates*)
        HELM_CHART_UPDATED=1
        ;;
    # Helm chart version, appVersion, or dependency (subchart) version was updated in Chart.yaml
    helm-charts/*/Chart.yaml)
        HELM_CHART_UPDATED=1
        ;;
    # A new .chloggen file was added or existing content updated
    .chloggen*)
        CHLOGGEN_CONTENT_UPDATED=1
        ;;
    # A new CHANGELOG.md entry was generated from .chloggen content
    CHANGELOG.md)
        CHLOGGEN_CONTENT_UPDATED=1
        ;;
  esac
done

# ---- Changelog Entry Validation ----
if { [[ $HELM_CHART_UPDATED -eq 1 ]]; } && [[ $CHLOGGEN_CONTENT_UPDATED -eq 0 ]]; then
    printf "Changed Files:\n${CHANGED_FILES}\n"
    echo "FAIL: A new changelog file in .chloggen/ or entry in CHANGELOG.md is required for this commit due to:"
    echo "- Updates to files under 'helm-charts/*/templates*'"
    echo "- Updates to Chart.yaml files"
    echo "Please run \"make chlog-new\" for normal PRs or \"make chlog-update\" for PRs creating releases."
    exit 1
fi

echo "PASS: all changelog entries required for PR are valid"
exit 0
