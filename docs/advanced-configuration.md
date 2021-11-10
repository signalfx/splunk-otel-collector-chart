# Advanced Configuration

The
[values.yaml](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/helm-charts/splunk-otel-collector/values.yaml)
lists all supported configurable parameters for this chart, along with detailed
explanation. Read through it to understand how to configure this chart.

Also check [examples of chart configuration](./examples/README.md). This also includes a guide to deploy for the k8s cluster with the windows worker node.

At the minimum you need to configure the following values to send data to Splunk
Enterprise/Cloud.

```yaml
splunkPlatform:
  token: xxxxxx
  endpoint: http://localhost:8088/services/collector
```

At the minimum you need to configure the following values to send data to Splunk
Observability Cloud.

```yaml
splunkObservability:
  accessToken: xxxxxx
  realm: us0
clusterName: my-k8s-cluster
```

## Cloud provider

Use the `provider` parameter to provide information about the cloud provider, if any.

- `aws` - Amazon Web Services
- `gcp` - Google Cloud
- `azure` - Microsoft Azure

This value can be omitted if none of the values apply.

## Kubernetes distribution

Use the `distro` parameter to provide information about underlying Kubernetes
deployment. This parameter allows the connector to automatically scrape
additional metadata. The supported options are:

- `eks` - Amazon EKS
- `gke` - Google GKE
- `aks` - Azure AKS
- `openshift` - Red Hat OpenShift

This value can be omitted if none of the values apply.

## Deployment environment

Optional `environment` parameter can be used to specify an additional `deployment.environment`
attribute that will be added to all the telemetry data. It will help Splunk Observability
users to investigate data coming from different source separately.
Value examples: development, staging, production, etc.

```yaml
environment: production
```

## Disable particular types of telemetry

By default all telemetry data (metrics, traces and logs) is collected from the
Kubernetes cluster and sent to one of (or both) configured destinations. It's
possible to disable any kind of telemetry for a specific destination. For
example, the following configuration will send logs to Splunk Platform and
metrics and traces to Splunk Observability assuming that both destinations are
configured properly.

```yaml
splunkObservability:
  metricsEnabled: true
  tracesEnabled: true
  logsEnabled: false
splunkPlatform:
  metricsEnabled: false
  logsEnabled: true
```

## Logs collection

The helm chart currently utilizes [fluentd](https://docs.fluentd.org/) for Kubernetes logs
collection. Logs collected with fluentd are sent through Splunk OTel Collector agent which
does all the necessary metadata enrichment.

OpenTelemetry Collector also has
[native functionality for logs collection](https://github.com/open-telemetry/opentelemetry-log-collection).
This chart soon will be migrated from fluentd to the OpenTelemetry logs collection.

You already have an option to use OpenTelemetry logs collection instead of fluentd.
The following configuration can be used to achieve that:

```yaml
logsEngine: otel
```

There are following known limitations of native OTel logs collection:

- `service.name` attribute will not be automatically constructed in istio environment.
  This means that correlation between logs and traces will not work in Splunk Observability.
  Logs collection with fluentd is still recommended if chart deployed with `autodetect.istio=true`.
- Journald logs cannot be collected natively by Splunk OTel Collector yet.

## Additional telemetry sources

Use `autodetect` config option to enable additional telemetry sources.

Set `autodetect.prometheus=true` if you want the otel-collector agent to scrape
prometheus metrics from pods that have generic prometheus-style annotations:
- `prometheus.io/scrape: true`: Prometheus metrics will be scraped only from
  pods having this annotation;
- `prometheus.io/path`: path to scrape the metrics from, default `/metrics`;
- `prometheus.io/port`: port to scrape the metrics from, default `9090`.

Set `autodetect.istio=true`, if the otel-collector agent in running in Istio
environment, to make sure that all traces, metrics and logs reported by Istio
collected in a unified manner.

For example to enable both Prometheus and Istio telemetry add the following
lines to your `values.yaml` file:

```yaml
autodetect:
  istio: true
  prometheus: true
```
