#!/bin/bash
# Purpose: Validates the presence of a changelog entry based on file changes.
# Notes:
#   - Checks if certain types of files have been modified to require a changelog entry.
#   - Designed to be a pre-commit check or to be used within a CI/CD pipeline.
#
# Behavior:
#   - Finds the last common commit with the main branch.
#   - Checks for changes in specific directories and files:
#       1. Helm chart templates in 'helm-charts/splunk-otel-collector/templates/*'
#       2. Rendered manifests in 'examples/*/rendered_manifests/*'
#   - Requires a '.chloggen' file if any of the above conditions is met.
#
# Example Usage:
#   make chlog-pr-validate

# Get the current branch name
current_branch=$(git rev-parse --abbrev-ref HEAD)

# Get the last common commit with the main branch
last_common_commit=$(git merge-base $current_branch main)

# Initialize variables to keep track of changes
helm_chart_updated=0
rendered_manifests_updated=0
chloggen_file_present=0

# Get a list of all changed files since the last common commit with main
changed_files=$(git diff --name-only $last_common_commit HEAD)

# Include uncommitted changes
uncommitted_files=$(git diff --name-only)

# Combine both lists
all_changed_files="$changed_files $uncommitted_files"

# Loop through each changed file
for file in $all_changed_files; do
    # Check if any Helm chart templates are updated (recursive)
    if [[ "$file" == helm-chart/splunk-otel-collector/templates* ]]; then
        helm_chart_updated=1
    fi

    # Check if files under ./examples/*/rendered_manifests are updated (recursive)
    if [[ "$file" == examples/*/rendered_manifests* ]]; then
        rendered_manifests_updated=1
    fi

    # Check if a .chloggen file is present
    if [[ "$file" == *.chloggen ]]; then
        chloggen_file_present=1
    fi
done

# If Helm chart or rendered manifests are updated, ensure a .chloggen file is present
if [[ $helm_chart_updated -eq 1 ]] || [[ $rendered_manifests_updated -eq 1 ]]; then
    if [[ $chloggen_file_present -eq 0 ]]; then
        echo "A changelog entry (.chloggen) is required for this commit."
        exit 1
    fi
fi

echo "Successfully validated any required changelog entries exist for a PR."
exit 0
