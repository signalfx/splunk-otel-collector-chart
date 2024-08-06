# Example of chart configuration

## Use the OpenTelemetry Operator Target Allocator

**Notice: Operator related features should be considered to have an alpha maturity level and be experimental. There may be breaking changes or Operator features may be replaced entirely with a better alternative in the future.**

This example shows how to use the [OpenTelemetry Operator Target Allocator](https://opentelemetry.io/docs/kubernetes/operator/target-allocator/) with our Helm chart.

> The OpenTelemetry Operator comes with an optional component, the Target Allocator (TA). In a nutshell, the TA is a mechanism for decoupling the service discovery and metric collection functions of Prometheus such that they can be scaled independently. The Collector manages Prometheus metrics without needing to install Prometheus. The TA manages the configuration of the Collector’s Prometheus Receiver.
>
> The TA serves two functions:
>
> Even distribution of Prometheus targets among a pool of Collectors
> Discovery of Prometheus Custom Resources
