#!/bin/bash
# Simplifies the process of preparing a new helm chart release.
# Usage:
# ./prepare-release.sh
# Environment Variables:
# CREATE_BRANCH - If set to "false", changes remain local. Default is "true" to push changes.
# CHART_VERSION - Optionally overrides the chart version in Chart.yaml.
# APP_VERSION - Optionally overrides the app version in Chart.yaml.

# Include the base utility functions for setting and debugging variables
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/base_util.sh"

update_versions() {
    yq e ".version = \"$LATEST_CHART_VERSION\"" -i "${CHART_FILE_PATH}"
    yq e ".appVersion = \"$LATEST_APP_VERSION\"" -i "${CHART_FILE_PATH}"
}

notify_workflows_for_need_update() {
    NEED_UPDATE=1
    emit_output "NEED_UPDATE"
    emit_output "LATEST_CHART_VERSION"
    emit_output "LATEST_APP_VERSION"
}

prepare_release() {
    echo "Preparing release: $LATEST_CHART_VERSION with app version: $LATEST_APP_VERSION"
    update_versions
    make render && make chlog-update
    git add .

    if ! git diff --staged --quiet; then
        notify_workflows_for_need_update
        if [[ "$CREATE_BRANCH" == "true" ]]; then
            BRANCH_NAME="release-update"
            setup_branch "$BRANCH_NAME" "$OWNER/splunk-otel-collector-chart"
            git commit -m "Prepare release $LATEST_CHART_VERSION"
            git push -u origin "$BRANCH_NAME"
            echo "Created branch: $BRANCH_NAME"
        else
            echo "Changes are local but not committed or pushed. CREATE_BRANCH is not set to true."
        fi
    else
        echo "No changes to commit."
    fi
}

CHART_VERSION_OVERRIDDEN=${CHART_VERSION:+true}
APP_VERSION_OVERRIDDEN=${APP_VERSION:+true}
GITHUB_EVENT_NAME=${GITHUB_EVENT_NAME:-workflow_dispatch}

# Fetch or set default versions
CURRENT_CHART_VERSION=$(yq e ".version" "${CHART_FILE_PATH}")
CURRENT_APP_VERSION=$(yq e ".appVersion" "${CHART_FILE_PATH}")
CREATE_BRANCH=${CREATE_BRANCH:-true}
CURRENT_CHART_VERSION_MAJOR=$(get_major_version "v$CURRENT_CHART_VERSION")
CURRENT_CHART_VERSION_MINOR=$(get_minor_version "v$CURRENT_CHART_VERSION")

# This is the version to use with the new release.
LATEST_APP_VERSION=$(curl -L -qs -H 'Accept: application/vnd.github+json' https://api.github.com/repos/"$OWNER"/splunk-otel-collector/releases/latest | jq -r .tag_name | sed 's/^v//')
if [[ "$APP_VERSION_OVERRIDDEN" = true ]]; then
    LATEST_APP_VERSION=$APP_VERSION
    debug "Using override collector app version value $LATEST_APP_VERSION"
fi
LATEST_CHART_VERSION_PATCH=$(get_major_version "v$LATEST_APP_VERSION")
LATEST_CHART_VERSION_MINOR=$(get_minor_version "v$LATEST_APP_VERSION")

# This will be the new version of the chart for the new release
LATEST_CHART_VERSION=$CURRENT_CHART_VERSION
# Check if chart version is overridden explicitly via environment variable
if [[ "$CHART_VERSION_OVERRIDDEN" == "true" ]]; then
    LATEST_CHART_VERSION=$CHART_VERSION
    debug "Using override chart version value $LATEST_CHART_VERSION"
# If the trigger is a manual dispatch or a scheduled event with differing current and latest collector app version
elif [[ "$GITHUB_EVENT_NAME" == "workflow_dispatch" ]] || ( [[ "$GITHUB_EVENT_NAME" == "schedule" ]] && [[ "$CURRENT_APP_VERSION" != "$LATEST_APP_VERSION" ]] ); then
    # If the major and minor versions of the chart and app match...
    if [[ "$CURRENT_CHART_VERSION_MAJOR" -eq "$LATEST_CHART_VERSION_PATCH" && "$CURRENT_CHART_VERSION_MINOR" -eq "$LATEST_CHART_VERSION_MINOR" ]]; then
        # Increment the chart's patch version
        CURRENT_CHART_VERSION_PATCH=$(get_patch_version "v$CURRENT_CHART_VERSION")
        LATEST_CHART_VERSION="$CURRENT_CHART_VERSION_MAJOR.$CURRENT_CHART_VERSION_MINOR.$((CURRENT_CHART_VERSION_PATCH + 1))"
        debug "Incrementing chart version to $LATEST_CHART_VERSION"
    else
        # If major or minor versions don't match, align chart version with app version (set patch to 0)
        LATEST_CHART_VERSION="$LATEST_CHART_VERSION_PATCH.$LATEST_CHART_VERSION_MINOR.0"
        debug "Aligning chart version to $LATEST_CHART_VERSION due to major.minor mismatch with app version"
    fi
else
  echo "No update required. Current release is up to date."
  exit 0
fi

# Check if the computed LATEST_CHART_VERSION already exists in the Helm repo to avoid duplicates
if make dep-update && helm search repo splunk-otel-collector-chart/splunk-otel-collector --versions | grep -q "splunk-otel-collector-$LATEST_CHART_VERSION"; then
    echo "Version $LATEST_CHART_VERSION already exists. Exiting."
    exit 1
fi

setup_git
prepare_release
