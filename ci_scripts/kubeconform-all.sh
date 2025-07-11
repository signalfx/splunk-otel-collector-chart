#!/bin/bash
# kubeconform-all.sh
# Runs kubeconform on all rendered manifests in examples, using both default and operator CRDs.
# Usage: ./ci_scripts/kubeconform-all.sh [k8s_version]

set -euo pipefail

K8S_VERSION="${1:-1.33.0}"
EXAMPLES_DIR="examples"
CRDS_DIR="helm-charts/splunk-otel-collector/charts/opentelemetry-operator-crds/crds"
SCHEMA_DIR="generated-crd-schemas"

rm -rf "$SCHEMA_DIR"
mkdir -p "$SCHEMA_DIR"

# This script will be used to generate JSON schemas from operator CRDs
OPENAPI2JSONSCHEMA="$SCHEMA_DIR/openapi2jsonschema.py"
curl -s -L "https://raw.githubusercontent.com/yannh/kubeconform/master/scripts/openapi2jsonschema.py" -o "$OPENAPI2JSONSCHEMA"

# Create venv and install dependencies
VENV_DIR=".kubeconform-venv"
if [ ! -d "$VENV_DIR" ]; then
  python3 -m venv "$VENV_DIR"
fi
source "$VENV_DIR/bin/activate"
pip install --quiet pyyaml

# Generate JSON schemas from operator CRDs
(
  cd "$SCHEMA_DIR"
  for crd in "$OLDPWD/$CRDS_DIR"/*.yaml; do
    FILENAME_FORMAT="{fullgroup}_{kind}_{version}" python "openapi2jsonschema.py" "$crd"
  done
)

rm -f "$OPENAPI2JSONSCHEMA"

deactivate

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
