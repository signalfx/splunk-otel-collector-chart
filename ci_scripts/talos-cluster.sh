#!/usr/bin/env bash
# Copyright Splunk Inc.
# SPDX-License-Identifier: Apache-2.0

# Keep all Talos and Kubernetes credentials in an explicit test directory.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
: "${TALOS_TEST_DIR:?Set TALOS_TEST_DIR to an absolute directory for this disposable cluster}"
TALOS_CLUSTER_NAME=${TALOS_CLUSTER_NAME:-splunk-otel-talos-test}
TALOS_VERSION=${TALOS_VERSION:-v1.14.6}
if [[ "$TALOS_TEST_DIR" != /* || "$TALOS_TEST_DIR" == / || "$TALOS_TEST_DIR" == "$HOME" ]]; then
  echo 'TALOS_TEST_DIR must be an absolute directory dedicated to this test' >&2
  exit 1
fi
if [[ ! "$TALOS_CLUSTER_NAME" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
  echo 'TALOS_CLUSTER_NAME must contain lowercase letters, digits, and hyphens' >&2
  exit 1
fi
state_dir="$TALOS_TEST_DIR/state"
talosconfig="$TALOS_TEST_DIR/talosconfig"
kubeconfig="$TALOS_TEST_DIR/kubeconfig"
marker="$TALOS_TEST_DIR/cluster-name"
export TALOSCONFIG="$talosconfig" KUBECONFIG="$kubeconfig"

case "${1:-}" in
  create)
    if [[ -e "$marker" || -e "$talosconfig" || -e "$kubeconfig" ]]; then
      echo 'Test cluster files already exist; destroy that test cluster or use a new TALOS_TEST_DIR' >&2
      exit 1
    fi
    kubernetes_version=${K8S_VERSION:-$(jq -er '.functional_test_v2["k8s-kind-version"] | sort_by(ltrimstr("v") | split(".") | map(tonumber)) | last' "$repo_root/ci-matrix.json")}
    mkdir -p "$TALOS_TEST_DIR"
    chmod 700 "$TALOS_TEST_DIR"
    printf '%s\n' "$TALOS_CLUSTER_NAME" > "$marker"
    printf 'Creating Talos %s with Kubernetes %s\n' "$TALOS_VERSION" "$kubernetes_version"
    talosctl cluster create docker \
      --name "$TALOS_CLUSTER_NAME" --state "$state_dir" \
      --talosconfig-destination "$talosconfig" \
      --image "ghcr.io/siderolabs/talos:$TALOS_VERSION" \
      --kubernetes-version "${kubernetes_version#v}" \
      --workers 1 --memory-controlplanes 2GiB --memory-workers 2GiB
    # The Docker provisioner writes KUBECONFIG with its mapped API endpoint.
    if [[ ! -s "$kubeconfig" ]]; then
      echo "Talos did not write the isolated kubeconfig: $kubeconfig" >&2
      exit 1
    fi
    kubectl --kubeconfig "$kubeconfig" wait nodes --all --for=condition=Ready --timeout=5m
    # This namespace belongs to the disposable test cluster. The agent requires
    # host networking and host paths, which Talos' baseline policy rejects.
    kubectl --kubeconfig "$kubeconfig" label namespace default \
      pod-security.kubernetes.io/enforce=privileged --overwrite
    printf 'KUBECONFIG=%s\n' "$kubeconfig"
    ;;
  diagnostics)
    diagnostics_dir="$TALOS_TEST_DIR/diagnostics"
    mkdir -p "$diagnostics_dir"
    # Container logs remain available even when cluster creation failed before
    # the API server could write a kubeconfig.
    for node in controlplane-1 worker-1; do
      docker logs --tail 1000 "$TALOS_CLUSTER_NAME-$node" > "$diagnostics_dir/talos-$node.log" 2>&1 || true
    done
    if [[ -f "$kubeconfig" ]]; then
      kubectl --kubeconfig "$kubeconfig" get nodes,pods,daemonsets,deployments -A -o wide > "$diagnostics_dir/resources.txt" 2>&1 || true
      kubectl --kubeconfig "$kubeconfig" get events -A --sort-by=.metadata.creationTimestamp > "$diagnostics_dir/events.txt" 2>&1 || true
      kubectl --kubeconfig "$kubeconfig" describe pods -A > "$diagnostics_dir/pods.txt" 2>&1 || true
      while IFS= read -r pod; do
        [[ -n "$pod" ]] || continue
        kubectl --kubeconfig "$kubeconfig" logs -n default "$pod" --all-containers --tail=1000 > "$diagnostics_dir/$pod.log" 2>&1 || true
      done < <(kubectl --kubeconfig "$kubeconfig" get pods -n default -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null)
    fi
    ;;
  destroy)
    if [[ ! -f "$marker" || "$(cat "$marker")" != "$TALOS_CLUSTER_NAME" ]]; then
      echo 'Cluster marker does not match; refusing to destroy an unowned cluster' >&2
      exit 1
    fi
    talosctl cluster destroy --name "$TALOS_CLUSTER_NAME" --state "$state_dir"
    rm -f "$marker" "$talosconfig" "$kubeconfig"
    ;;
  *)
    echo "Usage: TALOS_TEST_DIR=/absolute/test/directory $0 {create|diagnostics|destroy}" >&2
    exit 1
    ;;
esac
