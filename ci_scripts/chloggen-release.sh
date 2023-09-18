#!/bin/bash
# Purpose: Updates CHANGELOG.md file for a release.
# Notes:
#   - Intended to be used as part of the release process.
#   - Automates the process of hyperlinking PR IDs in the CHANGELOG.md.
#   - Should be executed via the `make chlog-update` command.
#
# Example Usage:
#   make chlog-update VERSION=0.85.0

# Include the base utility functions for setting and debugging variables
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# TODO: Add this
#source "$SCRIPT_DIR/base_util.sh"

# ---- Initialize Temporary Variables ----
# Create a temporary file to hold the updated CHANGELOG.md content
TEMP_CHANGELOG="CHANGELOG.md.tmp"

# ---- Update CHANGELOG.md for Subcontext and PR Links ----
# Insert subcontext about the Splunk Otel Collector used in this release
while IFS= read -r line; do
    if [[ $line =~ \(\#([0-9,# ]+)\)$ ]]; then
        pr_ids=${BASH_REMATCH[1]}
        replacement=""
        first=1
        IFS=',' read -ra ADDR <<< "$pr_ids"
        for i in "${ADDR[@]}"; do
            trimmed_i=$(echo "$i" | xargs)  # Remove leading/trailing whitespaces
            hyperlink="[#${trimmed_i}](https://github.com/signalfx/splunk-otel-collector-chart/pull/${trimmed_i})"
            # Remove extra '#' characters from the hyperlink
            hyperlink=${hyperlink//##/#}
            if [ "$first" -eq 1 ]; then
              replacement+="$hyperlink"
              first=0
            else
              replacement+=",$hyperlink"
            fi
        done
        prefix_length=$((${#line} - ${#pr_ids} - 3))
        echo "${line:0:${prefix_length}}($replacement)" >> "$TEMP_CHANGELOG"
    else
        echo "$line" >> "$TEMP_CHANGELOG"
    fi
done < "CHANGELOG.md"
mv "$TEMP_CHANGELOG" "CHANGELOG.md"

# Insert the line about the Splunk OpenTelemetry Collector version adopted in this Kubernetes release
appVersion=$(grep "appVersion:" helm-charts/splunk-otel-collector/Chart.yaml | awk '{print $2}')
insert_line="This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v${appVersion}](https://github.com/signalfx/splunk-otel-collector/releases/tag/v${appVersion}).\n"
awk -v n=9 -v s="$insert_line" 'NR == n {print s} {print}' CHANGELOG.md > $TEMP_CHANGELOG
mv "$TEMP_CHANGELOG" "CHANGELOG.md"

echo "Successfully updated PR links in CHANGELOG.md"
exit 0
