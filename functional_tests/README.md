# Functional tests

## Setup
Run the following commands prior to running the test locally:

```
export KUBECONFIG=/tmp/kube-config-splunk-otel-collector-chart-functional-testing
export KUBE_TEST_ENV=kind
export K8S_VERSION=v1.29.0
kind create cluster --kubeconfig=/tmp/kube-config-splunk-otel-collector-chart-functional-testing --config=.github/workflows/configs/kind-config.yaml --image=kindest/node:$K8S_VERSION
kubectl get csr -o=jsonpath='{range.items[?(@.spec.signerName=="kubernetes.io/kubelet-serving")]}{.metadata.name}{" "}{end}' | xargs kubectl certificate approve
make cert-manager
kind load docker-image quay.io/splunko11ytest/nodejs_test:latest --name kind
kind load docker-image quay.io/splunko11ytest/java_test:latest --name kind
kind load docker-image quay.io/splunko11ytest/dotnet_test:latest --name kind
# On Mac M1s, you can also push this image so kind doesn't get confused with the platform to use:
kind load docker-image ghcr.io/signalfx/splunk-otel-dotnet/splunk-otel-dotnet:v1.8.0 --name kind
kind load docker-image ghcr.io/signalfx/splunk-otel-js/splunk-otel-js:v2.4.4 --name kind
kind load docker-image ghcr.io/signalfx/splunk-otel-java/splunk-otel-java:v1.30.0 --name kind
```

## Config switches

When running tests you can use the following env vars to help with local development:
- `KUBECONFIG`: Path to the kubeconfig file for the test cluster.
- `KUBE_TEST_ENV`: Set the type of cluster (e.g., `kind`, `eks`, `gce`).
- `SKIP_SETUP`: Skip setting up the chart/apps (useful if already deployed).
- `SKIP_TEARDOWN`: Skip cleanup (useful to keep apps for local dev).
- `SKIP_TESTS`: Skip tests; only set up and tear down the cluster.
- `TEARDOWN_BEFORE_SETUP`: Clean up deployments before setting up.
- `TAGS`: Specify which tests to run (e.g., `TAGS="functional"`).
- `UPDATE_EXPECTED_RESULTS`: Generate new golden files for test results.

## Run

From the root repository directory run `make functionaltest`.
