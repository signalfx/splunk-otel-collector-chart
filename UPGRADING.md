# Upgrade guidelines

## 0.87.0 to 0.88.0

The `networkExplorer` option is deprecated now. Please use the upstream OpenTelemetry eBPF Helm chart to collect
the network metrics by following the next steps:

1. Make sure the Splunk OpenTelemetry Collector helm chart is installed with the gateway enabled:

```yaml
gateway:
  enabled: true
```

2. Disable the network explorer:

```yaml
networkExplorer:
  enabled: false
```

3. Grab name of the Splunk OpenTelemetry Collector gateway service:

```bash
kubectl get svc | grep splunk-otel-collector-gateway
```

4. Install the upstream OpenTelemetry eBPF helm chart pointing to the Splunk OpenTelemetry Collector gateway service:

```bash
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update open-telemetry
helm install my-opentelemetry-ebpf -f ./otel-ebpf-values.yaml open-telemetry/opentelemetry-ebpf
```

`otel-ebpf-values.yaml` must at least have `endpoint.address` option set to the Splunk OpenTelemetry
Collector gateway service name captured in the step 2. Additionally, if you had any custom confgurations in the
`networkExplorer` section, you need to move them to the `otel-ebpf-values.yaml` file.

```yaml
endpoint:
  address: <my-splunk-otel-collector-gateway>

# additional custom configuration moved from the networkExplorer section in Splunk OpenTelemetry Collector helm chart.
```

## 0.85.0 to 0.86.0

The default logs collection engine (`logsEngine`) changed from `fluentd` to the native OpenTelemetry logs collection (`otel`).
If you want to keep using Fluentd sidecar for the logs collection, set `logsEngine: fluentd` in your values.yaml.

## 0.84.0 to 0.85.0

The format for defining auto-instrumentation images has been refactored. Previously, the image was
defined using the `operator.instrumentation.spec.{library}.image` format. This has been changed to
separate the repository and tag into two distinct fields: `operator.instrumentation.spec.{library}.repository`
and `operator.instrumentation.spec.{library}.tag`.

If you were defining a custom image under  `operator.instrumentation.spec.{library}.image`, update
your `values.yaml` to accommodate this change.

- Before:

```yaml
operator:
  instrumentation:
    spec:
      java:
        image: ghcr.io/custom-owner/splunk-otel-java/custom-splunk-otel-java:v1.27.0
```

- After:

```yaml
operator:
  instrumentation:
    spec:
      java:
        repository: ghcr.io/custom-owner/splunk-otel-java/custom-splunk-otel-java
        tag: v1.27.0
```

## 0.67.0 to 0.68.0

There is a new receiver: [Kubernetes Objects Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sobjectsreceiver) that can pull or watch any object from Kubernetes API server.
It will replace the [Kubernetes Events Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8seventsreceiver) in the future.

To migrate from Kubernetes Events Receiver to Kubernetes Object Receiver, configure `clusterReceiver` values.yaml section with:

```yaml
k8sObjects:
  - mode: watch
    name: events
```

There are differences in the log record formatting between the previous `k8s_events` receiver and the now adopted `k8sobjects` receiver results.
The `k8s_events` receiver stores event messages their log body, with the following fields added as attributes:

* `k8s.object.kind`
* `k8s.object.name`
* `k8s.object.uid`
* `k8s.object.fieldpath`
* `k8s.object.api_version`
* `k8s.object.resource_version`
* `k8s.event.reason`
* `k8s.event.action`
* `k8s.event.start_time`
* `k8s.event.name`
* `k8s.event.uid`
* `k8s.namespace.name`

Now with the `k8sobjects` receiver, the whole payload is stored in the log body and `object.message` refers to the event message.


You can monitor more Kubernetes objects configuring by `clusterReceiver.k8sObjects` according to the instructions from the
[Kubernetes Objects Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sobjectsreceiver) documentation.

Remember to define `rbac.customRules` when needed. For example, when configuring:

```yaml
objectsEnabled: true
k8sObjects:
  - name: events
    mode: watch
    group: events.k8s.io
    namespaces: [default]
```

You should add `events.k8s.io` API group to the `rbac.customRules`:

```yaml
rbac:
  customRules:
    - apiGroups:
      - "events.k8s.io"
      resources:
      - events
      verbs:
      - get
      - list
      - watch
```

## 0.58.0 to 0.59.0
[receiver/filelogreceiver] Datatype for `force_flush_period` and `poll_interval` were changed from map to string.

If you are using custom filelog receiver plugin, you need to change the config from:
```yaml
filelog:
  poll_interval:
    duration: 200ms
  force_flush_period:
    duration: "0"
```
to:
```yaml
filelog:
  poll_interval: 200ms
  force_flush_period: "0"
```

## 0.57.1 to 0.58.0
[receiver/filelogreceiver] Datatype for `force_flush_period` and `poll_interval` were changed from
sring to map. Because of that, the default values in Helm Chart were causing problems [#519](https://github.com/signalfx/splunk-otel-collector-chart/issues/519)

If you are using custom filelog receiver plugin, you need to change the config from:
```yaml
filelog:
  poll_interval: 200ms
  force_flush_period: "0"
```
to:
```yaml
filelog:
  poll_interval:
    duration: 200ms
  force_flush_period:
    duration: "0"
```

## 0.54.0 to 0.55.0

[[receiver/k8sclusterreceiver] The receiver.k8sclusterreceiver.reportCpuMetricsAsDouble feature gate has been removed](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/10838)

If you are disabling this feature gate to keep previous functionality, you will
have to complete the steps in
[upgrade guidelines 0.47.0 to 0.47.1](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0470-to-0471)
to upgrade since the feature gate no longer exists.

## 0.53.2 to 0.54.0

[OTel Kubernetes receiver is now used for events collection instead of Signalfx events receiver](https://github.com/signalfx/splunk-otel-collector-chart/pull/478)

Before this change, if `clusterReceiver.k8sEventsEnabled=true`, Kubernetes events used to be collected by a Signalfx
receiver and sent both to Splunk Observability Infrastructure Monitoring and Splunk Observability Log Observer.
Now we utilize [a native OpenTelemetry receiver for collecting Kubernetes
events](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8seventsreceiver).
Therefore `clusterReceiver.k8sEventsEnabled` option is now deprecated and replaced by the following two options:
- `clusterReceiver.eventsEnabled`: to send Kubernetes events in the new OTel format to Splunk Observability Log Observer
(if splunkObservability.logsEnabled=true) or to Splunk Platform (if splunkPlatform.logsEnabled=true).
- `splunkObservability.infrastructureMonitoringEventsEnabled`: to collect Kubernetes events using the Signalfx
Kubernetes events receiver and send them to Splunk Observability Infrastructure Monitoring.

If you have `clusterReceiver.k8sEventsEnabled` set to `true` to send Kubernetes events to both Splunk Observability
Infrastructure Monitoring and Splunk Observability Log Observer, remove `clusterReceiver.k8sEventsEnabled` from your
custom values.yaml enable both `clusterReceiver.eventsEnabled` and
`splunkObservability.infrastructureMonitoringEventsEnabled` options. This will send the Kubernetes events to Splunk
Observability Log Observer in the new OpenTelemetry format.

If you want to keep sending Kubernetes events to Splunk Observability Log Observer in the old Signalfx format to keep
exactly the same behavior as before, remove `clusterReceiver.k8sEventsEnabled` from your custom values.yaml and add the
following configuration:

```yaml
splunkObservability:
  logsEnabled: true
  infrastructureMonitoringEventsEnabled: true
clusterReceiver:
  config:
    exporters:
      splunk_hec/events:
        endpoint: https://ingest.<SPLUNK_OBSERVABILITY_REALM>.signalfx.com/v1/log
        log_data_enabled: true
        profiling_data_enabled: false
        source: kubelet
        sourcetype: kube:events
        token: ${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}
    service:
      pipelines:
        logs/events:
          exporters:
            - signalfx
            - splunk_hec/events
```

where `SPLUNK_OBSERVABILITY_REALM` must be replaced by `splunkObservability.realm` value.

## 0.48.0 to 0.49.0

New releases of opentelemetry-log-collection (
[v0.29.0](https://github.com/open-telemetry/opentelemetry-log-collection/blob/v0.29.0/CHANGELOG.md#0290---2022-04-11),
[v0.28.0](https://github.com/open-telemetry/opentelemetry-log-collection/blob/v0.29.0/CHANGELOG.md#0280---2022-03-28)
) have breaking changes

Several of the logging receivers supported by the Splunk Otel Collector Chart
were updated to use v0.29.0 instead v0.27.2 of opentelemetry-log-collection.

- Affected Receivers
  - [filelog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver)
  - [syslog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/syslogreceiver)
  - [tcplog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/tcplogreceiver)
  - [journald](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/journaldreceiver)

1) Check to see if you have any custom log monitoring setup with the
[extraFileLogs](https://github.com/signalfx/splunk-otel-collector-chart/blob/941ad7f255cce585f4c06dd46c0cd63ef57d9903/helm-charts/splunk-otel-collector/values.yaml#L488)
config, the
[logsCollection.containers.extraOperators](https://github.com/signalfx/splunk-otel-collector-chart/blob/941ad7f255cce585f4c06dd46c0cd63ef57d9903/helm-charts/splunk-otel-collector/values.yaml#L431)
config, or any of the affected receivers. If you don't have any custom log
monitoring setup, you can stop here.
2) Read the documentation for
[upgrading to opentelemetry-log-collection v0.29.0](https://github.com/open-telemetry/opentelemetry-log-collection/blob/v0.29.0/CHANGELOG.md#upgrading-to-v0290).
3) If opentelemetry-log-collection v0.29.0 or v0.28.0 will break any of your
custom log monitoring, update your log monitoring to accommodate the breaking
changes.

[[receiver/k8sclusterreceiver] The receiver.k8sclusterreceiver.reportCpuMetricsAsDouble feature gate is now enabled by default](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/9367)

If you haven't already completed the steps in
[upgrade guidelines 0.47.0 to 0.47.1](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0470-to-0471)
, then complete them.

## 0.47.0 to 0.47.1
[[receiver/k8sclusterreceiver] Fix k8s node and container cpu metrics not being reported properly](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/8245)

The Splunk Otel Collector added a feature gate to enable a bug fix for three
metrics. These metrics have a current and a legacy name, we list both as
pairs (current, legacy) below.

- Affected Metrics
  - `k8s.container.cpu_request`, `kubernetes.container_cpu_request`
  - `k8s.container.cpu_limit`, `kubernetes.container_cpu_limit`
  - `k8s.node.allocatable_cpu`, `kubernetes.node_allocatable_cpu`
- Upgrade Steps
  1. Check to see if any of your custom monitoring uses the affected metrics.
  Check for the current and legacy names of the affected metrics. If you don't
  use the affected metrics in your custom monitoring, you can stop here.
  2. Read the documentation for the
  [receiver.k8sclusterreceiver.reportCpuMetricsAsDouble](https://github.com/signalfx/splunk-otel-collector-chart/tree/splunk-otel-collector-0.54.0/docs/advanced-configuration.md#highlighted-feature-gates)
  feature gate and the bug fix it applies.
  3. If the bug fix will break any of your custom monitoring for the affected
  metrics, update your monitoring to accommodate the bug fix.
- Feature Gate Stages and Versions
  - Alpha (versions 0.47.1-0.48.0):
    - The feature gate is disabled by default. Use the `--set clusterReceiver.featureGates=receiver.k8sclusterreceiver.reportCpuMetricsAsDouble`
      argument with the helm install/upgrade command, or add the following line to
      your custom values.yaml to enable the feature gate:
    ```yaml
    clusterReceiver:
      featureGates: receiver.k8sclusterreceiver.reportCpuMetricsAsDouble
    ```
  - Beta (versions 0.49.0-0.54.0):
    - The feature gate is enabled by default. Use the `--set clusterReceiver.featureGates=-receiver.k8sclusterreceiver.reportCpuMetricsAsDouble`
      argument with the helm install/upgrade command, or add the following line to
      your custom values.yaml to disable the feature gate:
    ```yaml
    clusterReceiver:
      featureGates: -receiver.k8sclusterreceiver.reportCpuMetricsAsDouble
    ```
  - Generally Available (versions +0.55.0):
    - The receiver.k8sclusterreceiver.reportCpuMetricsAsDouble feature gate
      functionality is permanently enabled and the feature gate is no longer available
      for anyone.

## 0.44.1 to 0.45.0

[[receiver/k8sclusterreceiver] Use newer batch and autoscaling APIs](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/7406)

Kubernetes clusters with version 1.20 stopped having active support on
2021-12-28 and had an end of life date on
[2022-02-28](https://kubernetes.io/releases/patch-releases/#non-active-branch-history).
The k8s_cluster receiver was refactored to use newer Kubernetes APIs that
are available starting in Kubernetes version 1.21. The latest version of the
k8s_cluster receiver will no longer be able to collect all the
[previously available metrics](https://docs.splunk.com/Observability/gdi/kubernetes-cluster/kubernetes-cluster-receiver.html#metrics)
with Kubernetes clusters that have versions below 1.21.

If version 0.45.0 of the chart cannot collect metrics from your Kubernetes
cluster that is a version below 1.21, you will see error messages in your
cluster receiver logs that look like this.

`Failed to watch *v1.CronJob: failed to list *v1.CronJob: the server could not
find the requested resource`

To better support users, in a future release we are adding a feature that
will allow users to use the last version of the k8s_cluster receiver that
supported Kubernetes clusters below version 1.21.

If you still want to keep the previous behavior of the k8s_cluster receiver and
upgrade to v0.45.0 of the chart, make sure your Kubernetes cluster uses one of
the following versions.
- `kubernetes`, `aks`, `eks`, `eks/fargate`, `gke`,  `gke/autopilot`
  - Use version 1.21 or above
- `openshift`
  - Use version 4.8 or above

## 0.43.1 to 0.43.2

[#375 Resource detection processor is configured to override all host and cloud
attributes](https://github.com/signalfx/splunk-otel-collector-chart/pull/375)

If you still want to keep the previous behavior, use the following custom
values.yaml configuration:

```yaml
agent:
  config:
    processors:
      resourcedetection:
        override: false
```

## 0.41.0 to 0.42.0

[#357 Double expansion issue in splunk-otel-collector is
fixed](https://github.com/signalfx/splunk-otel-collector-chart/pull/357)

If you use OTel native logs collection with any custom log processing operators
in `filelog` receiver, please replace any occurrences of `$$$$` with `$$`.

## 0.38.0 to 0.39.0

[#325 Logs collection is now disabled by default for Splunk Observability
destination](https://github.com/signalfx/splunk-otel-collector-chart/pull/325)

If you send logs to Splunk Observability destination, make sure to enable logs.
Use `--set="splunkObservability.logsEnabled=true"` argument with helm
install/upgrade command, or add the following line to your custom values.yaml:

```yaml
splunkObservability:
  logsEnabled: true
```

## 0.37.1 to 0.38.0

[#297](https://github.com/signalfx/splunk-otel-collector-chart/pull/297),
[#301](https://github.com/signalfx/splunk-otel-collector-chart/pull/301) Several
parameters in values.yaml configuration were renamed according to [Splunk GDI
Specification](https://github.com/signalfx/gdi-specification/blob/main/specification/configuration.md#kubernetes-package-management-solutions)

If you use the following parameters in your custom values.yaml, please rename
them accordingly:
- `provider` -> `cloudProvider`
- `distro` -> `distribution`
- `otelAgent` -> `agent`
- `otelCollector` -> `gateway`
- `otelK8sClusterReceiver` -> `clusterReceiver`

[#306 Some parameters under `splunkPlatform` group were
renamed](https://github.com/signalfx/splunk-otel-collector-chart/pull/306)

If you use the following parameters under `splunkPlatform` group, please make
sure they are updated:
- `metrics_index` -> `metricsIndex`
- `max_connections` -> `maxConnections`
- `disable_compression` -> `disableCompression`
- `insecure_skip_verify` -> `insecureSkipVerify`

[#295 Secret names are changed according to the GDI
specification](https://github.com/signalfx/splunk-otel-collector-chart/pull/295)

If you provide access token for Splunk Observability using a custom Kubernetes
secret (secter.create=false), please update the secret key from
`splunk_o11y_access_token` to `splunk_observability_access_token`

[#273 Changed configuration to fetch attributes from labels and annotations of pods and namespaces](https://github.com/signalfx/splunk-otel-collector-chart/pull/273)

`podLabels` parameter under the `extraAttributes` group is now deprecated.
in favor of `fromLabels`. Please update your custom values.yaml accordingly.

For example, the following config:

```yaml
extraAttributes:
  podLabels:
    - app
    - git_sha
```

Should be changed to:

```yaml
extraAttributes:
  fromLabels:
    - key: app
    - key: git_sha
```

[#316 Busybox dependency is removed, splunk/fluentd-hec image is used in init container
instead](https://github.com/signalfx/splunk-otel-collector-chart/pull/316)

`image.fluentd.initContainer` is not being used anymore. Please remove it from
your custom values.yaml.

## 0.36.2 to 0.37.0

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
agent:
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

## 0.35.3 to 0.36.0

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

## 0.26.4 to 0.27.0

[#163 Auto-detection of prometheus metrics is disabled by default](https://github.com/signalfx/splunk-otel-collector-chart/pull/163):
If you rely on automatic prometheus endpoints detection to scrape prometheus
metrics from pods in your k8s cluster, make sure to add this configuration to
your values.yaml:

```yaml
autodetect:
  prometheus: true
```
