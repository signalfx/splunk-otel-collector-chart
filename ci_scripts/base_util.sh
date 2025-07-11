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
ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/../"
CHART_FILE_PATH="${ROOT_DIR}helm-charts/splunk-otel-collector/Chart.yaml"
VALUES_FILE_PATH="${ROOT_DIR}helm-charts/splunk-otel-collector/values.yaml"

# Set default OWNER to "signalfx" if not already set
: "${OWNER:=signalfx}"  # Sets OWNER to "signalfx" if it is not already set

# Helper function to interpret DEBUG_MODE boolean values as 1 or 0
interpret_debug_boolean() {
    case $1 in
        true|1) echo 1 ;;
        false|0) echo 0 ;;
        *) echo 0 ;;  # Default to false/0 for any other input
    esac
}

# DEBUG_MODE Configuration:
# By default, DEBUG_MODE is set to 0 (disabled). It can be enabled in two ways:
# 1. By passing the argument "--debug" when executing the script.
#    This will set DEBUG_MODE to 1 (enabled).
# 2. By setting the DEBUG_MODE environment variable before sourcing this script.
#    DEBUG_MODE can be set to either 'true' or 1 to enable debug mode,
#    or 'false' or 0 to disable it. If set to any other value, it defaults to 0 (disabled).
# When DEBUG_MODE is enabled (set to 'true' or 1), debug information will be displayed.
DEBUG_MODE=$(interpret_debug_boolean "$DEBUG_MODE")

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
            # Prints out the supplied file path and the content of the file
            echo "[DEBUG] $var_name: Content of file $var_value:"
            cat "$var_value"
        else
          if [[ -z "$var_value" ]]; then
              # Prints out the supplied string's value
              echo "[DEBUG] $var_name"
          else
              # Prints out the supplied variable name and value
              echo "[DEBUG] $var_name: $var_value"
          fi
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

# ---- CI/CD Methods ----
# Function: setup_git
# Description: Configures git so commits are published under the bot user.
# Usage: setup_git
setup_git() {
  git config --global user.name release-bot
  git config --global user.email ssg-srv-gh-o11y-gdi@splunk.com
  echo "set git config for release-bot (ssg-srv-gh-o11y-gdi@splunk.com)"
}

# Function: get_release_version
# Description: Removes the leading 'v' from a version string.
# Usage: get_release_version "v1.2.3"
get_release_version() {
  local release_tag="$1"
  echo "$release_tag" | cut -c2-
}

# Function: get_major_version
# Description: Extracts the major version number from a version string.
# Usage: get_major_version "v1.2.3"
get_major_version() {
  local release_tag="$1"
  get_release_version "$release_tag" | awk -F'.' '{print $1}'
}

# Function: get_minor_version
# Description: Extracts the minor version number from a version string.
# Usage: get_minor_version "v1.2.3"
get_minor_version() {
  local release_tag="$1"
  get_release_version "$release_tag" | awk -F'.' '{print $2}'
}

# Function: get_patch_version
# Description: Extracts the patch version number from a version string.
# Usage: get_patch_version "v1.2.3"
get_patch_version() {
  local release_tag="$1"
  get_release_version "$release_tag" | awk -F'.' '{print $3}'
}

# Function: validate_version
# Description: Validates that the provided version string follows the major.minor.patch format.
# Usage: validate_version "1.2.3"
validate_version() {
  local version="$1"
  if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
  then
    echo "Invalid release version: $version"
    echo "Release version must follow the pattern major.minor.patch, e.g. 1.2.3"
    exit 1
  fi
}

# Function: setup_branch
# Description: Checks if the specified branch exists and has open PRs. If it does and has no open PRs, it resets the branch to main. If it doesn't exist, it creates a new branch.
# Usage: setup_branch "release-1.2.3" "github.com/example/repo"
setup_branch() {
  local branch="$1"
  local repo_url="$2"

  # check if the branch exists
  if git ls-remote --exit-code --heads origin "$branch"; then
    # get number of open PRs for the branch
    pr_count="$( gh pr list --repo "$repo_url" --head "$branch" --state open --json id --jq length )"
    if [[ ! "$pr_count" =~ ^[0-9]+$ ]]; then
      echo "ERROR: Failed to get PRs for the $branch branch!" >&2
      echo "$pr_count" >&2
      exit 1
    fi
    if [[ "$pr_count" != "0" ]]; then
      echo ">>> The $branch branch exists and has $pr_count open PR(s)."
      echo ">>> Nothing to do."
      exit 0
    fi
    echo ">>> Resetting the $branch branch to main ..."
    git checkout "$branch"
    git reset --hard origin/main
  else
    git checkout -b "$branch"
  fi
}

# ---- Docker Methods ----
# These methods provide functions for managing docker images.
# To use this utility, source it in your script as shown in the example below:

# Function: get_current_repo
# Description: Extracts the current repository from a YAML file based on the provided yq query string.
#              This function supports both simple path queries and complex yq expressions.
# Usage: get_current_repo "path/to/yaml/file.yaml" ".path.to.image"
# Usage: get_current_repo "path/to/yaml/file.yaml" ".path.to.image.repository"
get_current_repo() {
    local yaml_file_path="$1"
    local yq_query_string="$2"
    local value_type
    local current_repo

    # Determine the type of the value at the given yq path
    if [[ "$yq_query_string" =~ ^select\(.*\) || "$yq_query_string" =~ ^\..* ]]; then
        # The path is a complex yq expression or a direct path starting with a dot
        value_type="$(yq eval-all "${yq_query_string} | type" "${yaml_file_path}")"
    else
        # The path is a direct path without a leading dot, so prepend one
        value_type="$(yq eval-all ".${yq_query_string} | type" "${yaml_file_path}")"
    fi

    # Parse the repository from the image reference based on the value type
    if [[ "$value_type" == *'str'* ]]; then
        # It's a string, assume it's a Docker image reference
        local docker_image_ref="$(yq eval "${yq_query_string}" "${yaml_file_path}")"
        current_repo="${docker_image_ref%:*}"
    elif [[ "$value_type" == *'map'* ]]; then
        # It's a map, assume it contains 'repository' and 'tag'
        current_repo="$(yq eval "${yq_query_string}.repository" "${yaml_file_path}")"
    else
        echo "Error: Unsupported type encountered: $value_type"
        exit 1
    fi

    echo "$current_repo"
}

# Function: get_current_tag
# Description: Extracts the current tag from a YAML file based on the provided yq query string.
#              This function supports both simple path queries and complex yq expressions.
# Usage:
#   get_current_tag "path/to/yaml/file.yaml" ".path.to.image"
#   get_current_tag "path/to/yaml/file.yaml" ".path.to.image.tag"
get_current_tag() {
    local yaml_file_path="$1"
    local yq_query_string="$2"
    local value_type
    local current_tag

    # Determine the type of the value at the given yq path
    if [[ "$yq_query_string" =~ ^select\(.*\) || "$yq_query_string" =~ ^\..* ]]; then
        # The path is a complex yq expression or a direct path starting with a dot
        value_type="$(yq eval-all "${yq_query_string} | type" "${yaml_file_path}")"
    else
        # The path is a direct path without a leading dot, so prepend one
        value_type="$(yq eval-all ".${yq_query_string} | type" "${yaml_file_path}")"
    fi

    # Parse the repository and tag from the image reference based on the value type
    if [[ "$value_type" == *'str'* ]]; then
        # It's a string, assume it's a Docker image reference
        local docker_image_ref="$(yq eval "${yq_query_string}" "${yaml_file_path}")"
        current_tag="${docker_image_ref##*:}"
    elif [[ "$value_type" == *'map'* ]]; then
        # It's a map, assume it contains 'repository' and 'tag'
        current_tag="$(yq eval "${yq_query_string}.tag" "${yaml_file_path}")"
    else
        echo "Error: Unsupported type encountered: $value_type"
        exit 1
    fi

    echo "$current_tag"
}

# Function: get_latest_tag
# Description: Retrieves the latest tag based on version priority from a Docker container registry.
#              It supports registries hosted at quay.io, ghcr.io, and Docker Hub. The function assumes
#              semantic versioning for tags. It prioritizes the most detailed semantic version
#              if multiple tags share the same image digest.
# Usage: get_latest_tag "quay.io/owner/repo"
get_latest_tag() {
    local repo_value="$1"
    local filter="$2"
    # Handle different container registries
    # For quay.io repositories
    if [[ $repo_value =~ ^quay\.io/(.+)/(.+) ]]; then
        local owner="${BASH_REMATCH[1]}"
        local repo_name="${BASH_REMATCH[2]}"
        local latest_api="https://quay.io/api/v1/repository/$owner/$repo_name/tag/?limit=1&onlyActiveTags=true"
        if [ -n "$filter" ]; then
            latest_api+="&filter_tag_name=$filter"
        fi
        local tag_name=$(curl -sL "$latest_api" | jq -r '.tags[0].name')
        if [ -z "$tag_name" ]; then
            echo "Error: No tag found or failed to fetch tag from quay.io" >&2
            return 1
        fi
        echo "$tag_name"
    # For ghcr.io repositories
    elif [[ $repo_value =~ ^ghcr\.io/([^/]+/[^/]+) ]]; then
        local full_repo_name="${BASH_REMATCH[1]}"
        local latest_api="https://api.github.com/repos/${full_repo_name}/tags"
        local tag_name=$(curl -sL -H 'Accept: application/vnd.github+json' "$latest_api" | jq -r '.[0].name')
        if [ -n "$filter" ]; then
            tag_name=$(curl -sL -H 'Accept: application/vnd.github+json' "$latest_api" | jq -r --arg filter "$filter" 'first(.[] | select(.name | startswith($filter))).name')
        fi
        if [ -z "$tag_name" ]; then
            echo "Error: No tag found or failed to fetch tag from ghcr.io" >&2
            return 1
        fi
        echo "$tag_name"
    # Default for Docker Hub repositories
    else
        if [ -n "$filter" ]; then
            # TODO support getting a specific tag from docker hub
            echo "Error: filters are not supported for docker hub yet"
            exit 1
        fi
       # Remove the 'docker.io/' prefix if present
       repo_value="${repo_value#docker.io/}"
       local tags_api="https://registry.hub.docker.com/v2/repositories/$repo_value/tags/?page_size=100"

       # Get the digest of the 'latest' tag if available
       local latest_digest=$(curl -sL "$tags_api" | jq -r '.results[] | select(.name == "latest").images[].digest' | head -1)

       # Define a variable for the most recent tag
       local most_recent_tag=""

       # If the 'latest' tag has a digest, find all tags with the same digest
       if [[ -n "$latest_digest" ]]; then
           # Retrieve all tags sharing the same digest
           local tags_with_same_digest=$(curl -sL "$tags_api" | jq -r --arg digest "$latest_digest" '.results[] | select(.images[].digest == $digest) | .name')

           # Filter tags to match semantic versioning and pick the most specific version
           most_recent_tag=$(echo "$tags_with_same_digest" | \
             grep -E '^[0-9]+(\.[0-9]+)?(\.[0-9]+)?$' | \
             sort -rV | head -1)
       fi

       # If no tag was found or 'latest' lacks a digest, retrieve the most recent semantic version
       if [ -z "$most_recent_tag" ]; then
           most_recent_tag=$(curl -sL "$tags_api" | jq -r '.results[] | .name' | \
             grep -E '^[0-9]+(\.[0-9]+)?(\.[0-9]+)?$' | \
             sort -rV | head -1)
       fi

       # Check and echo the most recent tag
       if [ -n "$most_recent_tag" ]; then
           echo "$most_recent_tag"
       else
           echo "Error: No tag found or failed to fetch tag from Docker Hub" >&2
           return 1
       fi
    fi
}

# Function: update_version
# Description: This function updates the image tag in a specified YAML file. It maintains the
# original formatting by only updating the specific line the docker tag exists on. This ensures
# accurate in-place updates without reformatting the entire file.
# Usage:
#   update_version ".path.to.image.tag" "path/to/yaml/file.yaml" "0.42.0"
#   update_version ".path.to.image" "path/to/yaml/file.yaml" "0.42.0"
#   update_version "select(.kind == \"yq_query\").image" "path/to/yaml/file.yaml" "0.42.0"
update_version() {
    local yq_query_string="$1" # Path to the image tag within the YAML file.
    local yaml_file_path="$2"  # The file path of the YAML file to be updated.
    local latest_tag="$3"      # The new tag to update the YAML file with.

    local current_tag=$(get_current_tag "$yaml_file_path" "$yq_query_string")
    echo "Updating tag from $current_tag to $latest_tag"

    # Temporary file for safely updating the YAML.
    local temp_file="/tmp/temp_file.yaml"

    # TODO: use yq to update the tag after adapting the yq formatting requirements in values.yaml
    # This is a workaround to avoid reformatting the entire YAML file.
    # We match the last segment of the yq query as the key, and the current value, then replace the tag part.
    local key_name=$(basename "$yq_query_string" | sed 's/\..*//')
    if awk -v KEY="$key_name" -v CURRENT_TAG="$current_tag" -v LATEST_TAG="$latest_tag" '
        {
            # Match lines with the key (e.g., image: or tag:) and the current tag value
            key_regex = "[[:space:]]*" KEY ":"
            if ($0 ~ key_regex && $0 ~ CURRENT_TAG) {
                print "[DEBUG] Updating line: " $0 > "/dev/stderr"
                line = $0
                last_colon = 0
                for (i = length(line); i > 0; i--) {
                    if (substr(line, i, 1) == ":") { last_colon = i; break; }
                }
                if (last_colon > 0) {
                    if (KEY == "tag" && match(line, /^[[:space:]]*tag:[ ]*[^ ]/)) {
                        # Find how many spaces after colon
                        space_start = last_colon + 1
                        space_count = 0
                        while (substr(line, space_start + space_count, 1) == " ") { space_count++ }
                        new_line = substr(line, 1, last_colon) substr(line, last_colon+1, space_count) LATEST_TAG
                    } else {
                        new_line = substr(line, 1, last_colon) LATEST_TAG
                    }
                    print new_line
                } else {
                    print line
                }
                next
            }
            print
        }
    ' "$yaml_file_path" > "$temp_file"; then
        mv "$temp_file" "$yaml_file_path"
        echo "show git diff"
        git diff "$yaml_file_path"
        echo "Tag updated to $latest_tag"
    else
        echo "Error updating the tag."
        exit 1
    fi

    # Verify the update to ensure the correct tag has been set.
    local updated_tag=$(get_current_tag "$yaml_file_path" "$yq_query_string")
    if [ "$updated_tag" == "$latest_tag" ]; then
        echo "Verification successful: Tag is now updated to $latest_tag."
    else
        echo "Verification failed: Tag did not update correctly. Current tag is $updated_tag."
        exit 1
    fi
}

# Function: maybe_update_version
# Description: Checks if the current image tag in the YAML file needs to be updated to the latest version.
#              If so, updates the file with the latest tag or the first image that matches the prefix filter.
# Usage: maybe_update_version "path/to/yaml/file.yaml" ".path.to.image.tag" "v1."
maybe_update_version() {
    local yaml_file_path="$1"
    local yq_query_string="$2"
    local filter="$3"

    echo "Checking for image tag updates in '$yaml_file_path' based on query '$yq_query_string'"

    # Fetch the current and latest tag using the helper functions
    local current_tag=$(get_current_tag "$yaml_file_path" "$yq_query_string")
    echo "Current tag found: $current_tag"

    local image_repository=$(get_current_repo "$yaml_file_path" "$yq_query_string")
    echo "Image repository identified: $image_repository"

    local latest_tag=$(get_latest_tag "$image_repository" "$filter")
    echo "Latest tag available: $latest_tag"

    # Check if we need to update the current tag value with the latest tag value
    if [[ -z "$latest_tag" ]]; then
        echo "Unable to retrieve the latest tag at this time."
        echo "On rare occasions, the latest tag lookup request times out."
    elif [[ "$latest_tag" == "$current_tag" ]]; then
        echo "No update required. Current tag ($current_tag) is the latest version."
    else
        # The current tag is different and needs to be updated
        setd "NEED_UPDATE" 1
        setd "CURRENT_TAG" "$current_tag"
        setd "LATEST_TAG" "$latest_tag"
        # Emit the NEED_UPDATE variable to either GitHub output (or stdout) to notify possible
        # downstream CI/CD tasks about needed info
        emit_output "NEED_UPDATE"
        emit_output "CURRENT_TAG"
        emit_output "LATEST_TAG"
        update_version "$yq_query_string" "$yaml_file_path" "$latest_tag"
        echo "Update complete. Tag changed to $latest_tag."
    fi
    echo "Image update process completed successfully for '$yaml_file_path'."
}

# Function: redact_sensitive_info
# Description: Redacts sensitive information from a given input string and returns the redacted content as a string.
#              The function uses `awk` to redact specific patterns such as certificates, sensitive data, tokens, and passwords.
# Usage: redact_sensitive_info "$input_string"
redact_sensitive_info() {
    local input="$1"

    # Redact sensitive information from the input string using awk and return the result
    echo "$input" | awk '
    # Redact certificate sections
    /BEGIN CERTIFICATE/,/END CERTIFICATE/ {
        if (/BEGIN CERTIFICATE/) print;
        else if (/END CERTIFICATE/) print;
        else print "    [CERTIFICATE REDACTED]";
        next;
    }
    # Redact sensitive data patterns like caBundle, certificates, keys
    /caBundle|ca\.crt|client\.crt|client\.key|tls\.crt|tls\.key/ {
        print "    [SENSITIVE DATA REDACTED]";
        next;
    }
    # Redact tokens
    /[Tt][Oo][Kk][Ee][Nn]/ {
        print "    [TOKEN REDACTED]";
        next;
    }
    # Redact passwords
    /[Pp][Aa][Ss][Ss][Ww][Oo][Rr][Dd]/ {
        print "    [PASSWORD REDACTED]";
        next;
    }
    # Print other content unchanged
    {print}
    '
}

# Function: redact_files
# Description: Redacts sensitive information from all files matching the provided file pattern in the specified directory using a for loop.
# Usage: redact_files "path/to/directory" "*.yaml"
redact_files() {
    local dir="$1"
    local file_pattern="$2"

    # Use find to search for files matching the pattern in the specified directory
    for file in $(find "$dir" -type f -name "$file_pattern"); do
        # Redact the content of the file and save it back to the original file
        redacted_content=$(redact_sensitive_info "$(cat "$file")")
        echo "$redacted_content" > "$file"
    done
}
