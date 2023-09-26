#!/bin/bash
# Purpose: Validates the presence of a changelog entry based on file changes.
# Notes:
#   - Should be executed via the `make chlog-validate` command.
#   - Checks if certain types of files have been modified to require a changelog entry.
#   - Designed to be a local check or to be used within a CI/CD pipeline.
#   - Finds the last common commit with the main branch.
#   - Checks for changes in specific directories and files:
#       1. Helm chart templates in 'helm-charts/splunk-otel-collector/templates/*'
#       2. Rendered manifests in 'examples/*/rendered_manifests/*'
#   - Requires a '.chloggen' file if any of the above conditions is met.

# Include the base utility functions for setting and debugging variables
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/base_util.sh"

# ---- Initialize Variables ----
# Get the current branch name
setd "CURRENT_BRANCH" $(git rev-parse --abbrev-ref HEAD)
# Get the last common commit with the main branch
setd "LAST_COMMON_COMMIT" $(git merge-base $CURRENT_BRANCH main)
# Initialize variables to keep track of changes
setd "HELM_CHART_UPDATED" 0
setd "RENDERED_MANIFESTS_UPDATED" 0
setd "CHLOGGEN_FILE_PRESENT" 0
# Get a list of all changed files since the last common commit with main
setd "COMMITTED_CHANGED_FILES" $(git diff --name-only $LAST_COMMON_COMMIT HEAD)
# Include uncommitted changes
setd "UNCOMMITTED_CHANGED_FILES" $(git diff --name-only)
# Combine both lists
setd "CHANGED_FILES" "$COMMITTED_CHANGED_FILES $UNCOMMITTED_CHANGED_FILES"

# ---- File Change Analysis ----
# Assess each modified file to determine if a changelog entry is required
for file in $CHANGED_FILES; do
    # Monitor changes within the Helm chart templates
    if [[ "$file" == helm-chart/splunk-otel-collector/templates* ]]; then
        setd "HELM_CHART_UPDATED" 1
    fi

    # Monitor changes within the rendered manifests
    if [[ "$file" == examples/*/rendered_manifests* ]]; then
        setd "RENDERED_MANIFESTS_UPDATED" 1
    fi

    # Track the presence of a .chloggen file indicating a changelog entry
    if [[ "$file" == *.chloggen ]]; then
        setd "CHLOGGEN_FILE_PRESENT" 1
    fi
done

# ---- Changelog Entry Validation ----
# Ensure that if critical files are modified, a corresponding changelog entry exists
if [[ $HELM_CHART_UPDATED -eq 1 ]] || [[ $RENDERED_MANIFESTS_UPDATED -eq 1 ]]; then
    if [[ $CHLOGGEN_FILE_PRESENT -eq 0 ]]; then
        echo "A changelog entry (.chloggen) is required for this commit. Reason:"
        if [[ $HELM_CHART_UPDATED -eq 1 ]]; then
          echo "- Updates to files under helm-chart/splunk-otel-collector/templates "
        fi
        if [[ $RENDERED_MANIFESTS_UPDATED -eq 1 ]]; then
            echo "- Updates to files under examples/*/rendered_manifests"
        fi
        exit 1
    fi
fi

# This format matches the chloggen tool format
echo "PASS: all changelog entries required for PR are valid"
exit 0
