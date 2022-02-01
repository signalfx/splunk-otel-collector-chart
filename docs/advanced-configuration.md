# Advanced Configuration

The
[values.yaml](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/helm-charts/splunk-otel-collector/values.yaml)
lists all supported configurable parameters for this chart, along with detailed
explanation. Read through it to understand how to configure this chart.

Also check [examples of chart configuration](../examples/README.md). This also includes a guide to deploy for the k8s cluster with the windows worker node.

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

Use the `cloudProvider` parameter to provide information about the cloud
provider, if any.

- `aws` - Amazon Web Services
- `gcp` - Google Cloud
- `azure` - Microsoft Azure

This value can be omitted if none of the values apply.

## Kubernetes distribution

Use the `distribution` parameter to provide information about underlying
Kubernetes deployment. This parameter allows the connector to automatically
scrape additional metadata. The supported options are:

- `aks` - Azure AKS
- `eks` - Amazon EKS
- `eks/fargate` - Amazon EKS with Fargate profiles
- `gke` - Google GKE / Standard mode
- `gke/autopilot` - Google GKE / Autopilot mode
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

By default only metrics and traces are sent to Splunk Observability destination,
and only logs are sent to Splunk Platform destination. It's possible to enable
or disable any kind of telemetry for a specific destination. For example, with
the following configuration Splunk OTel Collector will send all collected
telemetry data to Splunk Observability and Splunk Platform assuming they are
both properly configured.

```yaml
splunkObservability:
  metricsEnabled: true
  tracesEnabled: true
  logsEnabled: true
splunkPlatform:
  metricsEnabled: true
  logsEnabled: true
```

## Windows worker nodes support

Splunk OpenTelemetry Collector for Kubernetes supports collection of metrics,
traces and logs (using OTel native logs collection only) from Windows nodes.

All windows images are available in a separate `quay.io` repository:
`quay.io/signalfx/splunk-otel-collector-windows`.

Use the following values.yaml configuration to install the helm chart on Windows
worker nodes:

```yaml
isWindows: true
image:
  otelcol:
    repository: quay.io/signalfx/splunk-otel-collector-windows
logsEngine: otel
readinessProbe:
  initialDelaySeconds: 60
livenessProbe:
  initialDelaySeconds: 60
```

If you have both Windows and Linux worker nodes in your Kubernetes cluster, you
need to install the helm chart twice. One of the installations with default
configuration `isWindows: false` will be applied on Linux nodes. Another
installation with values.yaml configuration that provided above will be applied
on Windows nodes. And it's important to disable `clusterReceiver` on one of the
installations to avoid cluster-wide metrics duplication, add the following line
to values.yaml of one of the installations:

```yaml
clusterReceiver:
  enabled: false
```

## GKE Autopilot support

If you want to run Splunk OTel Collector in [Google Kubernetes Engine
Autopilot](https://cloud.google.com/kubernetes-engine/docs/concepts/autopilot-overview),
make sure to set `distribution` setting to `gke/autopilot`:

```yaml
distribution: gke/autopilot
```

**NOTE:** Native OTel logs collection is not yet supported in GKE Autopilot.

Sometimes Splunk OTel Collector agent daemonset can have [problems scheduling in
Autopilot](https://cloud.google.com/kubernetes-engine/docs/concepts/daemonset#autopilot-ds-best-practices)
If you run into these issues, you can assign the daemonset a higher [priority
class](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/),
this will make sure that the daemonset pods are always present on each node:

1. Create a new priority class for Splunk OTel Collector agent:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: splunk-otel-agent-priority
value: 1000000
globalDefault: false
description: "Higher priority class for Splunk OpenTelemetry Collector pods."
EOF
```

2. Use the created priority class in the helm install/upgrade command:
with `--set="priorityClassName=splunk-otel-agent-priority"` cli argument or add
the following line to your custom values.yaml:

```yaml
priorityClassName: splunk-otel-agent-priority
```

## EKS Fargate support

If you want to run the Splunk OpenTelemetry Collector in [Amazon Elastic Kubernetes Service
with Fargate profiles](https://docs.aws.amazon.com/eks/latest/userguide/fargate.html),
make sure to set the required `distribution` value to `eks/fargate`:

```yaml
distribution: eks/fargate
```

**NOTE:** Fluentd and Native OTel logs collection are not yet automatically configured in EKS with Fargate profiles

This distribution will operate similarly to the `eks` distribution but with the following distinctions:

1. The Collector agent daemonset is not applied since Fargate doesn't support daemonsets. Any desired Collector instances
running as agents must be configured manually as sidecar containers in your custom deployments. This includes any application
logging services like Fluentd. We recommend setting the `gateway.enabled` to `true` and configuring your instrumented
applications to report metrics, traces, and logs to the gateway's `<installed-chart-name>-splunk-otel-collector` service address.
Any desired agent instances that would run as a daemonset should instead run as sidecar containers in your pods.

2. Since Fargate nodes use a VM boundary to prevent access to host-based resources used by other pods, pods are not able to reach their own kubelet. The cluster receiver
for the Fargate distribution has two primary differences between regular `eks` to work around this limitation:
    * The configured cluster receiver is deployed as a 2-replica StatefulSet instead of a Deployment and uses a
    [Kubernetes Observer extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/extension/observer/k8sobserver/README.md)
    that discovers the cluster's nodes and, on the second replica, its pods for user-configurable receiver creator additions. It uses this observer to dynamically create
    [Kubelet Stats receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/kubeletstatsreceiver/README.md)
    instances that will report kubelet metrics for all observed Fargate nodes. The first replica will monitor the cluster with a `k8s_cluster` receiver
    and the second will monitor all kubelets except its own (due to an EKS/Fargate networking restriction).

    * The first replica's collector will monitor the second's kubelet. This is made possible by a Fargate-specific `splunk-otel-eks-fargate-kubeletstats-receiver-node`
    node label. The Collector's ClusterRole for `eks/fargate` will allow the `patch` verb on `nodes` resources for the default API groups to allow the cluster
    receiver's init container to add this node label for designated self monitoring.

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
- Not yet supported in GKE Autopilot.

### Add log files from Kubernetes host machines/volumes

You can add additional log files to be ingested from Kubernetes host machines and Kubernetes volumes by configuring `agent.extraVolumes`, `agent.extraVolumeMounts` and `logsCollection.extraFileLogs` in the values.yaml file used to deploy Splunk OpenTelemetry Collector for Kubernetes.

Example of adding audit logs from Kubernetes host machines

```yaml
logsCollection:
  extraFileLogs:
    filelog/audit-log:
      include: [/var/log/kubernetes/apiserver/audit.log]
      start_at: beginning
      include_file_path: true
      include_file_name: false
      resource:
        com.splunk.source: /var/log/kubernetes/apiserver/audit.log
        host.name: 'EXPR(env("K8S_NODE_NAME"))'
        com.splunk.sourcetype: kube:apiserver-audit
agent:
  extraVolumeMounts:
    - name: audit-log
      mountPath: /var/log/kubernetes/apiserver
  extraVolumes:
    - name: audit-log
      hostPath:
        path: /var/log/kubernetes/apiserver
```

### Processing multi-line logs

Splunk OpenTelemetry Collector for Kubernetes supports parsing of multi-line logs to help read, understand, and troubleshoot the multi-line logs in a better way.
Process multi-line logs by configuring `logsCollection.containers.multilineConfigs` section in values.yaml.

```yaml
logsCollection:
  containers:
    multilineConfigs:
      - namespaceName:
          value: default
        podName:
          value: buttercup-app-.*
          useRegexp: true
        containerName:
          value: server
        firstEntryRegex: ^[^\s].*
```

Use https://regex101.com/ to find a golang regex that works for your format and specify it in the config file for the config option `firstEntryRegex`.


### Collect journald events

Splunk OpenTelemetry Collector for Kubernetes can collect journald events from kubernetes environment.
Process journald events by configuring `logsCollection.journald` section in values.yaml.

```yaml
logsCollection:
  journald:
    enabled: true
    directory: /run/log/journal
    # List of service units to collect and configuration for each. Please update the list as needed.
    units:
      - name: kubelet
        priority: info
      - name: docker
        priority: info
      - name: containerd
       priority: info
    # Route journald logs to its own Splunk Index by specifying the index value below, else leave it blank. Please make sure the index exist in Splunk and is configured to receive HEC traffic (Not applicable to Splunk Observability).
    index: ""
```

### Performance of native OpenTelemetry logs collection

Some configurations used with the OpenTelemetry Collector (as set using the Splunk OpenTelemetry Collector for Kubernetes helm chart) can have an impact on overall performance of log ingestion. The more receivers, processors, exporters, and extensions that are added to any of the pipelines, the greater the performance impact.

Splunk OpenTelemetry Collector for Kubernetes can exceed the default throughput of the The HTTP Event Collector (HEC). To best address capacity needs, monitor the HEC throughput and back pressure on Splunk OpenTelemetry Collector for Kubernetes deployments and be prepared to add additional nodes as needed.

Here is the summary of performance benchmarks run internally.
| Log Generator Count | Event Size (byte) | Agent CPU Usage | Agent EPS |
|---------------------|-------------------|-----------------|-----------|
|                   1 |               256 |             1.8 |    30,000 |
|                   1 |               516 |             1.8 |    28,000 |
|                   1 |              1024 |             1.8 |    24,000 |
|                   5 |               256 |             3.2 |    54,000 |
|                   7 |               256 |               3 |    52,000 |
|                  10 |               256 |             3.2 |    53,000 |

The data pipelines for these test runs involved reading container logs as they are being written, then parsing filename for metadata, enriching it with kubernetes metadata, reformatting data structure, and sending them (without compression) to Splunk HEC endpoint.

## Running the container in non-root user mode

Collecting logs often requires reading log files that are owned by the root user. By default, the container runs with `securityContext.runAsUser = 0` which gives the `root` user permission to read those files. To run the container in `non-root` user mode, set `.agent.securityContext` to `20000` to cause the container to run the required file system operations as UID and GID `20000`. (it can be any other UID & GUI)

Note: `cri-o` container runtime did not work during internal testing.

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

## Override underlying OpenTelemetry agent configuration

If you want to use your own OpenTelemetry Agent configuration, you can override it by providing a custom configuration in the `agent.config` parameter in the values.yaml, which will be merged into the default agent configuration, list parts of the configuration (for example, `service.pipelines.logs.processors`) to be fully re-defined.

### Override a control plane configuration

If your control plane is using non-standard ports or custom TLS certificates, then you can provide a custom
configuration so the otel-collector agent can still successfully connect to it.

To collect control plane metrics, we use a
[receiver creator](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/receivercreator/README.md)
that instantiates
[smartagent/{control_plane_component}](https://docs.splunk.com/observability/gdi/orchestration.html#nav-Orchestration)
receivers at runtime.

Below is an example configuration of how you could set up the agent to connect to an apiserver that is running on port
8443 (instead of the normal 443) and use custom TLS configurations.

```yaml
agent:
  config:
    receivers:
      receiver_creator:
        receivers:
          smartagent/kubernetes-apiserver:
            rule: type == "port" && port == 8443 && pod.labels["k8s-app"] == "kube-apiserver"
            config:
              clientCertPath: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
              clientKeyPath: /var/run/secrets/kubernetes.io/serviceaccount/token
              useServiceAccount: false
```
