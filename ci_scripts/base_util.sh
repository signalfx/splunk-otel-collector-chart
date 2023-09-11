#!/bin/bash
# Base Utility Functions Library For CI/CD
# This script provides a set of utility functions for debugging, variable setting,
# and common CI/CD operations. It's designed to be sourced by other scripts to
# provide a standardized way of setting variables, debugging, and handling common
# tasks like fetching Helm chart resources.

# Note: This utility sets "set -e", which will cause any script that sources it
# to exit if any command fails. Make sure your script is compatible with this behavior.
set -e

# Paths for the Helm chart resources
CHART_FILE_PATH="$SCRIPT_DIR/../helm-charts/splunk-otel-collector/Chart.yaml"
VALUES_FILE_PATH="$SCRIPT_DIR/../helm-charts/splunk-otel-collector/values.yaml"

# Set default OWNER to "signalfx" if not already set
: "${OWNER:=signalfx}"  # Sets OWNER to "signalfx" if it is not already set

# Debug mode is off by default but can be enabled with --debug
: "${DEBUG_MODE:=0}"  # Sets DEBUG_MODE to 0 if it is not already set

# Iterate over all arguments of the calling script
for arg in "$@"; do
    if [[ "$arg" == "--debug" ]]; then
        DEBUG_MODE=1  # Enable debug mode
        # Remove --debug from arguments
        for index in "${!@}"; do
            if [[ "${!index}" == "--debug" ]]; then
                unset "$index"
                break
            fi
        done
        # Re-index the arguments array
        set -- "${@}"
    fi
done

# ---- Debug Methods ----
# These methods provide functions for setting and debugging variables.
# To use this utility, source it in your script as shown in the example below:
#
# Example:
# ```bash
# #!/bin/bash
# # Source the utility script to get access to its functions and variables
# source /path/to/base_util.sh
#
# # Now you can use the utility functions and variables in this script
# DEBUG_MODE=1  # Turn on debug mode
# setd "my_var" "Hello, World!"
# debug "a string value"
# debug "$TEMP_FILE_WITH_CONTENT_PATH"
# ```

# Function: setd
# Description: Sets a variable and outputs a debug message.
# Usage: setd "variable_name" "value"
setd() {
    eval "$1=\"$2\""  # Set a variable with the given name and value
    debug "$1"        # Call the debug function to output the variable
}

# Function: debug
# Description: Outputs debug information based on the DEBUG_MODE setting.
# Supports variables, strings, and file paths for file content.
# Usage: debug "variable_name"
debug() {
    if [[ $DEBUG_MODE -eq 1 ]]; then
        local var_name="$1"
        local var_value="${!var_name}"  # Indirect reference to get the value
        if [[ -f "$var_value" ]]; then
            echo "[DEBUG] $var_name: Content of file $var_value:"
            cat "$var_value"
        else
            echo "[DEBUG] $var_name: $var_value"
        fi
    fi
}

# Function: emit_output
# Description: Outputs a given environment variable either to GitHub output or stdout.
# Usage: emit_output "VAR_NAME"
emit_output() {
    local var_name="$1"
    local var_value="${!var_name}"  # Indirect reference to get the value

    if [ -n "$GITHUB_OUTPUT" ]; then
        echo "${var_name}=${var_value}" >> "$GITHUB_OUTPUT"
    else
        echo "${var_name}=${var_value}"
    fi
}
