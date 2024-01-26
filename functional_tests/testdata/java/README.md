# Java test image

This image is used for testing the auto-instrumentation of Java application through the OpenTelemetry Operator.

This image is pushed to https://quay.io/repository/splunko11ytest/java_test.

The container performs two separate functions:
* It runs an Apache Tomcat server on port 8080 of the container host.
* It runs HTTP requests against the server every second.

Running this container inside a Kubernetes cluster under observation of the operator therefore creates traces.

## Develop

Login to quay.io and push with `make push`
