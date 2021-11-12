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

# Splunk OpenTelemetry Collector for Kubernetes

> :construction: This project is currently in **BETA**. Splunk **officially supports** this project; however, there may be breaking changes.
>
The Splunk OpenTelemetry Collector for Kubernetes is a [Helm](https://github.com/kubernetes/helm) chart for the [Splunk Distribution
of OpenTelemetry Collector](https://github.com/signalfx/splunk-otel-collector).
This chart creates a Kubernetes DaemonSet along with other Kubernetes objects
in a Kubernetes cluster and provides a unified way to receive, process and
export metric, trace, and log data for:

- [Splunk Enterprise](https://www.splunk.com/en_us/software/splunk-enterprise.html)
- [Splunk Cloud Platform](https://www.splunk.com/en_us/software/splunk-cloud-platform.html)
- [Splunk Observability Cloud](https://www.observability.splunk.com/)

**Installations that use this distribution can receive direct help from
Splunk's support teams.** Customers are free to use the core OpenTelemetry OSS
components (several do!) and we will provide our best effort to guide them through any issues that crop up. However only the Splunk distributions are officially supported by Splunk support and support-related SLAs.

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
- See https://github.com/kubernetes/helm for more information.

#### To send data to Splunk Enterprise/Cloud:
- Splunk Enterprise 7.0 or later.
- A minimum of one Splunk platform index ready to collect the log data. This index will be used for ingesting logs.
- An HTTP Event Collector (HEC) token and endpoint. See the following topics for more information:
  * https://docs.splunk.com/Documentation/Splunk/8.2.0/Data/UsetheHTTPEventCollector
  * https://docs.splunk.com/Documentation/Splunk/8.2.0/Data/ScaleHTTPEventCollector

- To send data to [Splunk Observability Cloud](https://docs.splunk.com/Observability/gdi/opentelemetry/install-k8s.html):
  - [Splunk Access Token](https://docs.splunk.com/Observability/admin/authentication-tokens/org-tokens.html#admin-org-tokens)
  - [Splunk Realm](https://dev.splunk.com/observability/docs/realms_in_endpoints/)


- Administrator access to your [Kubernetes cluster](https://kubernetes.io/) and familiarity with your Kubernetes configuration. You must know where your log information is being collected in your Kubernetes deployment.

### To install and configure defaults with Helm:

```bash
helm install my-splunk-otel-collector --set="splunkPlatform.endpoint=127.0.0.1:8088,splunkPlatform.token=xxxxxx,splunkPlatform.metrics_index=k8s-metrics,splunkPlatform.index=main,clusterName=my-cluster" splunk-otel-collector-chart/splunk-otel-collector
```


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
- `clusterName`: arbitrary value that will identify your Kubernetes cluster in
  Splunk Observability Cloud.

To deploy the chart to send data to Splunk Observability Cloud run the following
commands replacing the parameters above with their appropriate values.

```bash
$ helm repo add splunk-otel-collector-chart https://signalfx.github.io/splunk-otel-collector-chart
$ helm install my-splunk-otel-collector --set="splunkObservability.realm=us0,splunkObservability.accessToken=xxxxxx,clusterName=my-cluster" splunk-otel-collector-chart/splunk-otel-collector
```

Instead of setting helm values as arguments a YAML file can be provided:

```bash
$ helm install my-splunk-otel-collector --values my_values.yaml splunk-otel-collector-chart/splunk-otel-collector
```

The [rendered directory](rendered) contains pre-rendered Kubernetes resource manifests.

### How to uninstall

To uninstall/delete a deployment with name `my-splunk-otel-collector`:

```bash
$ helm delete my-splunk-otel-collector
```

## Log Architecture

Splunk OpenTelemetry Collector for Kubernetes deploys a DaemonSet on each node. And in the DaemonSet, a OpenTelemetry container runs and does the collecting job. Splunk OpenTelemetry Collector for Kubernetes uses the [node logging agent](https://kubernetes.io/docs/concepts/cluster-administration/logging/#using-a-node-logging-agent) method. See the [Kubernetes Logging Architecture](https://kubernetes.io/docs/concepts/cluster-administration/logging/) for an overview of the types of Kubernetes logs from which you may wish to collect data as well as information on how to set up those logs.
Splunk OpenTelemetry Collector for Kubernetes collects the following types of data:

* Logs: Splunk OpenTelemetry Collector for Kubernetes collects two types of logs:
  * Logs from Kubernetes system components (https://kubernetes.io/docs/concepts/overview/components/)
  * Applications (container) logs

To collect the data, Splunk OpenTelemetry Collector for Kubernetes leverages OpenTelemetry and the following receivers, processors, exporters and extensions:
* [OpenTelemetry](https://opentelemetry.io/)
* [OpenTelemetry collector](https://github.com/open-telemetry/opentelemetry-collector)
* [OpenTelemetry contrib collector](https://github.com/open-telemetry/opentelemetry-collector-contrib)
* [OpenTelemetry log collection](https://github.com/open-telemetry/opentelemetry-log-collection)
* [OpenTelemetry file storage extension](https://pkg.go.dev/github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage) for checkpointing
* [OpenTelemetry health check extension](https://pkg.go.dev/github.com/open-telemetry/opentelemetry-collector/extension/healthcheckextension) for overall health and status of the OpenTelemetry agent
* [OpenTelemetry filelog receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver) for tailing and parsing logs from files using the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) library
* [OpenTelemetry batch processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor) for compressing logs data and reduce the number of outgoing connections required to transmit the data
* [OpenTelemetry memory limiter processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiter) to prevent out of memory situations on the OpenTelemetry agent
* [OpenTelemetry kubernetes tagger processor](https://pkg.go.dev/github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor) for automatic tagging of logs with k8s metadata
* [OpenTelemetry resource processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/resourceprocessor) to apply changes on resource attributes
* [OpenTelemetry attributes processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/attributesprocessor) to modify attributes of logs
* Splunk OpenTelemetry Collector for Kubernetes uses multiple operators from [OpenTelemetry log collection operators](https://github.com/open-telemetry/opentelemetry-log-collection/tree/main/docs/operators) like regex_parser, recombine, restructure, json_parser, metadata for enriching logs with metadata and transforming/standardizing logs and metadata from various container runtimes
* [OpenTelemetry Splunk HEC exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/splunkhecexporter): to send logs to Splunk HTTP Event Collector. The [HTTP Event Collector](http://dev.splunk.com/view/event-collector/SP-CAAAE6M) collects all data sent to Splunk for indexing.

### Add logs from different Kubernetes distributions and container runtimes like(docker, cri-o, containerd)

Select the proper container runtime for your Kubernetes distribution.

[Example](https://github.com/splunk/sck-otel/blob/main/charts/sck-otel/values.yaml#L47)

### Add log files from Kubernetes host machines/volumes

You can add additional log files to be ingested from Kubernetes host machines and kubernetes volumes by configuring extraHostPathMounts and extraHostFileConfig in the values.yaml file used to deploy Splunk OpenTelemetry Collector for Kubernetes.

[Example](https://github.com/splunk/sck-otel/blob/main/example/values/extraHostFileValues.yaml#L102)

### Override underlying OpenTelemetry Agent configuration
If you want to use your own OpenTelemetry Agent configuration, you can build a [OpenTelemetry Agent config](https://github.com/splunk/sck-otel/blob/main/example/manifests/otel_config.yaml) and override our default config by configuring configOverride in the values.yaml file used to deploy Splunk OpenTelemetry Collector for Kubernetes.

### Adding Audit logs from Kubernetes host machines
You can ingest audit logs from your Kubernetes cluster by configuring extraHostPathMounts and extraHostFileConfig in the values.yaml file used to deploy Splunk OpenTelemetry Collector for Kubernetes.

[Example](https://github.com/splunk/sck-otel/blob/main/charts/sck-otel/values.yaml#L122)

### Processing Multi-Line Logs
Splunk Connect for Kubernetes-OpenTelmetry supports parsing of multiline logs to help read, understand and troubleshoot the multiline logs in a better way.
Process multiline logs by configuring `multilineSupportConfig` section in values.yaml.

[Example](https://github.com/splunk/sck-otel/blob/9bd92b9b2054b85eadfd744888cc19ebb46b0081/charts/sck-otel/values.yaml#L77)

If you have a specific format you are using for formatting a python stack traces, you can take an example of your stack trace output and use https://regex101.com/  to find a golang regex that works for your format and specify it in the config file for the config option "first_entry_regex" and for the config option pass in the appropriate container name.

### Tweak Performance/resources used by Splunk OpenTelemetry Collector for Kubernetes
If you want to tweak performance/cpu and memory resources used by  Splunk OpenTelemetry Collector for Kubernetes change the available cpu and memory for the Opentelemtry Agent by configuring resources:limits:cpu and resources:limits:memory in the values.yaml file used to deploy Splunk OpenTelemetry Collector for Kubernetes.

[Example](https://github.com/splunk/sck-otel/blob/main/charts/sck-otel/values.yaml#L143)

## Maintenance And Support
Splunk OpenTelemetry Collector for Kubernetes is supported through Splunk Support assuming the customer has a current Splunk support entitlement ([Splunk Support](https://www.splunk.com/en_us/about-splunk/contact-us.html#tabs/tab_parsys_tabs_CustomerSupport_4)). For customers that do not have a current Splunk support entitlement, please search [open and closed issues](https://github.com/splunk/sck-otel/issues?q=is%3Aissue) and create a new issue if not already there.
The current maintainers of this project are the DataEdge team at Splunk.

## Contributing
We welcome feedback and contributions from the community! Please see our ([contribution guidelines](https://github.com/splunk/sck-otel/blob/main/CONTRIBUTING.md)) for more information on how to get involved. PR contributions require acceptance of both the code of conduct and the contributor license agreement.

## Upgrading
## v0.2.x -> v0.3.0
If using `.Values.configOverride` and have expressions that refer log record, double up `$` characters for those expressions. [Expressions](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/types/expression.md)


## License

[Apache Software License version 2.0](LICENSE).
