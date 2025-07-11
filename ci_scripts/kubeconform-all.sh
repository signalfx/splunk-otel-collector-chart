#!/bin/bash
# kubeconform-all.sh
# Runs kubeconform on all rendered manifests in examples, using both default and operator CRDs.
# Usage: ./ci_scripts/kubeconform-all.sh [k8s_version]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$SCRIPT_DIR/.."
K8S_VERSION="${1:-1.33.0}"
EXAMPLES_DIR="$REPO_ROOT/examples"
SCHEMA_DIR="$REPO_ROOT/generated-crd-schemas"

# Find all rendered_manifests directories, excluding distribution-openshift
MANIFEST_DIRS=$(find "$EXAMPLES_DIR" -type d -name "rendered_manifests" ! -path "*/distribution-openshift/*")

if [ -z "$MANIFEST_DIRS" ]; then
  echo "No rendered_manifests directories found to validate."
  exit 1
fi

# Validate all found manifest dirs
if ! kubeconform -strict -schema-location default -schema-location "$SCHEMA_DIR/{{ .Group }}_{{ .ResourceKind }}_{{ .ResourceAPIVersion }}.json" -output pretty -verbose -kubernetes-version "$K8S_VERSION" $MANIFEST_DIRS; then
  echo "kubeconform version: $(kubeconform -v)"
  echo "Failed validating one or more manifest directories."
  exit 1
fi

echo "All manifests validated successfully."
