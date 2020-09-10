# Observability Collector for Kubernetes

`o11y-collector-for-kubernetes` is a [Helm](https://github.com/kubernetes/helm) chart that
creates a kubernetes daemonset along with other kubernetes objects in a
kubernetes cluster to collect the cluster's logs, traces and metrics send them to
[Signalfx](https://www.signalfx.com/).

### Log Collection

The deamonset runs [fluentd](https://www.fluentd.org/) with the
[Splunk HEC output plugin](https://github.com/splunk/fluent-plugin-splunk-hec)
to collect logs and send them over
[Splunk HEC](http://docs.splunk.com/Documentation/Splunk/7.1.0/Data/AboutHEC).

It does not only collects logs for applications which are running in the
kubernetes cluster, but also the logs for kubernetes itself (i.e. logs from
`kubelet`, `apiserver`, etc.). It reads logs from both file system with the
[fluentd tail plugin](https://docs.fluentd.org/v1.0/articles/in_tail) and
[systemd journal](http://0pointer.de/blog/projects/journalctl.html) with
[`fluent-plugin-systemd`](https://github.com/reevoo/fluent-plugin-systemd).

### Trace Collection

The deamonset runs [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector/) with the
[Splunk SAPM Exporter][(https://github.com/splunk/fluent-plugin-splunk-hec](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/master/exporter/sapmexporter))
to collect traces and send them to
[Splunk SignalFx Microservices APM](https://www.splunk.com/en_us/software/microservices-apm.html).

### Metric Collection

[OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector/) also collects kubernetes and host
metrics using the following components enabled by default:
- [Kubeletstats receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/master/receiver/kubeletstatsreceiver)
to collect metrics from Kubelet API.
- [K8s cluster receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/master/receiver/k8sclusterreceiver)
to collect metrics from Kubernetes API.
- [Host metrics receiver](https://github.com/open-telemetry/opentelemetry-collector/tree/master/receiver/hostmetricsreceiver)
to collect host metrics from kubernetes node.

The metrics are sent to
[Splunk SignalFx Infrastructure Monitoring](https://www.splunk.com/en_us/software/infrastructure-monitoring.html).

## Usage

### Prerequisites

The following components required to use the helm chart:

- [Helm client](https://helm.sh/docs/intro/install/)
- [Kubernetes cluster](https://kubernetes.io/)

### How to install

To install o11y-collector in k8s cluster at least three parameters must be provided:
- `splunkRealm` (default `us0`): SignalFx realm to send telemetry data to.
- `splunkAccessToken`: Your SignalFx org access token.
- `clusterName`: arbitrary value that will identify your kubernetes cluster in SignalFx environment.

The project is in active development state. There are no packages released yet.
In order to install helm chart you need to clone the repo first and use it locally.

```bash
$ git clone git@github.com:signalfx/o11y-collector-for-kubernetes.git
$ cd ./o11y-collector-for-kubernetes
$ helm install my-o11y-collector --set="splunkRealm=us0,splunkAccessToken=xxxxxx,clusterName=my-cluster" ./helm-charts/o11y-collector-for-kubernetes
```

Instead of setting helm values as arguments a yaml file can be provided:

```bash
$ helm install my-o11y-collector --values my_values.yaml ./helm-charts/o11y-collector-for-kubernetes
```

### How to uninstall

To uninstall/delete a deployment with name `my-o11y-collector`:

```bash
$ helm delete my-o11y-collector
```

The command removes all the Kubernetes components associated with the chart and
deletes the release.

## Configuration

The [values.yaml](https://github.com/signalfx/o11y-collector-for-kubernetes/helm-charts/o11y-collector-for-kubernetes/values.yaml)
lists all supported configurable parameters for this chart, along with detailed explanation.
Read through it to understand how to configure this chart.

At the minimum you need to configure the following values.

```yaml
clusterName: my-k8s-cluster
splunkAccessToken: xxxxxx
splunkRealm: us0
```

### Kubernetes platform

Use `platform` parameter to provide information about underlying kubernetes platform.
It'll allow the o11y collector to automatically scrape additional cloud metadata. Supported options:
- `aws` - Amazon EKS or self-managed k8s cluster in AWS environment.
- `gcp` - Google GKE or self-managed k8s cluster in GCP environment.
- `default` - default configuration for other platforms.

## License ##

See [LICENSE](LICENSE).
