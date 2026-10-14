#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
version=$(bash "$SCRIPT_DIR/helm-version.sh")
install_dir="${LOCALBIN:-$SCRIPT_DIR/../bin}"
mkdir -p "$install_dir"
install_dir="$(cd "$install_dir" && pwd)"

if [[ -x "$install_dir/helm" ]] &&
   [[ "$("$install_dir/helm" version --template='{{ .Version }}')" == "$version" ]]; then
  echo "Helm $version is already installed in $install_dir."
  exit 0
fi

export PATH="$install_dir:$PATH"
curl -fsSL "https://raw.githubusercontent.com/helm/helm/${version}/scripts/get-helm-4" |
  HELM_INSTALL_DIR="$install_dir" bash -s -- --version "$version" --no-sudo

installed_version=$("$install_dir/helm" version --template='{{ .Version }}')
if [[ "$installed_version" != "$version" ]]; then
  echo "Expected Helm $version in $install_dir, got: $installed_version" >&2
  exit 1
fi
