# Functional tests

`SUITE` selects a directory under `functional_tests`. Within `SUITE=functional`,
`Test_Functions` uses `KUBE_TEST_ENV` to select kind or hosted cluster coverage;
both share the chart install, upgrade, and cleanup code.

## Navigating the functional suite

| File | Responsibility |
| --- | --- |
| [functional_test.go](functional/functional_test.go) | Entry point, shared lifecycle, environment names, and constants |
| [setup_test.go](functional/setup_test.go) | Chart installation and environment-specific values |
| [kind_test.go](functional/kind_test.go) | Local sinks and kind telemetry assertions |
| [hosted_test.go](functional/hosted_test.go) | Cloud resource attributes and component checks |
| [resources_test.go](functional/resources_test.go) | Kind application, log, and storage fixtures; cleanup of legacy fixtures |
| [upgrade_test.go](functional/upgrade_test.go) | Target allocator upgrade checks |
| [health_test.go](functional/health_test.go) | Shared control plane receiver health checks |

Auto-instrumentation, Prometheus, and log attribute helpers remain in
`auto_instrumentation_test.go`, `prometheus_test.go`, and `log_attributes_test.go`.
Expected telemetry and environment-specific chart values are under
`functional/testdata`.

## Setup

Kind v0.32.0 or newer is required for the Kubernetes 1.36 node image and
for loading locally built images into that cluster.

### Option 1: Make cmd

Use make command that handles cluster creation, testing, and cleanup:

```bash
# Run functional tests with automatic cluster management
# This will create the cluster, build/load test images, run tests, and clean up
make functionaltest-local

# Run specific test suite
make functionaltest-local SUITE=histogram
```

### Option 2: Manual Cluster Management

Create and manage the kind cluster:

```bash
# Create kind cluster (includes TLS cert approval and helm dependencies)
make kind-setup

# Build and load the application test images
make kind-build-test-images

# Run functional tests
KUBECONFIG=/tmp/kube-config-splunk-otel-collector-chart-functional-testing \
  KUBE_TEST_ENV=kind make functionaltest SUITE=functional

# Clean up when done
make kind-delete
```

### Option 3: Raw Commands

If you prefer to run commands directly:

```bash
# Set up environment variables
export KUBECONFIG=/tmp/kube-config-splunk-otel-collector-chart-functional-testing
export KUBE_TEST_ENV=kind
export K8S_VERSION=v1.36.1

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

# Build and load the application test images into Kind
make kind-build-test-images

# Run functional tests
make functionaltest SUITE=functional

# Clean up when done
kind delete cluster --name=kind
```

## Hosted cluster coverage

Hosted functional tests install or upgrade the chart and validate collector
resource attributes and component health. They do not deploy the locally built
application images or the kind logging, Prometheus, or storage fixtures. Full kind install runs
deploy those fixtures, including setup-only runs with `SKIP_TESTS=true`; kind
upgrade runs deploy the chart without application fixtures.

Use an existing authenticated kubeconfig for a hosted cluster:

```bash
make dep-update
KUBECONFIG=/path/to/eks-kubeconfig KUBE_TEST_ENV=eks HOST_ENDPOINT=0.0.0.0 \
  TEARDOWN_BEFORE_SETUP=true make functionaltest SUITE=functional
```

`HOST_ENDPOINT=0.0.0.0` is a placeholder for the sink endpoints in hosted test
values. Hosted assertions read metrics from file exporters in collector pods or
check component health; they do not consume telemetry in the runner's local sinks.
Hosted cleanup also removes named kind fixtures that older test runs may have
left behind.

Supported hosted environments are `eks`, `eks/auto-mode`, `eks/fargate`, `gke`,
`gke/autopilot`, `aks`, `gce`, and `rosa`. EKS Fargate currently runs with
`SKIP_TESTS=true` to validate chart install/upgrade readiness without telemetry
assertions. Other suite directories are currently focused on kind; selecting a
hosted `KUBE_TEST_ENV` does not make them cloud-ready.

## IPv6 control plane coverage

CI runs the existing `histogram` suite on the newest kind Kubernetes version with
IPv6; its other Kubernetes versions use IPv4. This keeps the matrix at 36 jobs.
The version updater moves that IPv6 selection when the newest kind version changes.

The suite tests receiver creator endpoints. On IPv6 clusters, its values override
the static health check listener and kubelet endpoint, which still require explicit
IPv6 configuration. Passing this suite does not establish IPv6 support for all chart
defaults. The overrides follow the Kubernetes Service's IP family;
`EXPECT_COREDNS_IPV6` separately verifies the CoreDNS pod addresses so an IPv6 CI job
cannot silently pass on an IPv4 cluster.

## Build Test Images for a Different Platform

`kind-build-test-images` builds the four application images used by the
functional, secureapp, and OBI Kind suites and loads them into the cluster.
It defaults to `linux/amd64`, matching GitHub-hosted Linux runners. Override
the platform for an ARM64 local Kind cluster, such as on an Apple Silicon Mac:

```bash
TEST_IMAGE_PLATFORM=linux/arm64 make kind-build-test-images

# Load auto-instrumentation images (get the latest version from values.yaml) -
# On Mac M1s, you can also push this image so kind doesn't get confused with the platform to use:
kind load docker-image ghcr.io/signalfx/splunk-otel-dotnet/splunk-otel-dotnet:v1.11.0 --name kind
kind load docker-image ghcr.io/signalfx/splunk-otel-js/splunk-otel-js:v3.3.0 --name kind
kind load docker-image ghcr.io/signalfx/splunk-otel-java/splunk-otel-java:v2.20.1 --name kind
kind load docker-image quay.io/signalfx/splunk-otel-instrumentation-python:v2.7.0 --name kind
```

## Config switches

When running tests you can use the following env vars to help with local development:

- `KUBECONFIG`: Path to the kubeconfig file for the test cluster.
- `KUBE_TEST_ENV`: Set the type of cluster (e.g., `kind`, `eks`, `gce`).
- `SKIP_SETUP`: Skip setting up the chart/apps (useful if already deployed).
- `SKIP_TEARDOWN`: Skip cleanup (useful to keep apps for local dev).
- `SKIP_TESTS`: Skip assertions; only set up and tear down the chart and applicable fixtures.
- `TEARDOWN_BEFORE_SETUP`: Clean up deployments before setting up.
- `SUITE`: Specify which test suite to run (e.g., `SUITE="functional"`).
- `EXPECT_COREDNS_IPV6`: Set to `true` to require IPv6 CoreDNS pod addresses in the control plane metrics test.
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
KUBE_TEST_ENV=kind K8S_VERSION=v1.36.1 make functionaltest SUITE=functional

# Update expected test results
UPDATE_EXPECTED_RESULTS=true make functionaltest SUITE=functional
```
