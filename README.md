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
    <a href="docs/troubleshooting.md">Troubleshooting</a>
  </strong>
</p>

---

# Splunk OpenTelemetry Collector for Kubernetes

> :construction: This project is currently in **BETA**. Splunk **officially supports** this project. However, there may be breaking changes.
> 
The Splunk OpenTelemetry Collector for Kubernetes is a
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

The Helm chart currently uses Fluentd by default for Kubernetes logs collection, and supports an option to use native OpenTelemetry logs collection for higher throughput and performance. See the [logs collection section](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/docs/advanced-configuration.md#logs-collection) for more information, along with performance benchmarks run internally.

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
- See https://github.com/kubernetes/helm for more information.
- Splunk Enterprise 7.0 or later.
- A minimum of one Splunk platform index ready to collect the log data. This index will be used for ingesting logs.
- An HTTP Event Collector (HEC) token and endpoint. See the following topics for more information:
  * https://docs.splunk.com/Documentation/Splunk/8.2.0/Data/UsetheHTTPEventCollector
  * https://docs.splunk.com/Documentation/Splunk/8.2.0/Data/ScaleHTTPEventCollector
- To send data to [Splunk Observability Cloud:](https://docs.splunk.com/Observability/gdi/opentelemetry/install-k8s.html)
  - [Splunk Access Token](https://docs.splunk.com/Observability/admin/authentication-tokens/org-tokens.html#admin-org-tokens)
  - [Splunk Realm](https://dev.splunk.com/observability/docs/realms_in_endpoints/)
- Administrator access to your [Kubernetes cluster](https://kubernetes.io/) and familiarity with your Kubernetes configuration. You must know where your log information is being collected in your Kubernetes deployment.

### Deploy with Helm 3.0+

Helm, maintained by the CNCF, allows the Kubernetes administrator to install, upgrade, and manage the applications running in their Kubernetes clusters.  For more information on how to use and configure Helm Charts,  see the Helm [site](https://helm.sh/) and [repository](https://github.com/kubernetes/helm) for tutorials and product documentation. Helm is the only method that the Splunk software supports for installing Splunk Connect for Kubernetes.

To install and configure defaults with Helm:

* Add Splunk chart repo
```bash
helm repo add splunk-otel https://splunk.github.io/sck-otel/
```

* Get values file in your working directory

```bash
helm show values splunk-otel/sck-otel > values.yaml
```

* Prepare this Values file. This file has a lot of documentation for configuring Splunk OpenTelemetry Collector for Kubernetes. Look at this [example](https://github.com/splunk/sck-otel/blob/main/charts/sck-otel/values.yaml). Once you have a Values file, you can simply install the chart with by running

```bash
helm install my-splunk-connect -f my_values.yaml splunk-otel/sck-otel
```

To learn more about using and modifying charts, see:
* https://github.com/splunk/sck-otel/tree/main/charts
* https://docs.helm.sh/using_helm/#using-helm.


### Configuration variables for Helm

The default values file can be found here [default values file](https://github.com/splunk/sck-otel/blob/main/charts/sck-otel/values.yaml)


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

The command removes all the Kubernetes components associated with the chart and
deletes the release.

### Advanced Configuration

To fully configure the Helm chart, see the [advanced
configuration](docs/advanced-configuration.md).

## Setup Splunk

Splunk OpenTelemetry Collector for Kubernetes supports installation using Helm. Read the Prerequisites, Installation and Deployment documentation before you start your deployment of Splunk OpenTelemetry Collector for Kubernetes.

Perform the following steps before you install:

1. Create a minimum of one Splunk platform index. This index events index, which will handle logs.
If you do not configure this index, Splunk OpenTelemetry Collector for Kubernetes uses the defaults created in your HTTP Event Collector (HEC) token.

2. Create a HEC token if you do not already have one. If you are installing the connector on Splunk Cloud, file a ticket with Splunk Customer Service and they will deploy the indexes for your environment, and generate your HEC token.

### Setup for Non-Root User Group

It is best practice to run pods as a non-root user. To avoid running collector pod as `root` user, perform below steps on each kubernetes nodes.

In this chart, it is set to run as as a user with UID and GID of `10001` ([set here](https://github.com/splunk/sck-otel/blob/main/charts/sck-otel/values.yaml#L104)). But this user does not have the permission to read container log files typically owned by `root`. Below steps create a user with GID 10001 and grant access to that GID.

```bash
## create a user otel with uid=10001 and gid=10001
sudo adduser --disabled-password --uid 10001 --no-create-home otel

## setup a directory for storing checkpoints
sudo mkdir /var/lib/otel_pos
sudo chgrp otel /var/lib/otel_pos
sudo chmod g+rwx /var/lib/otel_pos

## setup container log directories.
## To check where the files are, check symlinks file on `/var/log/pods/` and its target paths.
ls -Rl /var/log/pods
## default paths are these
## `/var/lib/docker/containers` for docker
## `/var/log/crio/pods` for cri-o
## `/var/log/pods` for containerd
## add your container log path if different
if [ -d "/var/lib/docker/containers" ]
then
    sudo chgrp -R otel /var/lib/docker/containers
    sudo chmod -R g+rwx /var/lib/docker/containers
    sudo setfacl -Rm d:g:otel:rwx,g:otel:rwx /var/lib/docker/containers
fi

if [ -d "/var/log/crio/pods" ]
then
    sudo chgrp -R otel /var/log/crio/pods
    sudo chmod -R g+rwx /var/log/crio/pods
    sudo setfacl -Rm d:g:otel:rwx,g:otel:rwx /var/log/crio/pods
fi

if [ -d "/var/log/pods" ]
then
    sudo chgrp -R otel /var/log/pods
    sudo chmod -R g+rwx /var/log/pods
    sudo setfacl -Rm d:g:otel:rwx,g:otel:rwx /var/log/pods
fi
```


## Architecture

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


## Performance of Splunk OpenTelemetry Collector for Kubernetes

Some configurations used with Splunk OpenTelemetry Collector for Kubernetes can have an impact on overall performance of log ingestion. The more receivers, processors, exporters and extensions that are added to any of the pipelines, the greater the performance impact.

Splunk OpenTelemetry Collector for Kubernetes can exceed the default throughput of HEC. To best address capacity needs, Splunk recommends that you monitor the HEC throughput and back pressure on Splunk OpenTelemetry Collector for Kubernetes deployments and be prepared to add additional nodes as needed.

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



## Manage Splunk OpenTelemetry Collector for Kubernetes Log Ingestion by Using Annotations

Manage Splunk OpenTelemetry Collector for Kubernetes Logging with these supported annotations.
* Use `splunk.com/index` annotation on pod and/or namespace to tell which Splunk platform indexes to ingest to. Pod annotation will take precedence over namespace annotation when both are annotated.
  ex) `kubectl annotate namespace kube-system splunk.com/index=k8s_events`
* Use `splunk.com/sourcetype` annotation on pod to overwrite `sourcetype` field. If not set, it is dynamically generated to be `kube:container:CONTAINER_NAME` where CONTAINER_NAME is the container name of the container running in the pod.
* Set `splunk.com/exclude` annotation to true on pod and/or namespace to exclude its logs from ingested to your Splunk platform deployment.
* Set `splunk.com/include` annotation to true on pod and `containerLogs.useSplunkIncludeAnnotation` flag to `true` to include its logs from ingested to your Splunk platform deployment.
  All other logs will be ignored. You cant use this feature with the above mentioned exclude feature. You can only use either the include feature or the exclude feature.

## Search for Splunk OpenTelemetry Collector for Kubernetes metadata in Splunk
Splunk OpenTelemetry Collector for Kubernetes sends events to Splunk which can contain extra meta-data attached to each event. Metadata values such as "k8s.pod.name", "k8s.pod.uid", "k8s.deployment.name","k8s.cluster.name", "k8s.namespace.name", "k8s.node.name", "k8s.pod.start_time", "container_name", "run_id" and "stream" will appear as fields when viewing the event data inside Splunk.
There are two solutions for running searches in Splunk on meta-data.

* Modify search to use`fieldname::value` instead of `fieldname=value`.
* Configure `fields.conf` on your downstream Splunk system to have your meta-data fields available to be searched using `fieldname=value`. Example: [fields.conf.example](https://github.com/splunk/sck-otel/blob/main/example/fields.conf.example)

For more information on index time field extraction please view this [guide](https://docs.splunk.com/Documentation/Splunk/latest/Data/Configureindex-timefieldextraction#Where_to_put_the_configuration_changes_in_a_distributed_environment).

## Advanced Configurations for Splunk OpenTelemetry Collector for Kubernetes

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
