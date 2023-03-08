# Auto-instrumentation
> :INFO: More documentation coming soon.

## Automatically instrumenting Kubernetes pods
This chart supports deploying auto-instrumentation of applications running
in Kubernetes pods via the OpenTelemetry Operator.
- This **Splunk Otel Collector Chart** will deploy the collector and the
  operator.
- The **[OpenTelemetry Operator](https://github.com/open-telemetry/opentelemetry-operator)**
  will be setup up auto-instrumentation of
  Kubernetes pods.
- The auto-instrumented applications can be configured to send data to
  collector agents, collector gateways, or Splunk backend ingestion endpoints.

## Getting Started

### Learn by example

See: [Java Spring Clinic Example](../examples/enable-operator-and-auto-instrumentation/README.md)

### Documentation Resources
https://developers.redhat.com/devnation/tech-talks/using-opentelemetry-on-kubernetes
https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md
https://github.com/open-telemetry/opentelemetry-operator/blob/main/docs/api.md#instrumentation
https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md#opentelemetry-auto-instrumentation-injection
https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md#use-customized-or-vendor-instrumentation
https://opentelemetry.io/docs/k8s-operator/
