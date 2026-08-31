# Node.js test image

This image is used for testing the auto-instrumentation of Node.js application through the OpenTelemetry Operator.

This image is built locally by the chart repository's Kind functional-test
setup.

The container performs two separate functions:
* It runs a Node.js HTTP server on port 3000 of the container host.
* It runs HTTP requests against the server every second.

Running this container inside a Kubernetes cluster under observation of the operator therefore creates traces.

## Develop

Build this image directly with `make -C functional_tests/functional/testdata/nodejs build`.
Set `PLATFORM=linux/arm64` when building for an ARM64 local Kind cluster.
