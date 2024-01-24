#!/bin/bash
# Purpose: Installs or upgrades essential development tools.
# Notes:
#   - Should be executed via the `make install-tools` command.
#   - Supports macOS and Linux for installations via `brew install` and `go install`.
#   - Installs tools like kubectl, helm, pre-commit, go, and chloggen.

# Include the base utility functions for setting and debugging variables
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/base_util.sh"

# Function to install a tool
install() {
  local tool=$1
  local type=$2

  case $type in
    brew)
      install_brew "$tool"
      ;;
    go)
      install_go "$tool"
      ;;
    helm_plugin)
      install_helm_plugin "$tool"
      ;;
    *)
      echo "Unsupported tool type: $type"
      exit 1
      ;;
  esac
}

# Function to install a Homebrew-based tool
install_brew() {
  if ! command -v brew &> /dev/null
  then
      echo "Homebrew could not be found. Please install Homebrew and try again."
      return
  fi

  local tool=$1
  local installed_version=$(brew list $tool --versions | awk '{print $2}')
  local latest_version=$(brew info --json=v1 "$tool" | jq -r '.[0].versions.stable')

  if [ "$installed_version" == "$latest_version" ]; then
    echo "$tool is already up to date (version $installed_version)."
    return
  elif [ ! -z "$installed_version" ] && [ "$installed_version" != "$latest_version" ]; then
    echo "$tool $installed_version is installed. A new version $latest_version is available. Continuing for now..."
    return
  fi
  echo "$tool (version $latest_version) is not installed, installing now..."
  brew install $tool || echo "Failed to install $tool. Continuing..."
}

# Function to install a Go-based tool
install_go() {
  if ! command -v go &> /dev/null
  then
      echo "Go could not be found. Please install Go and try again."
      return
  fi

  local tool=$1
  local tool_path="$LOCALBIN/$(basename $tool)"

  if [ -f "$tool_path" ]; then
    local installed_version=$($tool --version 2>/dev/null)  # Try to get the version
    if [ -z "$installed_version" ]; then  # If version is empty
      # Fallback to file modification time
      installed_version="UNKNOWN (Last updated: $(stat -c %y "$tool_path" 2>/dev/null || stat -f "%Sm" "$tool_path"))"
    fi
    echo "$tool is already installed (version: $installed_version). Continuing for now..."
  else
    echo "$tool is not installed, installing now..."
    GOBIN=$LOCALBIN go install ${tool}@latest || echo "Failed to install $tool. Continuing..."
  fi
}

# Function to install a helm plugin
install_helm_plugin() {
  if ! command -v helm &> /dev/null
  then
      echo "Helm could not be found. Please install Helm and try again."
      return
  fi
  local plugin="${1%%=*}"
  local repo="${1#*=}"
  local installed_version=$(helm plugin list | grep ${plugin} | awk '{print $2}')

  if [ -z "$installed_version" ]; then
    echo "Helm plugin $tool is not installed, installing now..."
    helm plugin install ${repo} || echo "Failed to install plugin ${plugin}. Continuing..."
  else
    echo "Helm plugin ${plugin} (version: $installed_version) already installed."
  fi
}

# install brew-based tools
for tool in kubectl helm chart-testing pre-commit go; do
  install "$tool" brew
done

# install helm plugin
install "unittest=https://github.com/helm-unittest/helm-unittest.git" helm_plugin

# install Go-based tools
install "go.opentelemetry.io/build-tools/chloggen" go

echo "Tool installation process completed!"
exit 0
