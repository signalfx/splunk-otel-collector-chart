{{/*
Config for the otel-collector k8s cluster receiver deployment.
The values can be overridden in .Values.clusterReceiver.config
*/}}
{{- define "splunk-otel-collector.clusterReceiverConfig" -}}
{{ $clusterReceiver := fromYaml (include "splunk-otel-collector.clusterReceiver" .) -}}
extensions:
  {{- if (eq (include "splunk-otel-collector.persistentQueueEnabledLogs" .) "true") }}
  {{- include "splunk-otel-collector.persistentQueueLogs" (dict "Values" .Values "forAgent" false) | nindent 2 }}
  {{- end }}
  {{- if (eq (include "splunk-otel-collector.persistentQueueEnabledMetrics" .) "true") }}
  {{- include "splunk-otel-collector.persistentQueueMetrics" (dict "Values" .Values "forAgent" false) | nindent 2 }}
  {{- end }}

  health_check:

  memory_ballast:
    size_mib: ${SPLUNK_BALLAST_SIZE_MIB}

  {{- if eq (include "splunk-otel-collector.distribution" .) "eks/fargate" }}
  # k8s_observer w/ pod and node detection for eks/fargate deployment
  k8s_observer:
    auth_type: serviceAccount
    observe_pods: true
    observe_nodes: true
  {{- end }}

receivers:
  # Prometheus receiver scraping metrics from the pod itself, both otel and fluentd
  prometheus/k8s_cluster_receiver:
    config:
      scrape_configs:
      - job_name: 'otel-k8s-cluster-receiver'
        scrape_interval: 10s
        static_configs:
        - targets: ["${K8S_POD_IP}:8889"]
  k8s_cluster:
    auth_type: serviceAccount
    {{- if eq (include "splunk-otel-collector.o11yMetricsEnabled" $) "true" }}
    metadata_exporters: [signalfx]
    {{- end }}
    {{- if eq (include "splunk-otel-collector.distribution" .) "openshift" }}
    distribution: openshift
    {{- end }}
  {{- if and (eq (include "splunk-otel-collector.objectsEnabled" .) "true") (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
  k8sobjects:
    auth_type: serviceAccount
    objects: {{ .Values.clusterReceiver.k8sObjects | toYaml | nindent 6 }}
  {{- end }}
  {{- if and $clusterReceiver.eventsEnabled (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
  k8s_events:
    auth_type: serviceAccount
  {{- end }}
  {{- if eq (include "splunk-otel-collector.o11yInfraMonEventsEnabled" .) "true" }}
  smartagent/kubernetes-events:
    type: kubernetes-events
    alwaysClusterReporter: true
    whitelistedEvents:
    - reason: Created
      involvedObjectKind: Pod
    - reason: Unhealthy
      involvedObjectKind: Pod
    - reason: Failed
      involvedObjectKind: Pod
    - reason: FailedCreate
      involvedObjectKind: Job
  {{- end }}
  {{- if eq (include "splunk-otel-collector.distribution" .) "eks/fargate" }}
  # dynamically created kubeletstats receiver to report all Fargate "node" kubelet stats
  # with exception of collector "node's" own since Fargate forbids connection.
  receiver_creator:
    receivers:
      kubeletstats:
        rule: type == "k8s.node" && name contains "fargate"
        config:
          auth_type: serviceAccount
          collection_interval: 10s
          endpoint: "`endpoint`:`kubelet_endpoint_port`"
          extra_metadata_labels:
            - container.id
          metric_groups:
            - container
            - pod
            - node
    watch_observers:
      - k8s_observer
  {{- end }}

processors:
  {{- include "splunk-otel-collector.otelMemoryLimiterConfig" . | nindent 2 }}

  batch:
    send_batch_max_size: 32768

  {{- include "splunk-otel-collector.resourceDetectionProcessor" . | nindent 2 }}

  {{- if eq (include "splunk-otel-collector.o11yInfraMonEventsEnabled" .) "true" }}
  resource/add_event_k8s:
    attributes:
      - action: insert
        key: kubernetes_cluster
        value: {{ .Values.clusterName }}
  {{- end }}

  {{- if and $clusterReceiver.eventsEnabled (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
  # Drop high cardinality k8s event attributes
  attributes/drop_event_attrs:
    actions:
      - key: k8s.event.start_time
        action: delete
      - key: k8s.event.name
        action: delete
      - key: k8s.event.uid
        action: delete
  {{- end }}

  {{- if and (eq (include "splunk-otel-collector.objectsEnabled" .) "true") (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
  transform/add_sourcetype:
    log_statements:
      - context: log
        statements:
          - set(resource.attributes["com.splunk.sourcetype"], Concat(["kube:object:", attributes["k8s.resource.name"]], ""))
  {{- end }}

  # Resource attributes specific to the collector itself.
  resource/add_collector_k8s:
    attributes:
      - action: insert
        key: k8s.node.name
        value: "${K8S_NODE_NAME}"
      - action: insert
        key: k8s.pod.name
        value: "${K8S_POD_NAME}"
      - action: insert
        key: k8s.pod.uid
        value: "${K8S_POD_UID}"
      - action: insert
        key: k8s.namespace.name
        value: "${K8S_NAMESPACE}"

  resource:
    attributes:
      # TODO: Remove once available in mapping service.
      - action: insert
        key: metric_source
        value: kubernetes
      - action: upsert
        key: k8s.cluster.name
        value: {{ .Values.clusterName }}
      {{- range .Values.extraAttributes.custom }}
      - action: upsert
        key: {{ .name }}
        value: {{ .value }}
      {{- end }}


  {{- if and ( eq ( include "splunk-otel-collector.objectsOrEventsEnabled" . ) "true") .Values.environment }}
  resource/add_environment:
    attributes:
      - action: insert
        key: deployment.environment
        value: "{{ .Values.environment }}"
  {{- end }}

  resource/k8s_cluster:
    attributes:
      # XXX: Added so that Smart Agent metrics and OTel metrics don't map to the same MTS identity
      # (same metric and dimension names and values) after mappings are applied. This would be
      # the case if somebody uses the same cluster name from Smart Agent and OTel in the same org.
      - action: insert
        key: receiver
        value: k8scluster

exporters:
  {{- if or (eq (include "splunk-otel-collector.o11yMetricsEnabled" $) "true") (eq (include "splunk-otel-collector.o11yInfraMonEventsEnabled" .) "true") }}
  signalfx:
    ingest_url: {{ include "splunk-otel-collector.o11yIngestUrl" . }}
    api_url: {{ include "splunk-otel-collector.o11yApiUrl" . }}
    access_token: ${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}
    timeout: 10s
  {{- end }}

  {{- if and (eq (include "splunk-otel-collector.o11yLogsEnabled" .) "true") (eq (include "splunk-otel-collector.objectsOrEventsEnabled" .) "true") }}
  splunk_hec/o11y:
    endpoint: {{ include "splunk-otel-collector.o11yIngestUrl" . }}/v1/log
    token: "${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}"
    log_data_enabled: true
    profiling_data_enabled: false
    # Temporary disable compression until 0.68.0 to workaround a compression bug
    disable_compression: true
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformMetricsExporter" . | nindent 2 }}
  {{- end }}

  {{- if and (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") (eq (include "splunk-otel-collector.objectsOrEventsEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformLogsExporter" . | nindent 2 }}
  {{- if $clusterReceiver.eventsEnabled }}
    sourcetype: kube:events
  {{- end }}
  {{- end }}

service:
  telemetry:
    metrics:
      address: 0.0.0.0:8889
  extensions:
    - health_check
    - memory_ballast
    {{- if eq (include "splunk-otel-collector.distribution" .) "eks/fargate" }}
    - k8s_observer
    {{- end }}
    {{- if (eq (include "splunk-otel-collector.persistentQueueEnabledLogs" .) "true") }}
    - file_storage/persistent_queue_logs
    {{- end }}
    {{- if (eq (include "splunk-otel-collector.persistentQueueEnabledMetrics" .) "true") }}
    - file_storage/persistent_queue_metrics
    {{- end }}
  pipelines:
    {{- if or (eq (include "splunk-otel-collector.o11yMetricsEnabled" $) "true") (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
    # k8s metrics pipeline
    metrics:
      receivers: [k8s_cluster]
      processors: [memory_limiter, batch, resource, resource/k8s_cluster]
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}

    {{- if eq (include "splunk-otel-collector.distribution" .) "eks/fargate" }}
    metrics/eks:
      receivers: [receiver_creator]
      processors: [memory_limiter, batch, resource]
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}
    {{- end }}

    # Pipeline for metrics collected about the collector pod itself.
    metrics/collector:
      receivers: [prometheus/k8s_cluster_receiver]
      processors:
        - memory_limiter
        - batch
        - resource/add_collector_k8s
        - resourcedetection
        - resource
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}
    {{- end }}

    {{- if and $clusterReceiver.eventsEnabled (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
    logs:
      receivers:
        - k8s_events
      processors:
        - memory_limiter
        - batch
        - attributes/drop_event_attrs
        - resourcedetection
        - resource
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yLogsEnabled" .) "true") }}
        - splunk_hec/o11y
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") }}
        - splunk_hec/platform_logs
        {{- end }}
    {{- end }}

    {{- if and (eq (include "splunk-otel-collector.objectsEnabled" .) "true") (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
    logs/objects:
      receivers:
        - k8sobjects
      processors:
        - memory_limiter
        - batch
        - resourcedetection
        - resource
        - transform/add_sourcetype
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yLogsEnabled" .) "true") }}
        - splunk_hec/o11y
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") }}
        - splunk_hec/platform_logs
        {{- end }}
    {{- end }}

    {{- if eq (include "splunk-otel-collector.o11yInfraMonEventsEnabled" .) "true" }}
    logs/events:
      receivers:
        - smartagent/kubernetes-events
      processors:
        - memory_limiter
        - batch
        - resource
        - resource/add_event_k8s
      exporters:
        - signalfx
    {{- end }}
{{- end }}
