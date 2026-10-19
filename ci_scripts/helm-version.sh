#!/usr/bin/env bash
set -euo pipefail

# Dependabot maintains this dependency, keeping the Helm CLI and Go SDK in sync.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
version=$(awk '$1 == "helm.sh/helm/v4" { print $2 }' "$SCRIPT_DIR/../functional_tests/go.mod")

if [[ ! "$version" =~ ^v4\.[0-9]+\.[0-9]+$ ]]; then
  echo "Expected one stable Helm 4 version in functional_tests/go.mod, got: $version" >&2
  exit 1
fi

printf '%s\n' "$version"
