# Getting started with the Collector with Operator Auto-instrumentation

Auto-instrumentation helps you to easily add observability code to your application, enabling it to produce telemetry data. OpenTelemetry offers two main approaches to instrumentation: auto-instrumentation and manual instrumentation.

## Instrumentation Approaches

### Auto-Instrumentation
- Simple setup process
- Utilizes Kubernetes resources for implementation
- Only supports a subset of instrumentation libraries

### Manual Instrumentation
- More setup effort required
- Greater customizability
- Supports more instrumentation libraries and customization of telemetry data

Auto-instrumentation is particularly useful for applications using popular frameworks and libraries with pre-built instrumentation capabilities.

## Auto-Instrumentation Process Overview

OpenTelemetry auto-instrumentation is achieved using an [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) in a Kubernetes environment. The operator simplifies management of application auto-instrumentation, making it easier to gain insights into application performance.

When using the Splunk OTel Collector chart with the [OpenTelemetry Operator](https://github.com/open-telemetry/opentelemetry-operator#opentelemetry-auto-instrumentation-injection), you can deploy auto-instrumentation to your cluster and instrument your applications.

### Components Involved

- **Splunk OTel Collector Chart**: Deploys the collector and related resources, including the OpenTelemetry Operator.
- **OpenTelemetry Operator**: Manages auto-instrumentation of Kubernetes applications.
- **Instrumentation Libraries**: Generates telemetry data when your application uses instrumented components.
- **Kubernetes Object - opentelemetry.io/v1alpha1 Instrumentation**: Configures auto-instrumentation settings for applications.

## Splunk OTel Auto-instrumentation

### Quick Start
To use auto-instrumentation via the operator, these are the high-level steps:

1. Deploy OpenTelemetry infrastructure to your Kubernetes cluster, this includes cert-manager, Splunk OTel Collector, OpenTelemetry Operator, and Auto-Instrumentation Spec.
2. Apply annotations at the pod or namespace level for the Operator to know which pods to apply auto-instrumentation to.
3. Now, allow the Operator to do the work. As Kuberenetes api requests for create and update annotated pods are processed, the Operator will intercept and alter those requests so that the internal pod application containers are instrumented.

### More Information

For more technical documentation please see: [auto-instrumentation-install.md](auto-instrumentation-install.md)
