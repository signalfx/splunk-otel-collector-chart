# Upgrade guidelines

## 0.119.0 to 0.120.0

This guide provides steps for new users, transitioning users, and those maintaining previously deployed Operator-related TLS certificates and configurations.

- New users: No migration is required for Operator TLS certificates.
- Previous users: Migration may be needed if using `operator.enabled=true` or `certmanager.enabled=true`.

To maintain previous functionality and avoid breaking changes, review the following sections.

### **Maintaining Previous Functionality via Helm Values Update**

#### **Scenario 1: Operator and cert-manager Deployed via This Helm Chart**

If you previously deployed both the Operator and cert-manager via this Helm chart (`operator.enabled=true` and `certmanager.enabled=true`), add the following values:

```yaml
operator:
  admissionWebhooks:
    certManager:
      enabled: true
      certificateAnnotations:
        "helm.sh/hook": post-install,post-upgrade
        "helm.sh/hook-weight": "1"
      issuerAnnotations:
        "helm.sh/hook": post-install,post-upgrade
        "helm.sh/hook-weight": "1"
certmanager:
  enabled: true
  installCRDs: true
```

#### **Scenario 2: Operator Deployed with External cert-manager (Not Managed by This Helm Chart)**

If you deployed the Operator via this Helm chart and used an externally managed cert-manager (`operator.enabled=true` and `certmanager.enabled=false`), you can preserve functionality by adding the following Helm values or using the `--reuse-values` argument:

```yaml
operator:
  admissionWebhooks:
    certManager:
      enabled: true
```

### **Adopting New Functionality (Requires Migration Steps)**

If you want to migrate from cert-manager-managed certificates to the now default Helm-generated certificates, additional steps may be required to avoid conflicts.

#### **Potential Upgrade Issue: Existing Secret Conflict**

If you see an error message like the following during Helm install/upgrade:

```
warning: Upgrade "{helm_release_name}" failed: pre-upgrade hooks failed: warning: Hook pre-upgrade splunk-otel-collector/charts/operator/templates/admission-webhooks/operator-webhook.yaml failed: 1 error occurred:* secrets "splunk-otel-collector-operator-controller-manager-service-cert" already exists
```

This typically occurs because:
- cert-manager deletes its `Certificate` resources immediately.
- However, cert-manager does not delete the associated **secrets** instantly. It waits for its garbage collector process to remove them.

You will first have to delete this chart, wait for cert-manager to do garbage collection, and then install the latest version of this chart.
With the assumption your Helm release is named "splunk-otel-collector", we show the commands to run below.
- `Be aware these steps likely include the operator being unavailable and having down time for this service in your environment.`

#### **Step 1: Verify If the Old Secret Still Exists**

Use a command like this to delete the chart in your namespace:

```bash
helm delete splunk-otel-collector --namespace <your_namespace>
```

#### **Step 2: Verify If the Old Cert Manager Secret Does Not Exists Anymore**

Use the following command to check if the certificate secret remains in your namespace:

```bash
kubectl get secret splunk-otel-collector-operator-controller-manager-service-cert --namespace <your_namespace>
```

#### **Step 3: Wait for Secret Removal or Manually Delete It**

If the secret still exists, you must wait for cert-manager to remove it or delete it manually:

```bash
kubectl delete secret splunk-otel-collector-operator-controller-manager-service-cert --namespace <your_namespace>
```

#### **Step 4: Proceed with Helm Install**

Once the secret is no longer present, you can install the chart with the latest version (`0.119.0`) successfully:

```bash
helm install splunk-otel-collector splunk-otel-collector-chart/splunk-otel-collector --values ~/values.yaml --namespace <your_namespace>
```

## 0.113.0 to 0.116.0

This guide provides steps for new users, transitioning users, and those maintaining previous operator CRD configurations:
- New users: No migration for CRDs is required.
- Previous users: Migration may be needed if using `operator.enabled=true`.

CRD deployment has evolved over chart versions:
- Before 0.110.0: CRDs were deployed via a crds/ directory (upstream default).
- 0.110.0 to 1.113.0: CRDs were deployed using Helm templates  (upstream default), which had reported issues.
- 0.116.0 and later:  Users must now explicitly configure their preferred CRD deployment method or deploy the
  [CRDs manually](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/docs/auto-instrumentation-install.md#crd-management)
  to avoid potential issues. Users can deploy CRDs via a crds/ directory again by enabling a newly added value.

### **New Users**

New users are advised to deploy CRDs via the `crds/` directory. For a fresh installation, use the following Helm values:

```yaml
operatorcrds:
  install: true
operator:
  enabled: true
```

To install the chart:

```bash
helm install <release-name> splunk-otel-collector-chart/splunk-otel-collector --set operatorcrds.install=true,operator.enabled=true <extra_args>
```

### **Current Users (Recommended Migration to `crds/` Directory)**

If you're using chart versions 0.110.0 to 1.113.0, CRDs are likely deployed via Helm templates. To migrate to the recommended `crds/` directory deployment:

#### Step 1: Delete the Existing Chart

Remove the chart to prepare for a fresh installation:

```bash
helm delete <release-name>
```

#### Step 2: Verify or Remove Existing CRDs

Check if the following CRDs are present and delete them if necessary:

```bash
kubectl get crds | grep opentelemetry
```

```bash
kubectl delete crd opentelemetrycollectors.opentelemetry.io
kubectl delete crd opampbridges.opentelemetry.io
kubectl delete crd instrumentations.opentelemetry.io
```

#### Step 3: Reinstall with Recommended Values

Reinstall the chart with the updated configuration:
```bash
helm install <release-name> splunk-otel-collector --set operatorcrds.install=true,operator.enabled=true <extra_args>
```

### **Previous Users (Maintaining Legacy Helm Templates)**

If you're using chart versions 0.110.0 to 1.113.0 and prefer to continue deploying CRDs via Helm templates (not recommended), you can do so with the following values:

```yaml
operator:
  enabled: true
operator:
  crds:
    create: true
```

**Warning**: This method may cause race conditions during installation or upgrades, leading to errors like:
```plaintext
ERROR: INSTALLATION FAILED: failed post-install: warning: Hook post-install splunk-otel-collector/templates/operator/instrumentation.yaml failed: 1 error occurred:
* Internal error occurred: failed calling webhook "minstrumentation.kb.io": failed to call webhook: Post "https://splunk-otel-collector-operator-webhook.default.svc:443/mutate-opentelemetry-io-v1alpha1-instrumentation?timeout=10s": dial tcp X.X.X.X:443: connect: connection refused
```

## 0.105.5 to 0.108.0

We've simplified the Helm chart configuration for `operator` auto-instrumentation.
The values previously under `.Values.operator.instrumentation.spec.*` have been moved to `.Values.instrumentation.*`.

- **No Action Needed**: If you have no customizations under `.Values.operator.instrumentation.spec.*`, no migration is required.
- **Action Required**: Continuing to use the old values path will result in a Helm install or upgrade error, blocking the process.

Migration Steps:

1. **Find** any references to `.Values.operator.instrumentation.spec.*` in your Helm values with custom values.
2. **Migrate** them from `.Values.operator.instrumentation.spec.*` to `.Values.instrumentation.*`.

Example Migration:

Before (Deprecated Path):

```yaml
operator:
  instrumentation:
    spec:
      endpoint: XXX
      ...
```

After (Updated Path):
```yaml
instrumentation:
  endpoint: XXX
  ...
```

## 0.105.3 to 0.105.4

The `Java instrumentation` for Operator auto-instrumentation has been upgraded from v1.32.2 to v2.7.0.
This major update introduces several breaking changes. Below we have supplied a customer migration
guide and outlined the key changes to highlight the impact.

Please refer to the [Migration guide for OpenTelemetry Java 2.x](https://docs.splunk.com/observability/en/gdi/get-data-in/application/java/migrate-metrics.html)
to update your custom dashboards, detectors, or alerts using Java application telemetry data.

### Breaking Changes Overview
- Runtime metrics will now be enabled by default, this can increase the number of metrics collected.
- The default protocol changed from gRPC to http/protobuf. For custom Java exporter endpoint
configurations, verify that you’re sending data to http/protobuf endpoints like this [example](https://github.com/signalfx/splunk-otel-collector-chart/blob/splunk-otel-collector-0.105.4/examples/enable-operator-and-auto-instrumentation/rendered_manifests/operator/instrumentation.yaml#L59).
- Span Attribute Name Changes:

| Old Attribute (1.x)           | New Attribute (2.x)           |
| ----------------------------- | ----------------------------- |
| http.method                   | http.request.method           |
| http.status_code              | http.response.status_code     |
| http.request_content_length   | http.request.body.size        |
| http.response_content_length  | http.response.body.size       |
| http.target                   | url.path and url.query        |
| http.scheme                   | url.scheme                    |
| http.client_ip                | client.address                |

- Metric Name Changes:

| Old Metric (1.x)                                                        | New Metric (2.x)                                     |
|-------------------------------------------------------------------------|------------------------------------------------------|
| db.pool.connections.create_time                                         | db.client.connections.create_time (Histogram, ms)    |
| db.pool.connections.idle.max                                            | db.client.connections.idle.max                       |
| db.pool.connections.idle.min                                            | db.client.connections.idle.min                       |
| db.pool.connections.max                                                 | db.client.connections.max                            |
| db.pool.connections.pending_threads                                     | db.client.connections.pending_requests               |
| db.pool.connections.timeouts                                            | db.client.connections.timeouts                       |
| db.pool.connections.idle                                                | db.client.connections.usage[state=idle]              |
| db.pool.connections.active                                              | db.client.connections.usage[state=used]              |
| db.pool.connections.use_time                                            | db.client.connections.use_time (Histogram, ms)       |
| db.pool.connections.wait_time                                           | db.client.connections.wait_time (Histogram, ms)      |
| runtime.jvm.buffer.count                                                | jvm.buffer.count                                     |
| runtime.jvm.buffer.total.capacity                                       | jvm.buffer.memory.limit                              |
| runtime.jvm.buffer.memory.used                                          | jvm.buffer.memory.usage                              |
| runtime.jvm.classes.loaded                                              | jvm.class.count                                      |
| runtime.jvm.classes.unloaded                                            | jvm.class.unloaded                                   |
| runtime.jvm.gc.concurrent.phase.time                                    | jvm.gc.duration (Histogram, <concurrent gcs>)        |
| runtime.jvm.gc.pause                                                    | jvm.gc.duration (<non-concurrent gcs>)               |
| runtime.jvm.gc.memory.allocated \| process.runtime.jvm.memory.allocated | jvm.memory.allocated*                                |
| runtime.jvm.memory.committed                                            | jvm.memory.committed                                 |
| runtime.jvm.memory.max                                                  | jvm.memory.limit                                     |
| runtime.jvm.gc.max.data.size                                            | jvm.memory.limit{jvm.memory.pool.name=<long lived>}  |
| runtime.jvm.memory.used                                                 | jvm.memory.used                                      |
| runtime.jvm.gc.live.data.size                                           | jvm.memory.used_after_last_gc{jvm.memory.pool.name=} |
| runtime.jvm.threads.daemon \| runtime.jvm.threads.live                  | jvm.thread.count                                     |

- Dropped Metrics:
  - executor.tasks.completed
  - executor.tasks.submitted
  - executor.threads
  - executor.threads.active
  - executor.threads.core
  - executor.threads.idle
  - executor.threads.max
  - runtime.jvm.memory.usage.after.gc
  - runtime.jvm.gc.memory.promoted
  - runtime.jvm.gc.overhead
  - runtime.jvm.threads.peak
  - runtime.jvm.threads.states

# 0.93.0 to 0.94.0

The `networkExplorer` option is removed.

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
