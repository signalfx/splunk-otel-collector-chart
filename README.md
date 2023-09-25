---

<p align="center">
  <strong>
    <a href="#getting-started">Getting Started</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/blob/main/docs/signalfx-smart-agent-migration.md">Migrating from Smart Agent</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="docs/migration-from-sck.md">Migrating from Splunk Connect for Kubernetes</a>
  </strong>
</p>

<p align="center">
  <a href="https://github.com/signalfx/splunk-otel-collector-chart/actions/workflows/lint-test.yaml?query=branch%3Amain">
    <img alt="Build Status" src="https://img.shields.io/github/actions/workflow/status/signalfx/splunk-otel-collector-chart/lint-test.yaml?branch=main&style=for-the-badge">
  </a>
  <a href="https://github.com/signalfx/splunk-otel-collector/releases">
    <img alt="GitHub release (latest by date including pre-releases)" src="https://img.shields.io/github/v/release/signalfx/splunk-otel-collector-chart?include_prereleases&style=for-the-badge">
  </a>
</p>

<p align="center">
  <strong>
    <a href="docs/advanced-configuration.md">Configuration</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/blob/main/docs/components.md">Components</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/blob/main/docs/monitoring.md">Monitoring</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/blob/main/docs/security.md">Security</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/signalfx/splunk-otel-collector/blob/main/docs/sizing.md">Sizing</a>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="docs/troubleshooting.md">Troubleshooting</a>
  </strong>
</p>

---

# Splunk OpenTelemetry Collector for Kubernetes

The Splunk OpenTelemetry Collector for Kubernetes is a [Helm](https://github.com/kubernetes/helm) chart for the [Splunk Distribution
of OpenTelemetry Collector](https://github.com/signalfx/splunk-otel-collector).
This chart creates a Kubernetes DaemonSet along with other Kubernetes objects
in a Kubernetes cluster and provides a unified way to receive, process and
export metric, trace, and log data for:

- [Splunk Enterprise](https://www.splunk.com/en_us/software/splunk-enterprise.html)
- [Splunk Cloud Platform](https://www.splunk.com/en_us/software/splunk-cloud-platform.html)
- [Splunk Observability Cloud](https://www.observability.splunk.com/)

## Current Status

- The Splunk OpenTelemetry Collector for Kubernetes Helm chart is production tested; it is in use by a number of customers in their production environments
- Customers using the helm chart can receive direct help from official Splunk support within SLA's
- Customers can use or migrate to the Splunk OpenTelemetry Collector for Kubernetes Helm chart without worrying about future breaking changes to its core configuration experience for metrics and traces collection (OpenTelemetry logs collection configuration is in beta). There may be breaking changes to the Collector's own metrics.

**Installations that use this distribution can receive direct help from
Splunk's support teams.** Customers are free to use the core OpenTelemetry OSS
components (several do!). We will provide best effort guidance for using these components;
however, only the Splunk distributions are in scope for official Splunk support and support-related SLAs.

This distribution currently supports:

- [Splunk APM](https://www.splunk.com/en_us/products/apm-application-performance-monitoring.html) via the
  [`sapm`
  exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/sapmexporter).
  The [`otlphttp`
  exporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlphttpexporter)
  can be used with a [custom
  configuration](https://github.com/signalfx/splunk-otel-collector/blob/main/cmd/otelcol/config/collector/otlp_config_linux.yaml).
  More information available
  [here](https://docs.signalfx.com/en/latest/apm/apm-getting-started/apm-opentelemetry-collector.html).
- [Splunk Infrastructure
  Monitoring](https://www.splunk.com/en_us/products/infrastructure-monitoring.html)
  via the [`signalfx`
  exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/signalfxexporter).
  More information available
  [here](https://docs.signalfx.com/en/latest/otel/imm-otel-collector.html).
- [Splunk Log Observer](https://www.splunk.com/en_us/products/log-observer.html) via
  the [`splunk_hec`
  exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/splunkhecexporter).
- [Splunk Cloud](https://www.splunk.com/en_us/products/splunk-cloud-platform.html) or
  [Splunk
  Enterprise](https://www.splunk.com/en_us/products/splunk-enterprise.html) via
  the [`splunk_hec`
  exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/splunkhecexporter).

## Supported Kubernetes distributions

This helm chart is tested and works with default configurations on the following
Kubernetes distributions:

- [Vanilla (unmodified version) Kubernetes](https://kubernetes.io)
- [Amazon Elastic Kubernetes Service](https://aws.amazon.com/eks)
  including [with Fargate profiles](docs/advanced-configuration.md#eks-fargate-support)
- [Azure Kubernetes Service](https://docs.microsoft.com/en-us/azure/aks)
- [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine)
  including [GKE Autopilot](docs/advanced-configuration.md#gke-autopilot-support)
- [Red Hat OpenShift](https://www.redhat.com/en/technologies/cloud-computing/openshift)

While this helm chart should work for other Kubernetes distributions, it may
require additional configurations applied to
[values.yaml](helm-charts/splunk-otel-collector/values.yaml).

## Getting Started

### Prerequisites

The following prerequisites are required to use the helm chart:

- [Helm 3](https://helm.sh/docs/intro/install/) (Helm 2 is not supported)
- Administrator access to your [Kubernetes cluster](https://kubernetes.io/) and familiarity with your Kubernetes configuration. You must know where your log information is being collected in your Kubernetes deployment.

#### To send data to Splunk Enterprise or Splunk Cloud

- Splunk Enterprise 8.0 or later.
- A minimum of one Splunk platform index ready to collect the log data. This index will be used for ingesting logs.
- An HTTP Event Collector (HEC) token and endpoint. See the following topics for more information:

  * [Set up and use HTTP Event Collector in Splunk Web](https://docs.splunk.com/Documentation/Splunk/8.2.0/Data/UsetheHTTPEventCollector)
  * [Scale HTTP Event Collector with distributed deployments](https://docs.splunk.com/Documentation/Splunk/8.2.0/Data/ScaleHTTPEventCollector)

#### To send data to Splunk Observability Cloud

- [Splunk Observability Cloud](https://docs.splunk.com/Observability/gdi/opentelemetry/install-k8s.html):
  - [Splunk Access Token](https://docs.splunk.com/Observability/admin/authentication-tokens/org-tokens.html#admin-org-tokens)
  - [Splunk Realm](https://dev.splunk.com/observability/docs/realms_in_endpoints/)

### Advanced Configuration

To fully configure the Helm chart, see the [advanced
configuration](docs/advanced-configuration.md).

### How to install

In order to install Splunk OpenTelemetry Collector in a Kubernetes cluster, at
least one of the destinations (`splunkPlatform` or `splunkObservability`) has
to be configured.

For Splunk Enterprise/Cloud the following parameters are required:

- `splunkPlatform.endpoint`: URL to a Splunk instance, e.g.
  "http://localhost:8088/services/collector"
- `splunkPlatform.token`: Splunk HTTP Event Collector token

For Splunk Observability Cloud the following parameters are required:

- `splunkObservability.realm`: Splunk realm to send telemetry data to.
- `splunkObservability.accessToken`: Your Splunk Observability org access
  token.

The following parameter is required for any of the destinations:

- `clusterName`: arbitrary value that identifies your Kubernetes cluster. The value will be associated with every trace, metric and log as "k8s.cluster.name" attribute.

Run the following commands, replacing the parameters above with their appropriate values.

Add Helm repo

```bash
helm repo add splunk-otel-collector-chart https://signalfx.github.io/splunk-otel-collector-chart
```

Sending data to Splunk Observability Cloud

```bash
helm install my-splunk-otel-collector --set="splunkObservability.realm=us0,splunkObservability.accessToken=xxxxxx,clusterName=my-cluster" splunk-otel-collector-chart/splunk-otel-collector
```

Sending data to Splunk Enterprise or Splunk Cloud

```bash
helm install my-splunk-otel-collector --set="splunkPlatform.endpoint=https://127.0.0.1:8088/services/collector,splunkPlatform.token=xxxxxx,splunkPlatform.metricsIndex=k8s-metrics,splunkPlatform.index=main,clusterName=my-cluster" splunk-otel-collector-chart/splunk-otel-collector
```

Sending data to both Splunk Observability Cloud and Splunk Enterprise or Splunk Cloud

```bash
helm install my-splunk-otel-collector --set="splunkPlatform.endpoint=https://127.0.0.1:8088/services/collector,splunkPlatform.token=xxxxxx,splunkPlatform.metricsIndex=k8s-metrics,splunkPlatform.index=main,splunkObservability.realm=us0,splunkObservability.accessToken=xxxxxx,clusterName=my-cluster" splunk-otel-collector-chart/splunk-otel-collector
```

You can specify a namespace to deploy the chart to with the `-n` argument. Here is an example showing how to deploy in the `otel` namespace:

```bash
helm -n otel install my-splunk-otel-collector -f values.yaml splunk-otel-collector-chart/splunk-otel-collector
```

Instead of setting helm values as arguments a YAML file can be provided:

```bash
helm install my-splunk-otel-collector --values my_values.yaml splunk-otel-collector-chart/splunk-otel-collector
```

The [examples directory](examples) contains examples of typical use cases with pre-rendered Kubernetes resource manifests for each example.

### How to upgrade

**Make sure you run `helm repo update` before you upgrade**

To upgrade a deployment follow the instructions for installing
but use `upgrade` instead of `install`, for example:

```bash
helm upgrade my-splunk-otel-collector --values my_values.yaml
```

### How to uninstall

To uninstall/delete a deployment with name `my-splunk-otel-collector`:

```bash
helm delete my-splunk-otel-collector
```

## Advanced Configuration

To fully configure the Helm chart, see the [advanced
configuration](docs/advanced-configuration.md).

## Auto-instrumentation

For setting up auto-instrumentation, see the [auto-instrumentation-introduction.md](docs/auto-instrumentation-introduction.md).

## Contributing

We welcome feedback and contributions from the community! Please see our ([contribution guidelines](CONTRIBUTING.md)) for more information on how to get involved.

## License

[Apache Software License version 2.0](LICENSE).

>ℹ️&nbsp;&nbsp;SignalFx was acquired by Splunk in October 2019. See [Splunk SignalFx](https://www.splunk.com/en_us/investor-relations/acquisitions/signalfx.html) for more information.
