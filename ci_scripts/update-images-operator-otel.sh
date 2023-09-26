#!/bin/bash
# Purpose: Updates OpenTelemetry and Splunk images for auto-instrumentation.
# Notes:
#   - OpenTelemetry images are centralized and may change with operator subchart updates.
#   - Splunk images are decentralized and have a separate update mechanism and release cadence.
#
# Example Usage:
#   ./update-images-operator-otel.sh
#   ./update-images-operator-otel.sh --debug

# Include the base utility functions for setting and debugging variables
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/base_util.sh"

# ---- Initialize Temporary Files ----
# Create a temporary file to hold a subsection of the values.yaml file
setd "TEMP_VALUES_FILE" "$SCRIPT_DIR/temp_values_subsection.yaml"
# Create a temporary file to store version information
setd "TEMP_VERSIONS" "$SCRIPT_DIR/versions.txt"

# ---- Operator Subchart Version Extraction ----
# Extract the version of the opentelemetry-operator subchart from the main Chart.yaml
# This version helps us fetch the corresponding appVersion and image versions.
SUBCHART_VERSION=$(yq eval '.dependencies[] | select(.name == "opentelemetry-operator") | .version' "$CHART_FILE_PATH")
echo "Opentelemetry Operator Subchart Version: $SUBCHART_VERSION"

# ---- Fetching App Version ----
# Fetch the appVersion corresponding to the Operator subchart Version.
# This is extracted from the subchart's definition in the Chart.yaml file.
SUBCHART_URL="https://raw.githubusercontent.com/open-telemetry/opentelemetry-helm-charts/opentelemetry-operator-$SUBCHART_VERSION/charts/opentelemetry-operator/Chart.yaml"
debug "Fetching: $SUBCHART_URL"
APP_VERSION=$(curl -s "$SUBCHART_URL" | grep 'appVersion:' | awk '{print $2}')
debug "Operator App Version: $APP_VERSION"

# ---- Fetch Version Mapping ----
# Fetch the version mappings from versions.txt for the fetched appVersion.
# This gives us a mapping of image keys to their corresponding version tags.
VERSIONS_URL="https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/v$APP_VERSION/versions.txt"
debug "Fetching: $VERSIONS_URL"
curl -s "$VERSIONS_URL" > "$TEMP_VERSIONS"
debug "Values from Operator OpenTelemetry versions.txt file containing image tags"
debug "$TEMP_VERSIONS"

# ---- Extract Subsection for Update ----
# Extract the content between "# Auto-instrumentation Libraries (Start)" and "# Auto-instrumentation Libraries (End)"
awk '/# Auto-instrumentation Libraries \(Start\)/,/# Auto-instrumentation Libraries \(End\)/' "$VALUES_FILE_PATH" | grep -v "# Auto-instrumentation Libraries " > "$TEMP_VALUES_FILE"

# ---- Update Image Information ----
while IFS='=' read -r IMAGE_KEY VERSION; do
    NEED_UPDATE="${NEED_UPDATE:-0}"  # Sets NEED_UPDATE to its current value or 0 if not set
    if [[ "$IMAGE_KEY" =~ ^autoinstrumentation-.* ]]; then
        # Upstream Operator Values
        setd "INST_LIB_NAME" "${IMAGE_KEY#autoinstrumentation-}"
        setd "REPOSITORY_UPSTREAM" "ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-${INST_LIB_NAME}"
        setd "TAG_UPSTREAM" "${VERSION}"

        setd "REPOSITORY_LOCAL_PATH" "${INST_LIB_NAME}.repository"
        setd "REPOSITORY_LOCAL" "$(yq eval ".${REPOSITORY_LOCAL_PATH}" "${TEMP_VALUES_FILE}")"

        if [[ -z "${REPOSITORY_LOCAL}" || "${REPOSITORY_LOCAL}" != *"splunk"* ]]; then
          yq eval -i ".${REPOSITORY_LOCAL_PATH} = \"${REPOSITORY_UPSTREAM}\"" "${TEMP_VALUES_FILE}"

          setd "TAG_LOCAL_PATH" "${INST_LIB_NAME}.tag"
          setd "TAG_LOCAL" "$(yq eval ".${TAG_LOCAL_PATH}" "${TEMP_VALUES_FILE}")"
          if [[ -z "${TAG_LOCAL}" || "${TAG_LOCAL}" == "null" || "${TAG_LOCAL}" != "$TAG_UPSTREAM" ]]; then
            debug "Upserting value for ${REPOSITORY_LOCAL}:${TAG_LOCAL}"
            yq eval -i ".${TAG_LOCAL_PATH} = \"${TAG_UPSTREAM}\"" "${TEMP_VALUES_FILE}"
            setd "NEED_UPDATE" 1
          else
            debug "Retaining existing value for ${REPOSITORY_LOCAL}:${TAG_LOCAL}"
          fi
        else
          # Splunk instrumentation libraries are updated in a different workflow.
          debug "Skipping updating ${REPOSITORY_LOCAL}:${TAG_LOCAL}"
        fi
    fi
done < "${TEMP_VERSIONS}"

# Emit the NEED_UPDATE variable to either GitHub output or stdout
emit_output "NEED_UPDATE"

# Merge the updated subsection back into values.yaml
# This approach specifically updates only the subsection between the start and end tokens.
# By doing so, we avoid reformatting the entire file, thus preserving the original structure and comments.
awk '
  !p && !/# Auto-instrumentation Libraries \(Start\)/ && !/# Auto-instrumentation Libraries \(End\)/ { print $0; next }
  /# Auto-instrumentation Libraries \(Start\)/ {p=1; print $0; next}
  /# Auto-instrumentation Libraries \(End\)/ {p=0; while((getline line < "'$TEMP_VALUES_FILE'") > 0) printf "      %s\n", line; print $0; next}
' "$VALUES_FILE_PATH" > "${VALUES_FILE_PATH}.updated"

# Replace the original values.yaml with the updated version
mv "${VALUES_FILE_PATH}.updated" "$VALUES_FILE_PATH"
# Cleanup temporary files
rm "$TEMP_VALUES_FILE"
rm "$TEMP_VERSIONS"

echo "Image update process completed successfully!"
exit 0
