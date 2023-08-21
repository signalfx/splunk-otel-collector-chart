#!/bin/bash

set -e  # Exit script if any command fails

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
CHART_PATH="$SCRIPT_DIR/../..//helm-charts/splunk-otel-collector/Chart.yaml"
VALUES_PATH="$SCRIPT_DIR/../..//helm-charts/splunk-otel-collector/values.yaml"
TEMP_SUBSECTION="$SCRIPT_DIR/temp_subsection.yaml"

# Extract the version of the opentelemetry-operator subchart from the Main Chart's Chart.yaml
SUBCHART_VERSION=$(yq eval '.dependencies[] | select(.name == "opentelemetry-operator") | .version' $CHART_PATH)
echo "Opentelemetry Operator Subchart Version: $SUBCHART_VERSION"

# Fetch the appVersion using the Operator Subchart Version
SUBCHART_URL="https://raw.githubusercontent.com/open-telemetry/opentelemetry-helm-charts/opentelemetry-operator-$SUBCHART_VERSION/charts/opentelemetry-operator/Chart.yaml"
echo "Fetching: "$SUBCHART_URL
APP_VERSION=$(curl -s $SUBCHART_URL | grep 'appVersion:' | awk '{print $2}')
echo "Operator App Version: $APP_VERSION"

# Fetch the Version Mapping from the versions.txt for the fetched appVersion
VERSIONS_URL="https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/v$APP_VERSION/versions.txt"
echo "Fetching: "$VERSIONS_URL
curl -s $VERSIONS_URL > versions.txt

# Extract the subsection between "operator:" and just before "cert-manager:" to a temporary file
awk '/^operator:/,/^\s*$/' $VALUES_PATH | grep -v "^The cert-manager is a CNCF application" > $TEMP_SUBSECTION

# Update the subsection using your yq logic
while IFS='=' read -r image_key version; do
    if [[ $image_key =~ ^autoinstrumentation-.* ]]; then
        actual_key="${image_key#autoinstrumentation-}"
        image_name="ghcr.io/open-telemetry/opentelemetry-operator/$image_key:$version"
        yaml_path="operator.instrumentation.spec.${actual_key}.image"
        existing_value=$(yq eval ".${yaml_path}" $TEMP_SUBSECTION)
        if [[ "$existing_value" != "null" && "$existing_value" == *"splunk"* ]]; then
            echo "Retaining existing value for $yaml_path: $existing_value"
            continue
        fi
        echo "Updating value for $yaml_path: $image_name"
        yq eval -i ".${yaml_path} = \"$image_name\"" $TEMP_SUBSECTION
    fi
done < versions.txt
# Add a newline to the end of TEMP_SUBSECTION
echo "" >> $TEMP_SUBSECTION

# Merge the updated subsection back into values.yaml
awk -v start="^operator:" -v end="The cert-manager is a CNCF application" -v file="$TEMP_SUBSECTION" '
  !p && $0 !~ start && $0 !~ end { print $0; next }
  $0 ~ start {p=1; while((getline line < file) > 0) print line; next}
  $0 ~ end {p=0; print $0}
' $VALUES_PATH > "${VALUES_PATH}.updated"

# Replace the original values.yaml with the updated version
mv "${VALUES_PATH}.updated" $VALUES_PATH

# Cleanup
rm $TEMP_SUBSECTION
rm versions.txt

echo "Image update process completed successfully!"
