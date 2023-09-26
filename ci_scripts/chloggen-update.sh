#!/bin/bash
# Purpose: Updates CHANGELOG.md file for a release.
# Notes:
#   - Should be executed via the `make chlog-update` command.
#   - Intended to be used as part of the release process.
#   - Automates the process of hyperlinking PR/issue IDs in the CHANGELOG.md.

# Include the base utility functions for setting and debugging variables
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/base_util.sh"

# ---- Initialize Variables ----
# Create a temporary file to hold the updated CHANGELOG.md content
setd "TEMP_CHANGELOG_PATH" "CHANGELOG.md.tmp"

# ---- Update CHANGELOG.md Content ----
# Convert static PR/issues references into hyperlinks
while IFS= read -r line; do
    if [[ $line =~ \(\#([0-9,# ]+)\)$ ]]; then
        pr_ids=${BASH_REMATCH[1]}
        setd "REPLACEMENT" ""
        setd "FIRST" "1"
        IFS=',' read -ra ADDR <<< "$pr_ids"
        for i in "${ADDR[@]}"; do
            setd "TRIMMED_I" $(echo "$i" | xargs)  # Remove leading/trailing whitespaces
            setd "HYPERLINK" "[#${TRIMMED_I}](https://github.com/${OWNER}/splunk-otel-collector-chart/pull/${TRIMMED_I})"
            # Remove extra '#' characters from the HYPERLINK
            setd "HYPERLINK" ${HYPERLINK//##/#}
            if [ "$FIRST" -eq 1 ]; then
              REPLACEMENT+="$HYPERLINK"
              FIRST=0
            else
              REPLACEMENT+=",$HYPERLINK"
            fi
        done
        setd "PREFIX_LENGTH" $((${#line} - ${#pr_ids} - 3))
        echo "${line:0:${PREFIX_LENGTH}}($REPLACEMENT)" >> "$TEMP_CHANGELOG_PATH"
    else
        echo "$line" >> "$TEMP_CHANGELOG_PATH"
    fi
done < "CHANGELOG.md"
mv "$TEMP_CHANGELOG_PATH" "CHANGELOG.md"

# Insert the subcontext line about the Splunk OpenTelemetry Collector version adopted in this release
setd "APP_VERSION" $(grep "appVersion:" $CHART_FILE_PATH | awk '{print $2}')
setd "INSERT_LINE" "This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v${APP_VERSION}](https://github.com/${OWNER}/splunk-otel-collector/releases/tag/v${APP_VERSION}).\n"
awk -v n=9 -v s="$INSERT_LINE" 'NR == n {print s} {print}' CHANGELOG.md > $TEMP_CHANGELOG_PATH
mv "$TEMP_CHANGELOG_PATH" "CHANGELOG.md"

echo "Successfully updated PR links in CHANGELOG.md"
exit 0
