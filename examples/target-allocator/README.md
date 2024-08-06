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

In this example, we deploy the Target Allocator separately as a Kubernetes deployment with the [`target-allocator.yaml`](./target-allocator.yaml) file.

This file configures the Target Allocator to monitor all service monitors and pod monitors across all namespaces and is offered as a suggestion. It should not be used in production.

To deploy the Target Allocator, one can run:
```
$> kubectl apply -f target-allocator.yaml
```


We configure the daemonset to connect to the Target Allocator service to receive scrape targets.

```yaml
agent:
  config:
    receivers:
      prometheus/crd:
        config:
          global:
            scrape_interval: 5s
        target_allocator:
          endpoint: http://targetallocator-service.default.svc.cluster.local:80
          interval: 10s
          collector_id: ${env:K8S_POD_NAME}
    service:
      pipelines:
        metrics:
          receivers:
            - hostmetrics
            - kubeletstats
            - otlp
            - prometheus/crd
            - signalfx
```
