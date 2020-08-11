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

### Metrics Collection

[OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector/) also collects kubernetes and host 
metrics using the following components enabled by default:
- [Kubeletstats receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/master/receiver/kubeletstatsreceiver) 
to collect metrics from Kubelet API.
- [K8s cluster receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/master/receiver/k8sclusterreceiver) 
to collect metrics from Kubernetes API.
- [Host metrics receiver](https://github.com/open-telemetry/opentelemetry-collector/tree/master/receiver/hostmetricsreceiver) 
to collect host metrics from kubernetes node.
Metrics are sent them to
[Splunk SignalFx Infrastructure Monitoring](https://www.splunk.com/en_us/software/infrastructure-monitoring.html).

## Install

See also [Using Helm](https://docs.helm.sh/using_helm/#using-helm).

To install o11y-collector in k8s cluster at least three parameters must be provided:
- `signalfx.realm` (default `us0`): SignalFx realm to send telemetry data to.
- `signalfx.accessToken`: Your SignalFx org access token.
- `clusterName`: arbitrary value that will identify your kubernetes cluster in SignalFx environment

The values can be provided as arguments to `helm install`:

```bash
$ helm install --name my-o11y-collector --set="signalfx.realm=us0,signalfx.accessToken=xxxxxx,clusterName=my-cluster" https://github.com/signalfx/o11y-collector-for-kubernetes/releases/download/0.1.0/o11y-collector-for-kubernetes-0.1.0.tgz
```

Or using by setting the values in a yaml file:

```bash
$ helm install --name my-o11y-collector --values my_values.yaml https://github.com/signalfx/o11y-collector-for-kubernetes/releases/download/0.1.0/o11y-collector-for-kubernetes-0.1.0.tgz
```

## Uninstall

To uninstall/delete a deployment with name `my-o11y-collector`:

```bash
$ helm delete --purge my-o11y-collector
```

The command removes all the Kubernetes components associated with the chart and
deletes the release.

## Configuration

The [values.yaml](https://github.com/signalfx/o11y-collector-for-kubernetes/helm-charts/o11y-collector-for-kubernetes/values.yaml) lists all supported configurable parameters for
this chart, along with detailed explanation. Read through it to understand how
to configure this chart.

At the minimum you need to configure the following values.

```yaml
clusterName: my-k8s-cluster
signalfx:
  accessToken: xxxxxx
  realm: us0
```

## License ##

See [LICENSE](LICENSE).
