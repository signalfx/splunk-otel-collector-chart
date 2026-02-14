#!/bin/bash
# generate-crd-schemas.sh
# Generates JSON schemas from operator CRDs for kubeconform
# Note: We only generate schemas for CRDs we explicitly support (instrumentations CRD).
# Other CRDs from the opentelemetry-operator chart (opampbridges, targetallocators, opentelemetrycollectors)
# and cert-manager CRDs are not supported and will be skipped during validation using -ignore-missing-schemas.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$SCRIPT_DIR/.."
VENV_DIR="$REPO_ROOT/.kubeconform-venv"
SCHEMA_DIR="$REPO_ROOT/generated-crd-schemas"
CRDS_DIR="$REPO_ROOT/helm-charts/splunk-otel-collector/charts/opentelemetry-operator-crds/crds"
OPENAPI2JSONSCHEMA="$SCHEMA_DIR/openapi2jsonschema.py"

rm -rf "$SCHEMA_DIR"
mkdir -p "$SCHEMA_DIR"

python3 -m venv "$VENV_DIR"
source "$VENV_DIR/bin/activate"
pip install --quiet pyyaml

curl -s -L "https://raw.githubusercontent.com/yannh/kubeconform/v0.7.0/scripts/openapi2jsonschema.py" -o "$OPENAPI2JSONSCHEMA"

# Generate schemas from opentelemetry-operator-crds subchart (only contains instrumentations CRD)
if [ -d "$CRDS_DIR" ] && [ "$(ls -A "$CRDS_DIR"/*.yaml 2>/dev/null)" ]; then
  echo "Generating schemas from opentelemetry-operator-crds..."
  (cd "$SCHEMA_DIR" && for crd in "$CRDS_DIR"/*.yaml; do FILENAME_FORMAT="{fullgroup}_{kind}_{version}" python "openapi2jsonschema.py" "$crd"; done)
fi

# Cleanup
rm -f "$OPENAPI2JSONSCHEMA"
deactivate

echo "CRD schema generation complete. Schemas available in: $SCHEMA_DIR"
