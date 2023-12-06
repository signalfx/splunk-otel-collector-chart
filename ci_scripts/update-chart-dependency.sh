#!/bin/bash
# Purpose: Updates Helm chart dependencies and related image tags.
# Notes:
#   - Automates the process of updating Helm chart dependencies in Chart.yaml and related image tags in values.yaml.
#   - Specifically designed to update the opentelemetry-operator as a subchart and related instrumentation configurations in values.yaml.
#   - Relies on Helm and yq tools for processing Helm charts and YAML files.
#
# Parameters:
#   $1: The file path to the Chart.yaml file of the Helm chart (mandatory).
#   $2: The name of the dependency (subchart) to check for updates (mandatory).
#   --debug: (Optional) Activates debug mode for verbose output, aiding in troubleshooting.
#
# Usage Examples:
#   ./update-chart-dependency.sh ./path/to/Chart.yaml opentelemetry-operator
#   ./update-chart-dependency.sh ./path/to/Chart.yaml opentelemetry-operator --debug

# Include the base utility functions for setting and debugging variables
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/base_util.sh"

# ---- Validate Input Arguments ----
if [ "$#" -lt 2 ]; then
    echo "Error: Incorrect number of arguments provided."
    echo "Usage: $0 <path-to-chart-file> <yq-query-string> [--debug]"
    exit 1
fi

# ---- Initialize Variables ----
# Set the YAML file path and the yq query string
setd "CHART_PATH" "$1"
setd "SUBCHART_NAME" "$2"

# Function: update_operator_images
# Description: Updates OpenTelemetry operator images
update_operator_images() {
    # TODO: Migrate the logic from update-images-operator-otel.sh to here
    echo "Updating OpenTelemetry operator images for $SUBCHART_NAME..."
    $SCRIPT_DIR/update-images-operator-otel.sh
}

# Function: maybe_update_chart_dependency_version
# Description: Updates the chart dependency version if a newer version is available.
maybe_update_chart_dependency_version() {
    echo "Checking for updates to $SUBCHART_NAME in $CHART_PATH..."

    # Fetch the latest version using Helm
    LATEST_VER=$(helm search repo $SUBCHART_NAME --versions | awk 'NR==2{print $2}')
    echo "Latest version of $SUBCHART_NAME is $LATEST_VER"

    # Retrieve the current version from Chart.yaml
    CURRENT_VER=$(yq eval ".dependencies[] | select(.name == \"$SUBCHART_NAME\") | .version" $CHART_PATH)
    echo "Current version of $SUBCHART_NAME in Chart.yaml is $CURRENT_VER"

    if [ "$LATEST_VER" != "$CURRENT_VER" ]; then
      echo "Updating to new version $LATEST_VER in Chart.yaml"

      # Emit the NEED_UPDATE variable to either GitHub output or stdout
      NEED_UPDATE=1
      emit_output "NEED_UPDATE"
      emit_output "CURRENT_VER"
      emit_output "LATEST_VER"

      # Update the version in Chart.yaml
      yq eval -i "(.dependencies[] | select(.name == \"$SUBCHART_NAME\")).version = \"$LATEST_VER\"" $CHART_PATH

      if [ "$SUBCHART_NAME" == "opentelemetry-operator" ]; then
        update_operator_images
      fi

      echo "Current git diff:"
      git --no-pager diff
    else
      echo "We are already up to date. Nothing else to do."
    fi
}

# ---- Update Version Information ----
# Call the maybe_update_chart_dependency_version function to update the version if necessary
maybe_update_chart_dependency_version
