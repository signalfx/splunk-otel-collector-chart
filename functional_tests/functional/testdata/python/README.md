# Python test image

This image is used for testing the auto-instrumentation of Python application through the OpenTelemetry Operator.

This image is pushed to https://quay.io/repository/splunko11ytest/python_test.

The container performs two separate functions:
* It runs a Python HTTP server on port 8000 of the container host.
* It runs HTTP requests against the server every second.

Running this container inside a Kubernetes cluster under observation of the operator therefore creates traces.

## Develop

Login to quay.io and push with `make push`
