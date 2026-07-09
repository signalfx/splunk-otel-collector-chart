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
SCHEMA_CACHE_DIR="${KUBECONFORM_SCHEMA_CACHE:-${RUNNER_TEMP:-/tmp}/kubeconform-schemas/$K8S_VERSION}"
KUBECONFORM_CONCURRENCY="${KUBECONFORM_CONCURRENCY:-1}"
KUBECONFORM_RETRIES="${KUBECONFORM_RETRIES:-3}"
KUBECONFORM_RETRY_DELAY="${KUBECONFORM_RETRY_DELAY:-10}"

# Find all rendered_manifests directories, excluding distribution-openshift
MANIFEST_DIRS=$(find "$EXAMPLES_DIR" -type d -name "rendered_manifests" ! -path "*/distribution-openshift/*")

if [ -z "$MANIFEST_DIRS" ]; then
  echo "No rendered_manifests directories found to validate."
  exit 1
fi

# Validate all found manifest dirs
# Note: -ignore-missing-schemas skips validation for unsupported CRDs.
# Ensure the Instrumentation CRD schema exists so validation is not silently skipped.
if ! compgen -G "$SCHEMA_DIR/opentelemetry.io_instrumentation_*.json" > /dev/null; then
  echo "Error: Expected Instrumentation CRD schema not found in $SCHEMA_DIR."
  echo "Make sure Instrumentation schemas are generated before running kubeconform."
  exit 1
fi

mkdir -p "$SCHEMA_CACHE_DIR"

echo "Validating rendered manifests against Kubernetes $K8S_VERSION."
echo "Using kubeconform schema cache: $SCHEMA_CACHE_DIR"

KUBECONFORM_ARGS=(
  -strict
  -ignore-missing-schemas
  -cache "$SCHEMA_CACHE_DIR"
  -n "$KUBECONFORM_CONCURRENCY"
  -schema-location default
  -schema-location "$SCHEMA_DIR/{{ .Group }}_{{ .ResourceKind }}_{{ .ResourceAPIVersion }}.json"
  -output pretty
  -verbose
  -kubernetes-version "$K8S_VERSION"
)

attempt=1
until kubeconform "${KUBECONFORM_ARGS[@]}" $MANIFEST_DIRS; do
  if (( attempt >= KUBECONFORM_RETRIES )); then
    echo "kubeconform version: $(kubeconform -v)"
    echo "Failed validating one or more manifest directories."
    exit 1
  fi

  echo "kubeconform attempt $attempt failed. Retrying in ${KUBECONFORM_RETRY_DELAY}s..."
  sleep "$KUBECONFORM_RETRY_DELAY"
  attempt=$((attempt + 1))
done

echo "All manifests validated successfully."
