# Functional tests

## Setup
Run the following commands prior to running the test locally:

```
export KUBECONFIG=/tmp/kube-config-splunk-otel-collector-chart-functional-testing
export KUBE_TEST_ENV=kind
export K8S_VERSION=v1.28.0
kind create cluster --kubeconfig=/tmp/kube-config-splunk-otel-collector-chart-functional-testing --config=.github/workflows/configs/kind-config.yaml --image=kindest/node:$K8S_VERSION
kubectl get csr -o=jsonpath='{range.items[?(@.spec.signerName=="kubernetes.io/kubelet-serving")]}{.metadata.name}{" "}{end}' | xargs kubectl certificate approve
make cert-manager
kind load docker-image quay.io/splunko11ytest/nodejs_test:latest --name kind
kind load docker-image quay.io/splunko11ytest/java_test:latest --name kind
kind load docker-image quay.io/splunko11ytest/dotnet_test:latest --name kind
# On Mac M1s, you can also push this image so kind doesn't get confused with the platform to use:
kind load docker-image ghcr.io/signalfx/splunk-otel-dotnet/splunk-otel-dotnet:v1.6.0 --name kind
kind load docker-image ghcr.io/signalfx/splunk-otel-js/splunk-otel-js:v2.4.4 --name kind
kind load docker-image ghcr.io/signalfx/splunk-otel-java/splunk-otel-java:v1.30.0 --name kind
```

## Config switches

When running tests you can use the following env vars to help with local development:
- `SKIP_SETUP`: skip setting up the chart and apps. Useful if they are already deployed.
- `SKIP_TEARDOWN`: skip deleting the chart and apps as part of cleanup. Useful to keep around for local development.
- `SKIP_TESTS`: skip running tests, just set up and tear down the cluster.
- `TEARDOWN_BEFORE_SETUP`: delete all the deployments made by these tests before setting up.
- `UPDATE_EXPECTED_RESULTS`: run golden.WriteMetrics() methods to generate new golden files for expected test results
