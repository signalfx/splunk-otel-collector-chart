# Functional tests

## Setup

### Option 1: Make cmd

Use make command that handles cluster creation, testing, and cleanup:

```bash
# Run functional tests with automatic cluster management
# This will create cluster, run tests with golden file updates, and clean up
make functionaltest-local

# Run specific test suite
make functionaltest-local SUITE=histogram
```

### Option 2: Manual Cluster Management

Create and manage the kind cluster:

```bash
# Create kind cluster (includes TLS cert approval and helm dependencies)
make kind-setup

# Run functional tests
KUBECONFIG=/tmp/kube-config-splunk-otel-collector-chart-functional-testing make functionaltest SUITE=functional

# Clean up when done
make kind-delete
```

### Option 3: Raw Commands

If you prefer to run commands directly:

```bash
# Set up environment variables
export KUBECONFIG=/tmp/kube-config-splunk-otel-collector-chart-functional-testing
export KUBE_TEST_ENV=kind
export K8S_VERSION=v1.33.2

# Create kind cluster with functional test configuration
kind create cluster \
  --kubeconfig=/tmp/kube-config-splunk-otel-collector-chart-functional-testing \
  --config=.github/workflows/configs/kind-config.yaml \
  --image=kindest/node:$K8S_VERSION \
  --name=kind

# Approve TLS certificates
kubectl get csr -o=jsonpath='{range.items[?(@.spec.signerName=="kubernetes.io/kubelet-serving")]}{.metadata.name}{" "}{end}' | xargs kubectl certificate approve

# Update helm dependencies
make dep-update

# Run functional tests (Kubernetes will pull images automatically)
make functionaltest SUITE=functional

# Clean up when done
kind delete cluster --name=kind
```

## Optional: Pre-load Test Images

If you want to pre-load images for faster test execution:

```bash
# Load application test images
kind load docker-image quay.io/splunko11ytest/nodejs_test:latest --name kind
kind load docker-image quay.io/splunko11ytest/java_test:latest --name kind
kind load docker-image quay.io/splunko11ytest/dotnet_test:latest --name kind
kind load docker-image quay.io/splunko11ytest/python_test:latest --name kind

# Load auto-instrumentation images (get the latest version from values.yaml) -
# On Mac M1s, you can also push this image so kind doesn't get confused with the platform to use:
kind load docker-image ghcr.io/signalfx/splunk-otel-dotnet/splunk-otel-dotnet:v1.11.0 --name kind
kind load docker-image ghcr.io/signalfx/splunk-otel-js/splunk-otel-js:v3.3.0 --name kind
kind load docker-image ghcr.io/signalfx/splunk-otel-java/splunk-otel-java:v2.19.0 --name kind
kind load docker-image quay.io/signalfx/splunk-otel-instrumentation-python:v2.7.0 --name kind
```

## Config switches

When running tests you can use the following env vars to help with local development:
- `KUBECONFIG`: Path to the kubeconfig file for the test cluster.
- `KUBE_TEST_ENV`: Set the type of cluster (e.g., `kind`, `eks`, `gce`).
- `SKIP_SETUP`: Skip setting up the chart/apps (useful if already deployed).
- `SKIP_TEARDOWN`: Skip cleanup (useful to keep apps for local dev).
- `SKIP_TESTS`: Skip tests; only set up and tear down the cluster.
- `TEARDOWN_BEFORE_SETUP`: Clean up deployments before setting up.
- `SUITE`: Specify which test suite to run (e.g., `SUITE="functional"`).
- `UPDATE_EXPECTED_RESULTS`: Generate new golden files (expected test results) for the functional tests.
  - The https://github.com/signalfx/splunk-otel-collector-chart/actions/workflows/functional_test_v2.yaml workflow can
    be used with the dispatch trigger and input `UPDATE_EXPECTED_RESULTS=true` to generate new results and upload
    them as a github workflow run artifact.

## Run Tests

```bash
# Run all functional tests (make sure KUBECONFIG is set)
make functionaltest

# Run specific test suite
make functionaltest SUITE=histogram

# Run with specific environment variables
KUBE_TEST_ENV=kind K8S_VERSION=v1.33.2 make functionaltest SUITE=functional

# Update golden files (expected test results)
UPDATE_EXPECTED_RESULTS=true make functionaltest SUITE=functional
```
