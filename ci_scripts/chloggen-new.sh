#!/bin/bash
# Purpose: Automates the creation or update of changelog entries based on input parameters.
# Notes:
#   - Should be executed via the `make chlog-new` command.
#   - Facilitates the automation of changelog entry creation.
#   - Intended to be used with the `make chlog-new` command.
#
# Optional Parameters:
#   - CHANGE_TYPE: Type of change (e.g., 'enhancement', 'bug_fix')
#   - COMPONENT: Component affected by the change (e.g., 'operator')
#   - NOTE: Brief description of the change
#   - ISSUES: List of related issues or PRs
#   - SUBCONTEXT: Additional information for the changelog entry
#   - FILENAME: Name of the file to create or update, defaults to git branch name
#   - DEBUG_MODE: Can be enabled for debugging

# Include the base utility functions for setting and debugging variables
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/base_util.sh"

# ---- Initialize Variables ----
if [[ -z "$FILENAME" ]]; then
  setd "FILENAME" $(git branch --show-current | tr -d '[:space:][:punct:]')
fi

# ---- Changelog Entry Validation ----
# Check for the existence of TEMPLATE.yaml
if [ ! -f ".chloggen/TEMPLATE.yaml" ]; then
  echo "Error: .chloggen/TEMPLATE.yaml not found. Ensure it exists."
  exit 1
fi

# ---- Changelog Entry Update ----
# Check if a changelog entry with the given filename already exists
if [ -f ".chloggen/${FILENAME}.yaml" ]; then
  echo "Changelog entry ${FILENAME}.yaml already exists. Updating."
else
  echo "Creating new changelog entry ${FILENAME}.yaml."
  cp .chloggen/TEMPLATE.yaml .chloggen/${FILENAME}.yaml
fi

# Update fields only if the argument was passed
[[ ! -z "$CHANGE_TYPE" ]] && yq eval -i ".change_type = \"$CHANGE_TYPE\"" .chloggen/${FILENAME}.yaml
[[ ! -z "$COMPONENT" ]] && yq eval -i ".component = \"$COMPONENT\"" .chloggen/${FILENAME}.yaml
[[ ! -z "$NOTE" ]] && yq eval -i ".note = \"$NOTE\"" .chloggen/${FILENAME}.yaml
[[ ! -z "$SUBTEXT" ]] && yq eval -i ".subtext = \"$SUBTEXT\"" .chloggen/${FILENAME}.yaml
# Extend the list of issue IDs rather than overwriting them
if [ ! -z "$ISSUES" ]; then
  # Extend the .issues field and update it
  setd "OLD_ISSUES" "$(yq eval '.issues' ".chloggen/${FILENAME}.yaml")"
  # Combine the old and new issues and deduplicate them
  setd "NEW_ISSUES" "'$(echo "$OLD_ISSUES" "$ISSUES" | jq -sc 'add | unique')'"
  echo "Resulting issues: $NEW_ISSUES"
  yq eval -i ".issues = ${NEW_ISSUES//\'/} | .issues style=\"flow\" " ".chloggen/${FILENAME}.yaml"
fi


echo "${FILENAME}.yaml has been created or updated."
exit 0
