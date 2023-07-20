#!/bin/bash
# This script reads the component version information from the Chart.yaml
# and values.yaml files in the Splunk OpenTelemetry Collector Chart. It outputs
# this information to a CSV file which contains the component name, version, source
# reference, and the yq query used to extract the version.
# The CSV file can be used to keep track of the different component versions
# used in the chart.

COMPONENT_CSV="ci_scripts/resources/components.csv"
echo "name, name_documentation, version,release_url,source_reference,query" > "$COMPONENT_CSV"

VERSION=$(yq eval '.version' helm-charts/splunk-otel-collector/Chart.yaml)
echo "chart,Splunk OpenTelemetry Collector Chart,$VERSION,https://github.com/signalfx/splunk-otel-collector-chart/releases/tag/splunk-otel-collector-$VERSION,helm-charts/splunk-otel-collector/Chart.yaml,.version" >> "$COMPONENT_CSV"

APP_VERSION=$(yq eval '.appVersion' helm-charts/splunk-otel-collector/Chart.yaml)
echo "collector,Splunk OpenTelemetry Collector App Version,$APP_VERSION,https://github.com/signalfx/splunk-otel-collector/releases/tag/v$VERSION,helm-charts/splunk-otel-collector/Chart.yaml,.appVersion" >> "$COMPONENT_CSV"

NETWORK_EXPLORER_VERSION=$(yq eval '.networkExplorer.images.tag' helm-charts/splunk-otel-collector/values.yaml)
NETWORK_EXPLORER_VERSION=${NETWORK_EXPLORER_VERSION#v}
echo "networkExplorer,Splunk Network Explorer,$NETWORK_EXPLORER_VERSION,https://quay.io/repository/signalfx/splunk-network-explorer-kernel-collector?tab=tags,helm-charts/splunk-otel-collector/Chart.yaml,.Values.networkExplorer.images.tag" >> "$COMPONENT_CSV"

DEPENDENCIES=$(yq eval '.dependencies[] | [.name, .version, .repository] | @csv' helm-charts/splunk-otel-collector/Chart.yaml)
while IFS=$',' read -r NAME VERSION REPOSITORY; do
  echo "$NAME,$NAME chart,$VERSION,$REPOSITORY,helm-charts/splunk-otel-collector/Chart.yaml,.dependencies[] | select(.name == \"$NAME\").version" >> "$COMPONENT_CSV"
done <<< "$DEPENDENCIES"

echo "Component versions CSV generated successfully!"
cat "$COMPONENT_CSV"
