# Migration from Splunk Connect for Kubernetes

## Assumptions

You are running Splunk Connect for Kubernetes (SCK) 1.4.9 and want to migrate to Splunk OpenTelemetry Collector for Kubernetes. Previous versions of the SCK are out of scope, but this guide still applies. You might encounter some version specific issues (such as missing/changed config options) but that shouldn't prevent you from proceeding with migration.

## Changes in Components

Splunk Connect for Kubernetes(SCK) has 3 components/applications:
1.  Logs, metrics and traces from a kubernetes cluster (deployed as a DaemonSet)
2.  Application to fetch cluster metrics from a kubernetes cluster (deployed as a deployment)
3.  Application to fetch kubernetes objects metadata from a kubernetes cluster (deployed as a deployment)

All the applications in SCK use Fluentd to work with logs, metrics and objects. Fluentd has significant performance issues when used to fetch logs in a Kubernetes cluster with pods that have very high throughput.  

Splunk OpenTelemetry Collector for Kubernetes provides significant performance improvements over the SCK through use of the OpenTelemetry Collector agent and native OpenTelemetry functionality for logs collection rather than Fluentd. You can learn more about the performance characteristics of this new application [here](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/docs/advanced-configuration.md#performance-of-native-opentelemetry-logs-collection).

Splunk OpenTelemetry Collector for Kubernetes has 2 components/applications:
1.  Application to fetch logs and traces from a kubernetes cluster (deployed as a DaemonSet)
2.  Application to fetch metrics and objects from a kubernetes cluster (deployed as a DaemonSet).

No application currently exists for fetching kubernetes objects metadata from a kubernetes cluster. 

### Changes in Logs, Metrics and Objects

| Logging | Metrics | Objects |
|---|---|---|
| Redhat UBI docker images for our applications are no longer available, as we now use scratch images | The naming convention of the metrics used in SCK-OTEL has changed and there are some minor differences in the names of the metrics | Not implemented/Not available |
| SCK-OTEL does not support AWS Firelens. AWS Firelens uses Fluentd and we can achieve similar outcomes with opentelemetry. | Changes in the number of metrics (fewer metrics available) and additional metrics for the open telemetry collector |  |
|  |  |  |


### Changes in Logs in Splunk OpenTelemetry Connector for Kubernetes(SCK-OTEL)
* Redhat UBI docker images for our applications are no longer available, as we now use scratch images.
* Splunk OpenTelemetry Collector for Kubernetes does not support AWS Firelens. AWS Firelens uses Fluentd and we can achieve similar outcomes with OpenTelemetry.

### Changes in Metrics in Splunk OpenTelemetry Connector for Kubernetes(SCK-OTEL)
* The naming convention of the metrics used in Splunk OpenTelemetry Collector for Kubernetes follows the OpenTelemetry specification and is different than SCK. You will observe some minor differences in the names of the metrics. 
* Previously in SCK, you could get a large number of metrics from various APIs for kubernetes. However, in recent versions of kubernetes 1.18+, these API sources are disabled by default and there are fewer metrics available. The previous list of metrics can be found [here](https://github.com/splunk/fluent-plugin-kubernetes-metrics/blob/master/metrics-information.md).

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

### Changes in Objects in Splunk OpenTelemetry Connector for Kubernetes (SCK-OTEL)
This chart currently does not support the collection of Kubernetes objects. This is future functionality. You can still use SCK to collect other objects with current functionality.

## Migration Overview
### Migration Options/Matrix
 The following is a matrix for migrating from SCK to Splunk OpenTelemetry Collector for Kubernetes:

| Method  | Logs | Metrics | Objects |
|---|---|---|---|
| SCK | Yes | Yes | Yes |
| Splunk OpenTelemetry Collector for Kubernetes | Yes | Yes | No |

As shown above, you can acquire logs and metrics using Splunk OpenTelemetry Collector for Kubernetes. If you have objects currently deployed and need objects data, we recommend you leave your current SCK objects deployment as-is (helm command args --set="splunk-kubernetes-logging.enabled=false,splunk-kubernetes-metrics.enabled=false,splunk-kubernetes-objects.enabled=true").  In this way, you can use the SCK just for objects data together with Splunk OpenTelemetry Collector for Kubernetes for logs and other telemetry data. 
### Checkpoint Translation
WIth migration to the Splunk OpenTelemetry Collector for Kubernetes, the underlying framework/agent being used has changed, so there needs to be a baton handoff for checkpoint data so that the new OpenTelemetry agent can continue where Fluentd left off. All of this occurs automatically as an initContainer when you deploy the new helm chart in your cluster for the first time. As long as you properly configured the values.yaml file for the new helm chart, it will pick up where it left off. Without proper configuration for migration, there is a possibility of either data duplication or data loss while migrating from SCK to Splunk OpenTelemetry Collector for Kubernetes. 
 
 If you need to migrate Fluentd's position files again, you can delete the OpenTelemetry checkpoint files in the ```"/var/lib/otel_pos/"``` directory from Kubernetes nodes. Then restart the new helm chart Daemonet.
 
## Step 1: Preparing Your values.yaml file for migration
You must translate the values.yaml file from SCK to an appropriate format for SCK-OTEL. Before we begin, here are the configurations for SCK and SCK-OTEL. 
### Current Configuration for SCK
<https://github.com/splunk/splunk-connect-for-kubernetes/blob/develop/helm-chart/splunk-connect-for-kubernetes/values.yaml>
### Current Configuration for Splunk OpenTelemetry Collector for Kubernetes
<https://github.com/andrewy-splunk/splunk-otel-collector-chart/blob/main/helm-charts/splunk-otel-collector/values.yaml>

### Translating global/generic/splunk configurations from SCK to Splunk OpenTelemetry Collector for Kubernetes
#### Specifying your Splunk Platform and HTTP Event Collector (HEC) configuration
You can combine the "host", "port" and "protocol" options from SCK to use the "endpoint" option in Splunk OpenTelemetry Collector for Kubernetes. This option uses this format: "<http://X.X.X.X:8088/services/collector>" which will be interpreted as "protocol://host:port/services/collector".

If you are using the "clientCert", "clientKey" and "caFile" options from SCK, you can use the corresponding "clientCert", "clientKey" and "caFile" options in Splunk OpenTelemetry Collector for Kubernetes to specify your HEC certificate chain.

If you are using the "insecureSSL" option from SCK, you can use the insecure_skip_verify option in Splunk OpenTelemetry Collector for Kubernetes to specify whether to verify the certificates on HEC.

If you are using the "indexName" option from SCK you can use the index option in Splunk OpenTelemetry Collector for Kubernetes to specify which index you want to index data into. Translating custom configs from SCK to Splunk OpenTelemetry Collector for Kubernetes for logs

## Translating custom configs from SCK to Splunk OpenTelemetry Collector for Kubernetes for logs
You can find all the configuration options used to upgrade in the values.yaml files linked above for both SCK and Splunk OpenTelemetry Collector for Kubernetes.
 
#### Using Root user/permissions for accessing log files
If you are using root user/permissions for accessing log files in SCK, you can do so by setting "runAsUser: 0" in the securityContext in Splunk OpenTelemetry Collector for Kubernetes.

#### Container runtimes
If you configured SCK to explicitly set any of the following, you can do the same with Splunk OpenTelemetry Collector for Kubernetes:

* Set the **containerRuntime** option to a value from (CRI-O, containerd and docker) depending on your runtime.
* Set the **path** option to the location of your logs on your nodes.
* If you are using the **exclude_path** option from SCK, you can use the **excludePaths** option in Splunk OpenTelemetry Collector for Kubernetes to exclude any logs you want.
* If you are using the **"logs"** option to define **"multiline**" configs in SCK, you can use the **multilineConfigs** option in Splunk OpenTelemetry Collector for Kubernetes to concatenate multiline logs. 
* If you are using the **checkpointFile** option in SCK to define a custom location for checkpointing your logs, you can also do so with the **checkpointPath** option in Splunk OpenTelemetry Collector for Kubernetes.
* If you are using the **"logs"** option to ingest any other logs from your nodes, you can also do so with the **extraFileLogs** option in Splunk OpenTelemetry Collector for Kubernetes.

#### Cluster Name
If you are using the clusterName option to set a cluster name metadata field in your logs in SCK, you can also use the clusterName option in Splunk OpenTelemetry Collector for Kubernetes.

#### Pod Security Context
If you are using the podSecurityContext option to set a pod security policy, you can use the podSecurityContext option in Splunk OpenTelemetry Collector for Kubernetes.

#### Custom Metadata
If you are using the customMetadata option to set a custom metadata field in your logs in SCK, you can use the customMetadata option in Splunk OpenTelemetry Collector for Kubernetes.

#### Custom Annotations
If you are using the customMetadataAnnotations option to set custom annotation fields in your logs in SCK, you can use the "annotations" and "podAnnotations" option in Splunk OpenTelemetry Collector for Kubernetes.

#### Pod Scheduling configs
If you are using any pod scheduling operations such as nodeSelector, affinity and tolerations in SCK, you can use the nodeSelector, affinity and tolerations options in Splunk OpenTelemetry Collector for Kubernetes.

#### Service Accounts
If you are using the serviceAccount option to use your own service accounts in SCK, you can use the serviceAccount option in Splunk OpenTelemetry Collector for Kubernetes.

#### Secrets
If you are using the secret option to use your own service accounts in SCK, you can use the secret option in Splunk OpenTelemetry Collector for Kubernetes.

#### Custom docker image and pull secrets
If you are using custom docker images, tags, pull secrets and pull policy in SCK, you can achieve the same using the "image" option in Splunk OpenTelemetry Collector for Kubernetes to specify the relevant configs for using docker images.

#### Limiting/Increasing Application Resources
If you are using the resources option in SCK to limit/increase the CPU and memory usage in SCK, you can do the same using the resources option in Splunk OpenTelemetry Collector for Kubernetes.

#### Extra Files from Host
For tailing files other than container or journald logs (i.e., kube audit logs), you must configure "extraFileLogs" using this "[filelog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver)" receiver configuration. 

[SCK values.yaml snippet]
```
logs:
  kube-audit:
    from:
      file:
        path: /var/log/kube-apiserver-audit.log
    timestampExtraction:
      format: "%Y-%m-%dT%H:%M:%SZ"
    sourcetype: kube:apiserver-audit
```    

[SCK-OTEL values.yaml snippet]
```
extraFileLogs:
 filelog/kube-audit:
   include: [/var/log/kube-apiserver-audit.log]
   start_at: beginning
   include_file_path: true
   include_file_name: false
   resource:
     service.name: /var/log/kube-apiserver-audit.log
     host.name: 'EXPR(env("K8S_NODE_NAME"))'
     com.splunk.sourcetype: kube:apiserver-audit
```     

You must use the same keyword ("kube-audit" in the above example) for continuing to read from the translated checkpoint data.

### Translating custom configs from SCK to Splunk OpenTelemetry Collector for Kubernetes for metrics
#### Limiting/Increasing Application Resources
If you are using the resources option in SCK to limit/increase the CPU and memory usage in SCK, you can do the same using the resources option in "clusterDataCollector" section in Splunk OpenTelemetry Collector for Kubernetes.

#### Pod Scheduling configs
If you are using any pod scheduling operations such as nodeSelector, affinity and tolerations in SCK, you can use the nodeSelector, affinity and tolerations options in Splunk OpenTelemetry Collector for Kubernetes.

#### Pod Security Context
If you are using the podSecurityContext option to set a pod security policy, you can use the podSecurityContext option in SCK-OTEL.

#### Custom Annotations
If you are using the customMetadataAnnotations option to set a custom annotation fields in your logs in SCK, you can use the "annotations" and "podAnnotations" option in Splunk OpenTelemetry Collector for Kubernetes.

## Step 2: Delete SCK Deployment (Optional)
If you want to delete your SCK deployment:

* Get your current SCK deployment name by running this command:

  * helm ls

* **If you want to delete only logs and metrics**

  * Update the values.yaml file used to deploy SCK to disable objects and run the following command:

  * helm upgrade local-k8s -f your-values-file.yaml splunk/splunk-connect-for-kubernetes

* **If you want to delete only logs**

  * Update the values.yaml file used to deploy SCK to disable objects and run the following command:

  * helm upgrade local-k8s -f your-values-file.yaml splunk/splunk-connect-for-kubernetes

* **If you want to delete your entire SCK deployment (logs, objects and metrics), run the following command**

  * helm delete local-k8s

## Step 3: Installing Splunk OpenTelemetry Collector for Kubernetes

* Follow the README [here](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/README.md).

## Step 4: Check Data in Splunk
* Check your logs index to see if you are receiving logs from your kubernetes cluster
  * index="Your logs index"
* Check your metrics index to see if you receiving metrics from your kubernetes cluster
  * | mcatalog values(metric_name) WHERE index="Your metrics index"

## Step 5: Delete SCK Deployment (Optional)
### If you want to delete your SCK deployment
* Get your current SCK deployment name by running this command:
  * helm ls
* **If you want to delete only logs and metrics**
  * Update the values.yaml file used to deploy SCK to disable objects and run the following command:
    * helm upgrade local-k8s -f your-values-file.yaml splunk/splunk-connect-for-kubernetes
* **If you want to delete only logs**
  * Update the values.yaml file used to deploy SCK to disable objects and run the following command:
    * helm upgrade local-k8s -f your-values-file.yaml splunk/splunk-connect-for-kubernetes
* **If you want to delete your entire SCK deployment (logs, objects and metrics), run the following command**
  * helm delete local-k8s
