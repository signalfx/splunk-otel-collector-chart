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

The helm chart currently utilizes fluentd by default for Kubernetes logs collection and supports an option to use native OpenTelemetry logs collection for higher throughput and performance. [Go to Logs collection section for more details along with performance benchmarks run internally](#logs-collection)

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

In order to install Splunk OpenTelemetry Connector in a k8s cluster, at least
one of the destinations (`splunkPlatform` or `splunkObservability`) has to be
configured.

For Splunk Enterprise/Cloud the following parameters are required:

- `splunkPlatform.endpoint`: URL to a Splunk instance, e.g.
  "http://localhost:8088/services/collector"
- `splunkPlatform.token`: Splunk HTTP Event Collector token

For Splunk Observability Cloud the following parameters are required:

- `splunkObservability.realm`: Splunk realm to send telemetry data to.
- `splunkObservability.accessToken`: Your Splunk Observability org access
  token.
- `clusterName`: arbitrary value that will identify your Kubernetes cluster in
  Splunk Observability Cloud.

To deploy the chart to send data to Splunk Observability Cloud run the following
commands replacing the parameters above with their appropriate values.

```bash
$ helm repo add splunk-otel-collector-chart https://signalfx.github.io/splunk-otel-collector-chart
$ helm install my-splunk-otel-collector --set="splunkObservability.realm=us0,splunkObservability.accessToken=xxxxxx,clusterName=my-cluster" splunk-otel-collector-chart/splunk-otel-collector
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
logsEngine: otel
```

There are following known limitations of native OTel logs collection:

- `service.name` attribute will not be automatically constructed in istio environment.
  This means that correlation between logs and traces will not work in Splunk Observability.
  Logs collection with fluentd is still recommended if chart deployed with `autodetect.istio=true`.
- Journald logs cannot be collected natively by Splunk OTel Collector yet.

### Performance of Splunk Connect for Kubernetes-OpenTelemetry

Some configurations used with Splunk Connect for Kubernetes-OpenTelemetry can have an impact on overall performance of log ingestion. The more receivers, processors, exporters and extensions that are added to any of the pipelines, the greater the performance impact.

Splunk Connect for Kubernetes-OpenTelemetry can exceed the default throughput of HEC. To best address capacity needs, Splunk recommends that you monitor the HEC throughput and back pressure on Splunk Connect for Kubernetes-OpenTelemetry deployments and be prepared to add additional nodes as needed.

Here is the summary of performance benchmarks run internally.
| Log Generator Count | Total Generated EPS | Event Size (byte) | Agent CPU Usage | Agent EPS |
|---------------------|---------------------|-------------------|-----------------|-----------|
|                   1 |              27,000 |               256 |             1.6 |    27,000 |
|                   1 |              49,000 |               256 |             1.8 |    30,000 |
|                   1 |              49,000 |               516 |             1.8 |    28,000 |
|                   1 |              49,000 |              1024 |             1.8 |    24,000 |
|                   2 |              20,000 |               256 |             1.3 |    20,000 |
|                   7 |              40,000 |               256 |             2.4 |    40,000 |
|                   5 |              58,000 |               256 |             3.2 |    54,000 |
|                   7 |              82,000 |               256 |               3 |    52,000 |
|                  10 |              58,000 |               256 |             3.2 |    53,000 |


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

### 0.36.2 to 0.37.0

[#232 Access to underlying node's filesystem was reduced to the minimum scope
required for default functionality: host metrics and logs
collection](https://github.com/signalfx/splunk-otel-collector-chart/pull/232)

If you have any extra receivers that require access to node's files or
directories that are not [mounted by
default](https://github.com/signalfx/splunk-otel-collector-chart/blob/83fefe2a01effaab1e9eaba34a2557863981a2cd/helm-charts/splunk-otel-collector/templates/daemonset.yaml#L330-L347),
you need to setup additional volume mounts.

For example, if you have the following `smartagent/docker-container-stats`
receiver added to your configuration:

```yaml
otelAgent:
  config:
    receivers:
      smartagent/docker-container-stats:
        type: docker-container-stats
        dockerURL: unix:///hostfs/var/run/docker.sock
```

You need to mount the docker socket to your container as follows:

```yaml
  extraVolumeMounts:
    - mountPath: /hostfs/var/run/docker.sock
      name: host-var-run-docker
      readOnly: true
  extraVolumes:
    - name: host-var-run-docker
      hostPath:
        path: /var/run/docker.sock
```

[#246 Simplify configuration for switching to native OTel logs
collection](https://github.com/signalfx/splunk-otel-collector-chart/pull/246)

The config to enable native OTel logs collection was changed from

```yaml
fluentd:
  enabled: false
logsCollection:
  enabled: true
```

to

```yaml
logsEngine: otel
```

Enabling both engines is not supported anymore. If you need that, you can
install fluentd separately.

### 0.35.3 to 0.36.0

[#209 Configuration interface changed to support both Splunk Enterprise/Cloud and Splunk Observability destinations](https://github.com/signalfx/splunk-otel-collector-chart/pull/209)

The following parameters are now deprecated and moved under
`splunkObservability` group. They need to be updated in your custom values.yaml
files before backward compatibility is discontinued.

Required parameters:

- `splunkRealm` changed to `splunkObservability.realm`
- `splunkAccessToken` changed to `splunkObservability.accessToken`

Optional parameters:

- `ingestUrl` changed to `splunkObservability.ingestUrl`
- `apiUrl` changed to `splunkObservability.apiUrl`
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
