#!/bin/bash

# =====================================================================
#  Dev-only: This script is for local development only.
#  It runs the histogram functional tests against multiple k8s
#  versions using kind, matching the test matrix in ci-matrix.json.
# =====================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$SCRIPT_DIR/../.."
cd "$REPO_ROOT"

echo "Loading Kubernetes versions from ci-matrix.json..."
k8s_versions=( $(jq -r '.functional_test_v2["k8s-kind-version"][]' ci-matrix.json) )
echo "Kubernetes versions: ${k8s_versions[*]}"

# Number of times to run each test (default: 1, override with RUNS)
RUNS="${RUNS:-1}"

# Set to true to get expected output files for histogram tests (default: false)
GENERATE_EXPECTED="${GENERATE_EXPECTED:-false}"

run_histogram_test() {
  local k8s_version=$1
  local pass_count=0
  local fail_count=0

  for i in $(seq 1 $RUNS); do
    echo "Setting KUBECONFIG environment variable"
    export KUBECONFIG=/tmp/kube-config-splunk-otel-collector-chart-functional-testing

    echo "Creating kind cluster with Kubernetes version $k8s_version"
    kind create cluster --name kind --image kindest/node:$k8s_version --config .github/workflows/configs/kind-config.yaml

    echo "Approving kubelet TLS server certificates"
    kubectl get csr -o=jsonpath='{range.items[?(@.spec.signerName=="kubernetes.io/kubelet-serving")]}{.metadata.name}{" "}{end}' | xargs kubectl certificate approve || true

    echo "Updating dependencies"
    make dep-update

    sleep 30

    echo "Running histogram test (Attempt $i)"
    if GENERATE_EXPECTED="$GENERATE_EXPECTED" K8S_VERSION="$k8s_version" KUBECONFIG="$KUBECONFIG" SKIP_TEARDOWN=false SUITE=histogram make functionaltest; then
      pass_count=$((pass_count + 1))
    else
      fail_count=$((fail_count + 1))
    fi

    echo "Deleting kind cluster"
    kind delete cluster --name kind

    sleep 30
  done

  echo "Kubernetes version $k8s_version: $pass_count passed, $fail_count failed"
}

# Loop through each k8s version and run the histogram test
for k8s_version in "${k8s_versions[@]}"; do
  run_histogram_test $k8s_version
done
