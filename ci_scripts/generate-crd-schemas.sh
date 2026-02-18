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

# kubeconform script at specific commit (matches tag v0.7.0) + checksum verification
OPENAPI2JSONSCHEMA_URL="https://raw.githubusercontent.com/yannh/kubeconform/e65429b1e5990dd019ebb7b5642dcd22a3e9cd13/scripts/openapi2jsonschema.py"
OPENAPI2JSONSCHEMA_SHA256="d145babfbb765004030764e1b4e518bfb7a4bd7f111691a08fa57983b81881f3"

rm -rf "$SCHEMA_DIR"
mkdir -p "$SCHEMA_DIR"

echo "Downloading openapi2jsonschema.py (pinned to commit e65429b1...)..."
curl -sSL "$OPENAPI2JSONSCHEMA_URL" -o "$OPENAPI2JSONSCHEMA"
actual_sha=$(openssl dgst -sha256 "$OPENAPI2JSONSCHEMA" | awk '{print $NF}')
if [ "$actual_sha" != "$OPENAPI2JSONSCHEMA_SHA256" ]; then
  echo "Error: openapi2jsonschema.py checksum mismatch (expected $OPENAPI2JSONSCHEMA_SHA256, got $actual_sha)"
  exit 1
fi

python3 -m venv "$VENV_DIR"
source "$VENV_DIR/bin/activate"
pip install --quiet pyyaml

# Generate schemas from opentelemetry-operator-crds subchart (only contains instrumentations CRD)
if [ -d "$CRDS_DIR" ] && compgen -G "$CRDS_DIR"/*.yaml > /dev/null; then
  echo "Generating schemas from opentelemetry-operator-crds..."
  (cd "$SCHEMA_DIR" && for crd in "$CRDS_DIR"/*.yaml; do FILENAME_FORMAT="{fullgroup}_{kind}_{version}" python "openapi2jsonschema.py" "$crd"; done)
fi

rm -f "$OPENAPI2JSONSCHEMA"
deactivate

echo "CRD schema generation complete. Schemas available in: $SCHEMA_DIR"
