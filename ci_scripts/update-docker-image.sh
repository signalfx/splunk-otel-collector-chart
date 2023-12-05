#!/bin/bash
# Purpose: Automatically updates the Docker image tag in a YAML file to the latest version available.
# Notes:
#   - Retrieves the latest tag for a specified Docker image from quay.io, ghcr.io, or Docker Hub.
#   - Designed to streamline updates to Kubernetes manifests, Helm chart values, or similar YAML configurations.
#   - When multiple tags share the same image digest for the latest image, the script prefers the most specific version tag.
#
# Parameters:
#   $1: The file path to the YAML file containing the Docker image reference (mandatory).
#   $2: The yq query string to locate the image tag within the YAML file (mandatory).
#       Can be a direct path for simple structures or a complex yq query for nested structures.
#   --debug: (Optional) Activates debug mode for verbose output, aiding in troubleshooting.
#
# Usage Examples:
#   - Using a direct path to update a tag where '.images.splunk.repository' and '.images.splunk.tag' are present:
#     ./update-images-operator-splunk.sh ./path/to/values.yaml '.images.splunk'
#
#   - Using a direct path where the image value is in the 'owner/repo:tag' format:
#     ./update-images-operator-splunk.sh ./path/to/values.yaml '.images.splunk.image'
#
#   - Using a yq query for complex YAML structures like Kubernetes manifests:
#     ./update-images-operator-splunk.sh ./path/to/k8s-manifest.yaml 'select(.kind == "Pod").spec.containers[0].image'

# Include the base utility functions for setting and debugging variables
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/base_util.sh"

# ---- Validate Input Arguments ----
if [ "$#" -lt 2 ]; then
    echo "Error: Incorrect number of arguments provided."
    echo "Usage: $0 <path-to-yaml-file> <yq-query-string> [--debug]"
    exit 1
fi

# ---- Initialize Variables ----
# Set the YAML file path and the yq query string
setd "YAML_FILE_PATH" "$1"
setd "YQ_QUERY_STRING" "$2"

# ---- Update Version Information ----
# Call the maybe_update_version function to update the tag if necessary
maybe_update_version "$YAML_FILE_PATH" "$YQ_QUERY_STRING"

exit 0
