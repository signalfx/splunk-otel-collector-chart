#!/bin/bash
# Purpose: Updates CRDs for the OpenTelemetry Operator in the opentelemetry-operator-crds subchart
# The script compares the current CRDs with those found in the appVersion of the Opentelemetry Operator
# we are using as dependency. If they differ, it updates the CRDs in the opentelemetry-operator-crds subchart.

# Include the base utility functions for setting and debugging variables
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/base_util.sh"

# Initialize directories and paths
setd "ROOT_DIR" "${SCRIPT_DIR}/../"
setd "COLLECTOR_CHART_DIR" "${ROOT_DIR}/helm-charts/splunk-otel-collector"
setd "CRD_CHART_DIR" "${COLLECTOR_CHART_DIR}/charts/opentelemetry-operator-crds"
setd "CRD_DIR" "${CRD_CHART_DIR}/crds"
setd "CRD_CHART_FILE" "${CRD_CHART_DIR}/Chart.yaml"
setd "OTEL_OPERATOR_REPO" "https://github.com/open-telemetry/opentelemetry-operator.git"
setd "TEMP_DIR" "$(mktemp -d)"

trap "rm -rf $TEMP_DIR" EXIT

# Function: get_operator_app_version
# Description: Retrieves the app version of the OpenTelemetry operator dependency
get_operator_app_version() {
    debug "Retrieving OpenTelemetry operator version"

    local operator_version
    local operator_repo

    operator_version=$(yq eval '.dependencies[] | select(.name == "opentelemetry-operator") | .version' "${COLLECTOR_CHART_DIR}/Chart.yaml")
    operator_repo=$(yq eval '.dependencies[] | select(.name == "opentelemetry-operator") | .repository' "${COLLECTOR_CHART_DIR}/Chart.yaml")

    debug "Pulling OpenTelemetry operator Helm chart version ${operator_version}"
    helm pull opentelemetry-operator --version "$operator_version" --repo "$operator_repo" --untar --untardir "$TEMP_DIR/chart"

    local app_version
    app_version=$(yq eval '.appVersion' "$TEMP_DIR/chart/opentelemetry-operator/Chart.yaml")

    echo "$app_version"
}

# Function: bump_patch_version
# Description: Bumps the patch version of the CRDs chart and updates the dependency in the collector chart.
bump_patch_version() {
    local current_version
    current_version=$(yq eval '.version' "$CRD_CHART_FILE")
    debug "Current CRDs chart version: $current_version"

    local major minor patch
    IFS='.' read -r major minor patch <<< "$current_version"
    patch=$((patch + 1))
    local new_version="${major}.${minor}.${patch}"

    yq eval ".version = \"$new_version\"" -i "$CRD_CHART_FILE"
    echo "CRDs chart version bumped to $new_version"

    yq eval "(.dependencies[] | select(.name == \"opentelemetry-operator-crds\") | .version) = \"$new_version\"" -i "${COLLECTOR_CHART_DIR}/Chart.yaml"
    echo "Updated CRDs chart version in chart dependencies to $new_version"
}

# Function: update_crds
# Description: Updates the OpenTelemetry CRDs if there are changes
update_crds() {
    local latest_version
    latest_version=$(get_operator_app_version)
    setd "LATEST_VERSION" "$latest_version"
    echo "OpenTelemetry operator app version: $latest_version"

    debug "Cloning OpenTelemetry Operator repository..."
    git config --global advice.detachedHead false  # Suppress detached HEAD warnings
    git clone --quiet --depth 1 --branch "v${latest_version}" ${OTEL_OPERATOR_REPO} ${TEMP_DIR}/opentelemetry-operator || {
        echo "Error: Failed to clone OpenTelemetry Operator repository."
        exit 1
    }

    mkdir -p "${CRD_DIR}"
    cp ${TEMP_DIR}/opentelemetry-operator/config/crd/bases/*.yaml "${CRD_DIR}/" 2>/dev/null

    if git diff --quiet -- "${CRD_DIR}"; then
        echo "No CRD updates detected. CRDs are already up to date."
        setd "CRDS_NEED_UPDATE" 0
    else
        echo "CRD updates detected for OpenTelemetry Operator."
        setd "CRDS_NEED_UPDATE" 1
        bump_patch_version
    fi

    setd "CRDS_LATEST_VERSION" "$(yq eval '.version' "$CRD_CHART_FILE")"
    emit_output "CRDS_NEED_UPDATE"
    emit_output "CRDS_LATEST_VERSION"
}

update_crds
