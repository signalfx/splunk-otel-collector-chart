---

<p align="center">
  <strong>
    <a href="#getting-started">Getting Started</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="CONTRIBUTING.md">Getting Involved</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="docs/signalfx-smart-agent-migration.md">Migrating from Smart Agent</a>
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
    <a href="https://github.com/signalfx/splunk-otel-collector/docs/components.md">Components</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/docs/monitoring.md">Monitoring</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/docs/security.md">Security</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/docs/sizing.md">Sizing</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/docs/troubleshooting.md">Troubleshooting</a>
  </strong>
</p>

---

# Splunk OpenTelemetry Connector for Kubernetes

The Splunk OpenTelemetry Connector for Kubernetes is a
[Helm](https://github.com/kubernetes/helm) chart for the [Splunk Distribution
of OpenTelemetry Collector](https://github.com/signalfx/splunk-otel-collector).
This chart creates a Kubernetes DaemonSet along with other Kubernetes objects
in a Kubernetes cluster and provides a unified way to receive, process and
export metric, trace, and log data for [Splunk Observability
Cloud](https://www.observability.splunk.com/).

**Installations that use this distribution can receive direct help from
Splunk's support teams.** Customers are free to use the core OpenTelemetry OSS
components (several do!) and we will provide best effort guidance to them for
any issues that crop up, however only the Splunk distributions are in scope for
official Splunk support and support-related SLAs.

This distribution currently supports:

- [Splunk APM](https://www.splunk.com/en_us/software/splunk-apm.html) via the
  [`sapm`
  exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/master/exporter/sapmexporter).
  The [`otlphttp`
  exporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlphttpexporter)
  can be used with a [custom
  configuration](https://github.com/signalfx/splunk-otel-collector/blob/main/cmd/otelcol/config/collector/otlp_config_linux.yaml).
  More information available
  [here](https://docs.signalfx.com/en/latest/apm/apm-getting-started/apm-opentelemetry-collector.html).
- [Splunk Infrastructure
  Monitoring](https://www.splunk.com/en_us/software/infrastructure-monitoring.html)
  via the [`signalfx`
  exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/master/exporter/signalfxexporter).
  More information available
  [here](https://docs.signalfx.com/en/latest/otel/imm-otel-collector.html).
- [Splunk Log Observer](https://www.splunk.com/en_us/form/splunk-log-observer.html) via
  the [`splunk_hec`
  exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/master/exporter/splunkhecexporter).
- [Splunk Cloud](https://www.splunk.com/en_us/software/splunk-cloud.html) or
  [Splunk
  Enterprise](https://www.splunk.com/en_us/software/splunk-enterprise.html) via
  the [`splunk_hec`
  exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/master/exporter/splunkhecexporter).

> :construction: This project is currently in **BETA**.

## Getting Started

### Prerequisites

The following components required to use the helm chart:

- [Helm client](https://helm.sh/docs/intro/install/)
- [Kubernetes cluster](https://kubernetes.io/)
- [Splunk Access Token](https://docs.splunk.com/Observability/admin/authentication-tokens/org-tokens.html#admin-org-tokens)
- [Splunk Realm](https://dev.splunk.com/observability/docs/realms_in_endpoints/)

### How to install

To install splunk-otel-collector in k8s cluster at least three parameters must be provided:

- `splunkRealm` (default `us0`): Splunk realm to send telemetry data to.
- `splunkAccessToken`: Your Splunk org access token.
- `clusterName`: arbitrary value that will identify your Kubernetes cluster in Splunk.

To deploy the chart run the following commands replacing the parameters above
with their appropriate values.

```bash
$ helm repo add splunk-otel-collector-chart https://signalfx.github.io/splunk-otel-collector-chart
$ helm install my-splunk-otel-collector --set="splunkRealm=us0,splunkAccessToken=xxxxxx,clusterName=my-cluster" splunk-otel-collector-chart/splunk-otel-collector
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

At the minimum you need to configure the following values.

```yaml
clusterName: my-k8s-cluster
splunkAccessToken: xxxxxx
splunkRealm: us0
```

### Kubernetes platform

Use the `platform` parameter to provide information about underlying Kubernetes
platform. This parameter allows the connector to automatically scrape
additional cloud metadata. The supported options are:

- `aws` - Amazon EKS or self-managed k8s cluster in AWS environment.
- `gcp` - Google GKE or self-managed k8s cluster in GCP environment.
- `default` - default configuration for other platforms.

### Deployment environment

Optional `environment` parameter can be used to specify an additional `deployment.environment`
attribute that will be added to all the telemetry data. It will help Splunk Observability 
users to investigate data coming from different source separately. 
Value examples: development, staging, production, etc.

```yaml
environment: production
```

### Disable particular types of telemetry

By default all telemetry data (metrics, traces and logs) is collected from the Kubernetes cluster.
It's possible to disable any kind of telemetry with the following parameters:

- `metricsEnabled`: `false`
- `tracesEnabled`: `false`
- `logsEnabled`: `false`

For example, to install the connector only for logs:

```bash
$ helm install my-splunk-otel-collector \
  --set="splunkRealm=us0,splunkAccessToken=xxxxxx,clusterName=my-cluster,metricsEnabled=false,tracesEnabled=false" \
  splunk-otel-collector-chart/splunk-otel-collector
```

## License

[Apache Software License version 2.0](LICENSE).
