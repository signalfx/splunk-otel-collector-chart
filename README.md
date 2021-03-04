# Splunk OpenTelemetry Connector for Kubernetes

The Splunk OpenTelemetry Connector for Kubernetes is a
[Helm](https://github.com/kubernetes/helm) chart for the [Splunk Distribution
of OpenTelemetry Collector](https://github.com/signalfx/splunk-otel-collector).
This chart creates a Kubernetes DaemonSet along with other Kubernetes objects
in a Kubernetes cluster to collect the cluster's:

- Metrics for [Splunk Infrastructure
  Monitoring](https://www.splunk.com/en_us/software/infrastructure-monitoring.html)
- Traces for [Splunk
  APM](https://www.splunk.com/en_us/software/microservices-apm.html)
- Logs for Splunk Log Observer

> :construction: This project is currently in **BETA**.

## Getting Started

### Prerequisites

The following components required to use the helm chart:

- [Helm client](https://helm.sh/docs/intro/install/)
- [Kubernetes cluster](https://kubernetes.io/)

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
$ helm install my-splunk-otel-collector --values my_values.yaml ./helm-charts/splunk-otel-collector
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
  ./helm-charts/splunk-otel-collector
```

## License

[Apache Software License version 2.0](LICENSE).
