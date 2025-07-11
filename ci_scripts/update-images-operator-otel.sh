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
setd "TEMP_VALUES_FILE" "$SCRIPT_DIR/temp_values_subsection.out"
# Create a temporary file to store operator main.go code containing docker image repository information
setd "TEMP_MAIN_FILE" "$SCRIPT_DIR/temp_main.out"
# Create a temporary file to store docker image version information
setd "TEMP_VERSIONS" "$SCRIPT_DIR/temp_versions.out"

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

# ---- Fetch Docker Repository Information ----
# Fetch the code containing information about what instrumentation language uses what docker repository.
MAIN_URL="https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/v$APP_VERSION/main.go"
debug "Fetching: $MAIN_URL"
curl -s "$MAIN_URL" > "$TEMP_MAIN_FILE"
debug "Extracted main.go of the operator:"
debug "$TEMP_MAIN_FILE"

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
        setd "INST_LIB_NAME_RAW" "${IMAGE_KEY#autoinstrumentation-}"
        # Map upstream names to expected instrumentation key names
        case "${INST_LIB_NAME_RAW}" in
            apache-httpd)
                INST_LIB_NAME="apacheHttpd"
                ;;
            *)
                INST_LIB_NAME="${INST_LIB_NAME_RAW}"
                ;;
        esac
        setd "IMAGE_LOCAL_PATH" "${INST_LIB_NAME}.image"
        setd "IMAGE_LOCAL" "$(yq eval ".${IMAGE_LOCAL_PATH}" "${TEMP_VALUES_FILE}")"
        # Splunk instrumentation libraries are updated in a different workflow.
        if [[ "${IMAGE_LOCAL}" == *"splunk"* ]]; then
            debug "Skipping updating ${IMAGE_LOCAL_PATH}"
            continue
        fi
        setd "TAG_UPSTREAM" "${VERSION}"
        # Find the proper docker repository for the instrumentation library by scraping the main.go file of the operator
        INST_LIB_REPO=$(grep "auto-instrumentation-${INST_LIB_NAME_RAW}-image" "$TEMP_MAIN_FILE" | grep -o 'ghcr.io/[a-zA-Z0-9_-]*/[a-zA-Z0-9_-]*/autoinstrumentation-[a-zA-Z0-9_-]*' | sort | uniq )
        if [ -n "$INST_LIB_REPO" ]; then
            # Set the REPOSITORY_UPSTREAM variable
            setd "REPOSITORY_UPSTREAM" "$INST_LIB_REPO"
            debug "Set REPOSITORY_UPSTREAM to ${INST_LIB_REPO}"
        else
            echo "Failed to find repository for ${INST_LIB_NAME_RAW}"
            exit 1
        fi
        IMAGE_UPSTREAM="${REPOSITORY_UPSTREAM}:${TAG_UPSTREAM}"
        # Only update if the current image does not match the upstream value
        if [[ -z "${IMAGE_LOCAL}" || "${IMAGE_LOCAL}" != "${IMAGE_UPSTREAM}" ]]; then
            # Skopeo validates the existence of the Docker images no matter the current host architecture used
            if skopeo inspect --retry-times 3 --raw "docker://${IMAGE_UPSTREAM}" &>/dev/null; then
                echo "Image ${IMAGE_UPSTREAM} exists."
                echo "Upserting value for ${IMAGE_LOCAL_PATH}: ${IMAGE_UPSTREAM}"
                yq eval -i ".${IMAGE_LOCAL_PATH} = \"${IMAGE_UPSTREAM}\"" "${TEMP_VALUES_FILE}"
                setd "NEED_UPDATE" 1
            else
                echo "Failed to find Docker image ${IMAGE_UPSTREAM}. Check image repository and tag."
                exit 1
            fi
        else
            debug "Retaining existing value for ${IMAGE_LOCAL_PATH}: ${IMAGE_LOCAL}"
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
  /# Auto-instrumentation Libraries \(End\)/ {p=0; while((getline line < "'$TEMP_VALUES_FILE'") > 0) printf "    %s\n", line; print $0; next}
' "$VALUES_FILE_PATH" > "${VALUES_FILE_PATH}.updated"

# Replace the original values.yaml with the updated version
mv "${VALUES_FILE_PATH}.updated" "$VALUES_FILE_PATH"
# Cleanup temporary files
rm "$TEMP_MAIN_FILE" "$TEMP_VERSIONS" "$TEMP_VALUES_FILE"

echo "Image update process completed successfully!"
exit 0
