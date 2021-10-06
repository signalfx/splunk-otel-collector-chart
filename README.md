---

<p align="center">
  <strong>
    <a href="#getting-started">Getting Started</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="CONTRIBUTING.md">Getting Involved</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/blob/main/docs/signalfx-smart-agent-migration.md">Migrating from Smart Agent</a>
  </strong>
</p>

<p align="center">
  <a href="https://circleci.com/gh/signalfx/splunk-otel-collector-chart">
    <img alt="Build Status" src="https://img.shields.io/github/workflow/status/signalfx/splunk-otel-collector-chart/Lint%20and%20Test%20Charts?style=for-the-badge">
  </a>
  <a href="https://github.com/signalfx/splunk-otel-collector/releases">
    <img alt="GitHub release (latest by date including pre-releases)" src="https://img.shields.io/github/v/release/signalfx/splunk-otel-collector-chart?include_prereleases&style=for-the-badge">
  </a>
  <img alt="Beta" src="https://img.shields.io/badge/status-beta-informational?style=for-the-badge">
</p>

<p align="center">
  <strong>
    <a href="https://github.com/signalfx/splunk-otel-collector/blob/main/docs/components.md">Components</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/blob/main/docs/monitoring.md">Monitoring</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/blob/main/docs/security.md">Security</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/blob/main/docs/sizing.md">Sizing</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/blob/main/docs/troubleshooting.md">Troubleshooting</a>
  </strong>
</p>

---

# Splunk OpenTelemetry Connector for Kubernetes

The Splunk OpenTelemetry Connector for Kubernetes is a
[Helm](https://github.com/kubernetes/helm) chart for the [Splunk Distribution
of OpenTelemetry Collector](https://github.com/signalfx/splunk-otel-collector).
This chart creates a Kubernetes DaemonSet along with other Kubernetes objects
in a Kubernetes cluster and provides a unified way to receive, process and
export metric, trace, and log data for:

- [Splunk Enterprise](https://www.splunk.com/en_us/software/splunk-enterprise.html)
- [Splunk Cloud Platform](https://www.splunk.com/en_us/software/splunk-cloud-platform.html)
- [Splunk Observability Cloud](https://www.observability.splunk.com/)

**Installations that use this distribution can receive direct help from
Splunk's support teams.** Customers are free to use the core OpenTelemetry OSS
components (several do!) and we will provide best effort guidance to them for
any issues that crop up, however only the Splunk distributions are in scope for
official Splunk support and support-related SLAs.

This distribution currently supports:

- [Splunk APM](https://www.splunk.com/en_us/software/splunk-apm.html) via the
  [`sapm`
  exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/sapmexporter).
  The [`otlphttp`
  exporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlphttpexporter)
  can be used with a [custom
  configuration](https://github.com/signalfx/splunk-otel-collector/blob/main/cmd/otelcol/config/collector/otlp_config_linux.yaml).
  More information available
  [here](https://docs.signalfx.com/en/latest/apm/apm-getting-started/apm-opentelemetry-collector.html).
- [Splunk Infrastructure
  Monitoring](https://www.splunk.com/en_us/software/infrastructure-monitoring.html)
  via the [`signalfx`
  exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/signalfxexporter).
  More information available
  [here](https://docs.signalfx.com/en/latest/otel/imm-otel-collector.html).
- [Splunk Log Observer](https://www.splunk.com/en_us/form/splunk-log-observer.html) via
  the [`splunk_hec`
  exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/splunkhecexporter).
- [Splunk Cloud](https://www.splunk.com/en_us/software/splunk-cloud.html) or
  [Splunk
  Enterprise](https://www.splunk.com/en_us/software/splunk-enterprise.html) via
  the [`splunk_hec`
  exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/splunkhecexporter).

> :construction: This project is currently in **BETA**. It is **officially supported** by Splunk. However, breaking changes **MAY** be introduced.

### Supported Kubernetes distributions

This helm chart is tested and works with default configurations on the following
Kubernetes distributions:

- [Vanilla (unmodified version) Kubernetes](https://kubernetes.io)
- [Amazon Elastic Kubernetes Service](https://aws.amazon.com/eks)
- [Azure Kubernetes Service](https://docs.microsoft.com/en-us/azure/aks)
- [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine)
- [Red Hat OpenShift](https://www.redhat.com/en/technologies/cloud-computing/openshift)

While this helm chart should work for other Kubernetes distributions, it may
require additional configurations applied to
[values.yaml](helm-charts/splunk-otel-collector/values.yaml).

## Getting Started

### Prerequisites

The following prerequisites are required to use the helm chart:

- [Helm 3](https://helm.sh/docs/intro/install/) (Helm 2 is not supported)
- [Kubernetes cluster](https://kubernetes.io/)

- To send data to [Splunk Enterprise/Cloud](https://docs.splunk.com/Documentation/Splunk/8.2.2/Data/UsetheHTTPEventCollector)
  - HEC Token
  - HEC Endpoint

- To send data to [Splunk Observability Cloud](https://docs.splunk.com/Observability/gdi/opentelemetry/install-k8s.html)
  - [Splunk Access Token](https://docs.splunk.com/Observability/admin/authentication-tokens/org-tokens.html#admin-org-tokens)
  - [Splunk Realm](https://dev.splunk.com/observability/docs/realms_in_endpoints/)

### How to install

To install splunk-otel-collector in k8s cluster at one of the configuration groups
`spunkPlatform` or `splunkObservability` has to be fully configured.

For Splunk Enterprise/Cloud the following parameters are required:

- `spunkPlatform.endpoint`: URL to a Splunk instance, e.g.
  "http://localhost:8088/services/collector"
- `spunkPlatform.token`: Splunk HTTP Event Collector token

For Splunk Observability Cloud the following parameters are required:

- `splunkObservability.splunkRealm` (default `us0`): Splunk realm to send
  telemetry data to.
- `splunkObservability.splunkAccessToken`: Your Splunk Observability org access
  token.
- `clusterName`: arbitrary value that will identify your Kubernetes cluster in
  Splunk Observability Cloud.

To deploy the chart to send data to Splunk Observability Cloud run the following
commands replacing the parameters above with their appropriate values.

```bash
$ helm repo add splunk-otel-collector-chart https://signalfx.github.io/splunk-otel-collector-chart
$ helm install my-splunk-otel-collector --set="splunkObservability.splunkRealm=us0,splunkObservability.splunkAccessToken=xxxxxx,clusterName=my-cluster" splunk-otel-collector-chart/splunk-otel-collector
```

Instead of setting helm values as arguments a yaml file can be provided:

```bash
$ helm install my-splunk-otel-collector --values my_values.yaml splunk-otel-collector-chart/splunk-otel-collector
```

### How to uninstall

To uninstall/delete a deployment with name `my-splunk-otel-collector`:

```bash
$ helm delete my-splunk-otel-collector
```

The command removes all the Kubernetes components associated with the chart and
deletes the release.

## Advanced Configuration

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

### Cloud provider

Use the `provider` parameter to provide information about the cloud provider, if any.

- `aws` - Amazon Web Services
- `gcp` - Google Cloud
- `azure` - Microsoft Azure

This value can be omitted if none of the values apply.

### Kubernetes distribution

Use the `distro` parameter to provide information about underlying Kubernetes
deployment. This parameter allows the connector to automatically scrape
additional metadata. The supported options are:

- `eks` - Amazon EKS
- `gke` - Google GKE
- `aks` - Azure AKS
- `openshift` - Red Hat OpenShift

This value can be omitted if none of the values apply.

### Deployment environment

Optional `environment` parameter can be used to specify an additional `deployment.environment`
attribute that will be added to all the telemetry data. It will help Splunk Observability
users to investigate data coming from different source separately.
Value examples: development, staging, production, etc.

```yaml
environment: production
```

### Disable particular types of telemetry

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
fluentd:
  enabled: false
logsCollection:
  enabled: true
```

There are following known limitations of native OTel logs collection:

- Container attributes `container.id` and `container.image.name` are missed.
  This means that correlation between Splunk Log Observer and Splunk Infrastructure will not work
  on container level, but only on kubernetes pod level.
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

```
autodetect:
  istio: true
  prometheus: true
```

## Pre-rendered Kubernetes resources

The [rendered directory](rendered) contains pre-rendered Kubernetes resource manifests.

## Upgrade guidelines

### 0.35.3 to 0.36.0

[#209 Configuration interface changed to support both Splunk Enterprise/Cloud and Splunk Observability destinations](https://github.com/signalfx/splunk-otel-collector-chart/pull/209)

The following parameters required to send data to Splunk Observability are now
deprecated and moved under `splunkObservability` group. They need to be updated
in your custom values.yaml files before backward compatibility is discontinued.

- `splunkRealm` changed to `splunkObservability.realm`
- `splunkAccessToken` changed to `splunkObservability.accessToken`
- `ingestUrl` changed to `splunkObservability.ingestUrl`
- `apiUrl` changed to `splunkObservability.ingestUrl`
- `metricsEnabled` changed to `splunkObservability.metricsEnabled`
- `tracesEnabled` changed to `splunkObservability.tracesEnabled`
- `logsEnabled` changed to `splunkObservability.logsEnabled`

### 0.26.4 to 0.27.0

[#163 Auto-detection of prometheus metrics is disabled by default](https://github.com/signalfx/splunk-otel-collector-chart/pull/163):
If you rely on automatic prometheus endpoints detection to scrape prometheus
metrics from pods in your k8s cluster, make sure to add this configuration to
your values.yaml:

```
autodetect:
  prometheus: true
```

## License

[Apache Software License version 2.0](LICENSE).
