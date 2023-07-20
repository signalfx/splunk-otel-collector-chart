#!/bin/bash
# This script generates a changelog for the Splunk OpenTelemetry Collector Chart release.
# It reads the component version information from a CSV file and generates a markdown
# formatted changelog. The changelog includes the version number, release date,
# and the component versions. The changelog is then inserted into the existing
# CHANGELOG.md file, replacing the '## Unreleased' placeholder.
# This allows for an easy and standardized way to keep track of changes
# between different versions of the chart.

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
CHANGELOG_FILE="$SCRIPT_DIR/../CHANGELOG.md"
COMPONENT_VERSIONS_FILE="$SCRIPT_DIR/resources/components.csv"

# Generate changelog using component CSV
VERSION=$(grep "^chart," "$COMPONENT_VERSIONS_FILE" | awk -F',' '{print $3}')
DATE=$(date +%Y-%m-%d)
NEW_CONTENT="## Unreleased\n\n## [v$VERSION] - $DATE\n\nThis Splunk OpenTelemetry Collector Chart for Kubernetes release adopts the following components\n"

# Skip the first line (header)
while IFS=',' read -r NAME NAME_DOC VERSION RELEASE_URL SOURCE_REF QUERY; do
    NEW_CONTENT+="\n- $NAME_DOC - [$VERSION]($RELEASE_URL)"
done < <(tail -n +2 "$COMPONENT_VERSIONS_FILE")

# Replace '## Unreleased' with the new content
awk -v var="$NEW_CONTENT" '/## Unreleased/ {print var; next} 1' $CHANGELOG_FILE > tmp && mv tmp $CHANGELOG_FILE
