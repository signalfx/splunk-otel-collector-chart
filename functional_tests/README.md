# Functional tests

## Setup
Run the following commands prior to running the test locally:

```
export KUBECONFIG=/tmp/kube-config-splunk-otel-collector-chart-functional-testing
export KUBE_TEST_ENV=kind
export K8S_VERSION=v1.29.0
kind create cluster --kubeconfig=/tmp/kube-config-splunk-otel-collector-chart-functional-testing --config=.github/workflows/configs/kind-config.yaml --image=kindest/node:$K8S_VERSION
kubectl get csr -o=jsonpath='{range.items[?(@.spec.signerName=="kubernetes.io/kubelet-serving")]}{.metadata.name}{" "}{end}' | xargs kubectl certificate approve
make dep-update
kind load docker-image quay.io/splunko11ytest/nodejs_test:latest --name kind
kind load docker-image quay.io/splunko11ytest/java_test:latest --name kind
kind load docker-image quay.io/splunko11ytest/dotnet_test:latest --name kind
# On Mac M1s, you can also push this image so kind doesn't get confused with the platform to use:
kind load docker-image ghcr.io/signalfx/splunk-otel-dotnet/splunk-otel-dotnet:v1.8.0 --name kind
kind load docker-image ghcr.io/signalfx/splunk-otel-js/splunk-otel-js:v3.1.2 --name kind
kind load docker-image ghcr.io/signalfx/splunk-otel-java/splunk-otel-java:v1.30.0 --name kind
kind load docker-image ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-python:0.50b0 --name kind
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
    be used with the dispatch trigger and input `UPLOAD_UPDATED_EXPECTED_RESULTS=true` to generate new results and upload
    them as a github workflow run artifact.

## Run

From the root repository directory run `make functionaltest`.
