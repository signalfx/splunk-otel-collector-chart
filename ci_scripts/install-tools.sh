#!/bin/bash
# Purpose: Installs or upgrades essential development tools.
# Notes:
#   - Supports macOS and Linux (best effort) for installations via `brew install` and `go install`.
#   - Installs tools like kubectl, helm, pre-commit, go, and chloggen.
#   - Use OVERRIDE_OS_CHECK=true to bypass OS compatibility checks.
#   - This script is intended to be run via `make install-tools`.
#   - Prompts the user for approval to install or update each tool if the tool is out of date.
#
# Example Usage:
#    make install-tools [OVERRIDE_OS_CHECK=true]

# Function to install or upgrade a tool
install_or_upgrade() {
  local tool=$1
  local type=$2

  case $type in
    brew)
      install_or_upgrade_brew "$tool"
      ;;
    go)
      install_or_upgrade_go "$tool"
      ;;
    *)
      echo "Unsupported tool type: $type"
      exit 1
      ;;
  esac
}

# Function to install or upgrade a Homebrew-based tool
install_or_upgrade_brew() {
  local tool=$1
  local installed_version=$(brew list $tool --versions | awk '{print $2}')
  local latest_version=$(brew info --json=v1 "$tool" | jq -r '.[0].versions.stable')

  if [ "$installed_version" == "$latest_version" ]; then
    echo "$tool is already up to date (version $installed_version)."
    return
  fi

  read -p "$tool version $installed_version is installed. Would you like to upgrade to $latest_version? (y/n): " yn
  case $yn in
    [Yy]* )
      brew upgrade $tool || echo "Failed to upgrade $tool. Continuing..."
      ;;
    [Nn]* )
      echo "Skipping upgrade for $tool."
      ;;
  esac
}

# Function to install or upgrade a Go-based tool
install_or_upgrade_go() {
  local tool=$1
  local tool_path="$LOCALBIN/$(basename $tool)"
  local installed_version

  if [ -f "$tool_path" ]; then
    installed_version=$($tool --version 2>/dev/null)  # Try to get the version
    if [ -z "$installed_version" ]; then  # If version is empty
      # Fallback to file modification time
      installed_version="UNKNOWN (Last updated: $(stat -c %y "$tool_path" 2>/dev/null || stat -f "%Sm" "$tool_path"))"
    fi
  else
    installed_version="Not Installed"
  fi

  read -p "$tool version $installed_version is installed. Would you like to upgrade? (y/n): " yn
  case $yn in
    [Yy]* )
      GOBIN=$LOCALBIN go install ${tool}@latest || echo "Failed to upgrade $tool. Continuing..."
      ;;
    [Nn]* )
      echo "Skipping upgrade for $tool."
      ;;
    * )
      echo "Please answer yes or no."
      exit 1
      ;;
  esac
}

# Install or upgrade brew-based tools
for tool in kubectl helm pre-commit go; do
  install_or_upgrade "$tool" brew
done

# Install or upgrade Go-based tools
install_or_upgrade "go.opentelemetry.io/build-tools/chloggen" go

echo "Tool installation and upgrade process completed successfully!"
exit 0
