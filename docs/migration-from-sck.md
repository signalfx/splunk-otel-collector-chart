# Migration from Splunk Connect for Kubernetes

## Assumptions

You are running Splunk Connect for Kubernetes (SCK) 1.4.9 and want to migrate to Splunk OpenTelemetry Collector for Kubernetes. Previous versions of the SCK are out of scope, but this guide still applies. You might encounter some version specific issues (such as missing/changed config options) but that shouldn't prevent you from proceeding with migration.

## Changes in components

SCK has 3 components/applications:

1. Logs, metrics, and traces from a Kubernetes cluster (deployed as a DaemonSet)
2. Application to fetch cluster metrics from a Kubernetes cluster (deployed as a deployment)
3. Application to fetch Kubernetes objects metadata from a Kubernetes cluster (deployed as a deployment)

All SCK applications use Fluentd to work with logs, metrics, and objects. Fluentd has significant performance issues when used to fetch logs in a Kubernetes cluster with pods that have very high throughput.

Splunk OpenTelemetry Collector for Kubernetes provides significant performance improvements over the SCK through use of the OpenTelemetry Collector agent and native OpenTelemetry functionality for logs collection rather than Fluentd. See [Performance of native OpenTelemetry logs collection](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/docs/advanced-configuration.md#performance-of-native-opentelemetry-logs-collection) to learn more about the performance characteristics of this new application.

Splunk OpenTelemetry Collector for Kubernetes has the following components and applications:

1. Splunk OpenTelemetry Collector Agent (`agent`) to fetch logs, metrics, and traces from a Kubernetes cluster (deployed as a Kubernetes DaemonSet)
2. Splunk OpenTelemetry Collector Cluster Receiver (`clusterReceiver`) to fetch metrics from a Kubernetes API (deployed as a Kubernetes 1-replica Deployment)
3. Optional Splunk OpenTelemetry Collector Gateway (`gateway`) to forward data through it to reduce load on Kubernetes API and apply additional processing (deployed as a Kubernetes Deployment)

### Changes in logs in Splunk OpenTelemetry Collector for Kubernetes

* Red Hat Universal Base Image (UBI) Docker images for our applications are no longer available, as we now use scratch images.
* AWS FireLens is not supported.

### Changes in metrics in Splunk OpenTelemetry Collector for Kubernetes

* The naming convention of the metrics used in Splunk OpenTelemetry Collector for Kubernetes follows the OpenTelemetry specification and is different than SCK. You will observe minor differences in the names of the metrics.
* Previously in SCK, you could get a large number of metrics from various APIs for Kubernetes. However, in recent versions of Kubernetes 1.18+, these API sources are disabled by default and there are fewer metrics available. See [Metrics Information](https://github.com/splunk/fluent-plugin-kubernetes-metrics/blob/master/metrics-information.md) for the previous list of metrics.

Some additional metrics that are available in Splunk OpenTelemetry Collector for Kubernetes are metrics about the OpenTelemetry Collector itself.

These are the metrics available in Splunk OpenTelemetry Collector for Kubernetes:

* container.cpu.time
* container.cpu.utilization
* container.filesystem.available
* container.filesystem.capacity
* container.filesystem.usage
* container.memory.available
* container.memory.major_page_faults
* container.memory.page_faults
* container.memory.rss
* container.memory.usage
* container.memory.working_set
* k8s.container.cpu_limit
* k8s.container.cpu_request
* k8s.container.memory_limit
* k8s.container.memory_request
* k8s.container.ready
* k8s.container.restarts
* k8s.daemonset.current_scheduled_nodes
* k8s.daemonset.desired_scheduled_nodes
* k8s.daemonset.misscheduled_nodes
* k8s.daemonset.ready_nodes
* k8s.deployment.available
* k8s.deployment.desired
* k8s.namespace.phase
* k8s.node.condition_ready
* k8s.node.cpu.time
* k8s.node.cpu.utilization
* k8s.node.filesystem.available
* k8s.node.filesystem.capacity
* k8s.node.filesystem.usage
* k8s.node.memory.available
* k8s.node.memory.major_page_faults
* k8s.node.memory.page_faults
* k8s.node.memory.rss
* k8s.node.memory.usage
* k8s.node.memory.working_set
* k8s.node.network.errors
* k8s.node.network.io
* k8s.pod.cpu.time
* k8s.pod.cpu.utilization
* k8s.pod.filesystem.available
* k8s.pod.filesystem.capacity
* k8s.pod.filesystem.usage
* k8s.pod.memory.available
* k8s.pod.memory.major_page_faults
* k8s.pod.memory.page_faults
* k8s.pod.memory.rss
* k8s.pod.memory.usage
* k8s.pod.memory.working_set
* k8s.pod.network.errors
* k8s.pod.network.io
* k8s.pod.phase
* k8s.replicaset.available
* k8s.replicaset.desired
* otelcol_exporter_queue_size
* otelcol_exporter_send_failed_log_records
* otelcol_exporter_send_failed_metric_points
* otelcol_exporter_sent_log_records
* otelcol_exporter_sent_metric_points
* otelcol_otelsvc_k8s_ip_lookup_miss
* otelcol_otelsvc_k8s_namespace_added
* otelcol_otelsvc_k8s_namespace_updated
* otelcol_otelsvc_k8s_pod_added
* otelcol_otelsvc_k8s_pod_table_size
* otelcol_otelsvc_k8s_pod_updated
* otelcol_process_cpu_seconds
* otelcol_process_memory_rss
* otelcol_process_runtime_heap_alloc_bytes
* otelcol_process_runtime_total_alloc_bytes
* otelcol_process_runtime_total_sys_memory_bytes
* otelcol_process_uptime
* otelcol_processor_accepted_log_records
* otelcol_processor_accepted_metric_points
* otelcol_processor_batch_batch_send_size_bucket
* otelcol_processor_batch_batch_send_size_count
* otelcol_processor_batch_batch_send_size_sum
* otelcol_processor_batch_timeout_trigger_send
* otelcol_processor_dropped_log_records
* otelcol_processor_dropped_metric_points
* otelcol_processor_refused_log_records
* otelcol_processor_refused_metric_points
* otelcol_receiver_accepted_metric_points
* otelcol_receiver_refused_metric_points
* otelcol_scraper_errored_metric_points
* otelcol_scraper_scraped_metric_points
* scrape_duration_seconds
* scrape_samples_post_metric_relabeling
* scrape_samples_scraped
* scrape_series_added
* system.cpu.load_average.15m
* system.cpu.load_average.1m
* system.cpu.load_average.5m
* system.cpu.time
* system.disk.io
* system.disk.io_time
* system.disk.merged
* system.disk.operation_time
* system.disk.operations
* system.disk.pending_operations
* system.disk.weighted_io_time
* system.filesystem.inodes.usage
* system.filesystem.usage
* system.memory.usage
* system.network.connections
* system.network.dropped
* system.network.errors
* system.network.io
* system.network.packets
* system.paging.faults
* system.paging.operations
* system.paging.usage
* system.processes.count
* system.processes.created
* up


## Migration overview

### Migration options/matrix

The following table shows the options for migrating from SCK to Splunk OpenTelemetry Collector for Kubernetes:

| Method  | Logs | Metrics | Objects |
|---|---|---|---|
| SCK | Yes | Yes | Yes |
| Splunk OpenTelemetry Collector for Kubernetes | Yes | Yes | Yes |

As shown in the table, you can acquire logs, metrics and objects using Splunk OpenTelemetry Collector for Kubernetes.

### Checkpoint translation

With migration to the Splunk OpenTelemetry Collector for Kubernetes, the underlying framework/agent being used has changed, so there needs to be a translation for checkpoint data so that the new OpenTelemetry agent can continue where Fluentd left off. All of this occurs automatically as an initContainer when you deploy the new Helm chart in your cluster for the first time. If you are not using a custom path for checkpoint for fluentd, it should just work with default configurations for this new Helm chart.

To migrate Fluentd's position files again:

1. Stop the OpenTelemetry DaemonSet by deleting deployed Helm chart.
2. Delete the OpenTelemetry checkpoint files in the ```"/var/addon/splunk/otel_pos/"``` directory from Kubernetes nodes.
3. Restart the new Helm chart DaemonSet.

## Step 1: Preparing your values.yaml file for migration

Translate the values.yaml file from SCK to an appropriate format for Splunk OpenTelemetry Collector for Kubernetes. The following are the configurations for SCK and Splunk OpenTelemetry Collector for Kubernetes:

* [SCK configuration](https://github.com/splunk/splunk-connect-for-kubernetes/blob/develop/helm-chart/splunk-connect-for-kubernetes/values.yaml)
* [Splunk OpenTelemetry Collector for Kubernetes configuration](https://github.com/andrewy-splunk/splunk-otel-collector-chart/blob/main/helm-charts/splunk-otel-collector/values.yaml)

### Translating global/generic/splunk configurations from SCK to Splunk OpenTelemetry Collector for Kubernetes

#### Specifying your Splunk Platform and HTTP Event Collector (HEC) configuration

You can combine the `host`, `port` and `protocol` options from SCK to use the `splunkPlatform.endpoint` option in Splunk OpenTelemetry Collector for Kubernetes.

This option uses this format: "http://X.X.X.X:8088/services/collector" which is interpreted as "protocol://host:port/services/collector".

If you are using the `clientCert`, `clientKey`, and `caFile` options from SCK, use the corresponding `clientCert`, `clientKey`, and `caFile` options under `splunkPlatform` in Splunk OpenTelemetry Collector for Kubernetes to specify your HEC certificate chain.

If you are using the `insecureSSL` option from SCK, use the `splunkPlatform.insecureSkipVerify` option in Splunk OpenTelemetry Collector for Kubernetes to specify whether to verify the certificates on HEC.

If you are using the `indexName` option from SCK, use the `splunkPlatform.index` option in Splunk OpenTelemetry Collector for Kubernetes to specify which index you want to index data into

## Translating custom configurations from SCK to Splunk OpenTelemetry Collector for Kubernetes for logs

You can find all the configuration options used to upgrade in the values.yaml files linked above for both SCK and Splunk OpenTelemetry Collector for Kubernetes.

#### Container runtimes

If you configured SCK to explicitly set any of the following, you can do the same with Splunk OpenTelemetry Collector for Kubernetes:

* Set the `logsCollection.containers.containerRuntime` to a value from CRI-O, containerd, or Docker, depending on your runtime.
* Set the `path` option to the location of your logs on your nodes.
* If using the `exclude_path` option from SCK, you can use the `logsCollection.containers.excludePaths` option in Splunk OpenTelemetry Collector for Kubernetes to exclude any logs you want.
* If using the `logs` option to define `multiline` configs in SCK, you can use the `logsCollection.containers.multilineConfigs` option in Splunk OpenTelemetry Collector for Kubernetes to concatenate multiline logs.
* If using the `checkpointFile` option in SCK to define a custom location for checkpointing your logs, you can also do so with the `logsCollection.checkpointPath` option in Splunk OpenTelemetry Collector for Kubernetes.
* If using the `log` option to ingest any other logs from your nodes, you can also do so with the `logsCollection.extraFileLogs` option in Splunk OpenTelemetry Collector for Kubernetes.

#### Cluster name

If you are using the `clusterName` option to set a cluster name metadata field in your logs in SCK, you can also use the `clusterName` option in Splunk OpenTelemetry Collector for Kubernetes.

#### Pod security context

If you are using the `podSecurityContext` option to set a pod security policy, you can use the `agent.securityContext` option in Splunk OpenTelemetry Collector for Kubernetes.

#### Custom metadata

If you are using the `customMetadata` option to set a custom metadata field in your logs in SCK, you can use the `extraAttributes.custom` option in Splunk OpenTelemetry Collector for Kubernetes.

#### Custom annotations

If you are using the `customMetadataAnnotations` option to set custom annotation fields in your logs in SCK, you can use the `extraAttributes.fromAnnotations` option in Splunk OpenTelemetry Collector for Kubernetes.

#### Pod scheduling configurations

If you are using any pod scheduling operations such as `nodeSelector`, `affinity`, and `tolerations` in SCK, you can use the `nodeSelector`, `affinity` and `tolerations` options in Splunk OpenTelemetry Collector for Kubernetes.

#### Service accounts

If you are using the `serviceAccount` option to use your own service accounts in SCK, you can use the `serviceAccount` option in Splunk OpenTelemetry Collector for Kubernetes.

#### Secrets

If you are using the `secret` option to use your own service accounts in SCK, you can use the `secret` option in Splunk OpenTelemetry Collector for Kubernetes.

#### Custom Docker image and pull secrets

If you are using custom Docker images, tags, pull secrets, and pull policy in SCK, you can achieve the same using the `image` option in Splunk OpenTelemetry Collector for Kubernetes to specify the relevant configs for using docker images.

#### Limiting or increasing application resources

If you are using the `resources` option in SCK to limit/increase the CPU and memory usage in SCK, you can do the same using the `agent.resources` option in Splunk OpenTelemetry Collector for Kubernetes.

#### Extra files from host

For tailing files other than container or journald logs (that is, kube audit logs), configure `logsCollection.extraFileLogs` using this [filelog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver) receiver configuration.

[SCK values.yaml snippet]

```yaml
logs:
  kube-audit:
    from:
      file:
        path: /var/log/kube-apiserver-audit.log
    timestampExtraction:
      format: "%Y-%m-%dT%H:%M:%SZ"
    sourcetype: kube:apiserver-audit
```

[Splunk OpenTelemetry Collector for Kubernetes values.yaml snippet]

```yaml
logsCollection:
  extraFileLogs:
    filelog/kube-audit:
      include: [/var/log/kube-apiserver-audit.log]
      start_at: beginning
      include_file_path: true
      include_file_name: false
      storage: file_storage
      resource:
        host.name: 'EXPR(env("K8S_NODE_NAME"))'
        com.splunk.sourcetype: kube:apiserver-audit
        com.splunk.source: /var/log/kube-apiserver-audit.log
```

Use the `kube-audit` keyword to continue reading from the translated checkpoint data.

### Translating custom configurations from SCK to Splunk OpenTelemetry Collector for Kubernetes for objects

For collecting Kubernetes objects, configure `clusterReceiver.k8sObjects` using the [k8sobjects](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sobjectsreceiver) receiver configurations.

If you are using the following values for objects in `splunk-kubernetes-objects`:

[SCK values.yaml snippet]

```yaml
splunk-kubernetes-objects:
objects:
  core:
    v1:
      - name: pods
        namespace: default
        mode: pull
        interval: 60m
      - name: events
        mode: watch
  apps:
    v1:
      - name: daemon_sets
        labelSelector: environment=production
```

Equivalent configuration for Splunk OpenTelemetry Collector for Kubernetes:

```yaml
clusterReceiver:
  k8sObjects:
    - name: pods
      namespaces: [default]
      mode: pull
      interval: 60m
    - name: events
      mode: watch
    - name: daemonsets
      label_selector: environment=production
```

[k8sobjects](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sobjectsreceiver) pulls objects every 60m by default. But, SCK pulls at every 15m. If you wish to use the same interval, you can define the `interval` config. The important change in `k8sObjects` is that you don't need to specify resource group and version. It automatically detects it.


### Translating custom configurations from SCK to Splunk OpenTelemetry Collector for Kubernetes for metrics

#### Limiting or increasing application resources"

If you are using the `resources` option in SCK to limit/increase the CPU and memory usage in SCK, you can do the same using the `resources` option in `clusterReceiver` section in Splunk OpenTelemetry Collector for Kubernetes.

#### Pod scheduling configurations

If you are using any pod scheduling operations such as `nodeSelector`, `affinity`, and `tolerations` in SCK, you can use the `clusterReceiver.nodeSelector`, `clusterReceiver.affinity` and `clusterReceiver.tolerations` options in Splunk OpenTelemetry Collector for Kubernetes.

#### Pod security context

If you are using the `podSecurityContext` option to set a pod security policy, you can use the `clusterReceiver.securityContext` option in Splunk OpenTelemetry Collector for Kubernetes.

#### Custom annotations

If you are using the `customMetadataAnnotations` option to set a custom annotation fields in your logs in SCK, you can use the `extraAttributes.fromAnnotations` option in Splunk OpenTelemetry Collector for Kubernetes.

## (Optional) Step 2: Delete SCK Deployment

To delete the SCK deployment, find the name of the deployment using the `helm ls` command.

* **If you want to delete only logs and metrics**

  * Update the values.yaml file used to deploy SCK to disable objects and run the following command:

  * helm upgrade local-k8s -f your-values-file.yaml splunk/splunk-connect-for-kubernetes

* **If you want to delete only logs**

  * Update the values.yaml file used to deploy SCK to disable objects and run the following command:

  * helm upgrade local-k8s -f your-values-file.yaml splunk/splunk-connect-for-kubernetes

* **If you want to delete your entire SCK deployment (logs, objects and metrics), run the following command**

  * helm delete local-k8s

## Step 3: Installing Splunk OpenTelemetry Collector for Kubernetes

* See [How to install](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/README.md#how-to-install) for the instructions.

## Step 4: Check data in Splunk

* Check the logs index to see if you are receiving logs from your Kubernetes cluster
  * ```index="Your logs index"```
* Check the metrics index to see if you are receiving metrics from your Kubernetes cluster
  * ```| mcatalog values(metric_name) WHERE index="Your metrics index"```

## Differences between Splunk Connect for Kubernetes and Splunk OpenTelemetry Collector for Kubernetes

### Read logs location

Splunk Connect for Kubernetes by default read containers logs from `/var/log/containers/*`
Splunk OpenTelemetry Collector for Kubernetes by default read containers logs from `/var/log/pods/*`
Change is reflected in `source` filed for extracted logs.

### Default `sourcetype` for containers logs

Both Splunk Connect for Kubernetes and Splunk OpenTelemetry Collector for Kubernetes define `sourcetype` for container logs as `kube:container:<container_name>` by default. But, Splunk Connect for Kubernetes explicitly defines the sourcetype of Kubernetes core components as `kube:<container_name>`. They are defined [here](https://github.com/splunk/splunk-connect-for-kubernetes/blob/2cae9b12bbd6545c9ef09b23e619b9783d9ceb38/helm-chart/splunk-connect-for-kubernetes/values.yaml#L330-L408)
`sourcetype` configuration can be changed by adding `logsCollection.containers.extraOperators` configuration.

### Extracted fields for logs

Splunk OpenTelemetry Collector for Kubernetes follows naming convention for OpenTelemetry for extracted fields. Table below present differences in filed names extracted by Splunk OpenTelemetry Collector for Kubernetes and Splunk Connect for Kubernetes

| Splunk Connect for Kubernetes | Splunk OpenTelemetry Collector for Kubernetes |
|-------------------------------|-----------------------------------------------|
| container_id                  | container.id                                  |
| container_image               | container.image.name and container.image.tag  |
| container_name                | k8s.container.name                            |
| cluster_name                  | k8s.cluster.name                              |
| namespace                     | k8s.namespace.name                            |
| pod                           | k8s.pod.name                                  |
| pod_uid                       | k8s.pod_uid                                   |
| label_app                     | k8s.pod.labels.app                            |

If you wish to continue using Splunk Connect for Kubernetes's naming convention, you can use the following configuration:

```yaml
splunkPlatform:
  fieldNameConvention:
    # Boolean for renaming pod metadata fields to match to Splunk Connect for Kubernetes helm chart.
    renameFieldsSck: true
    # Boolean for keeping Otel convention fields after renaming it
    keepOtelConvention: false
```
