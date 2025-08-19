{{/*
Config for the otel-collector k8s cluster receiver deployment.
The values can be overridden in .Values.clusterReceiver.config
*/}}
{{- define "splunk-otel-collector.clusterReceiverConfig" -}}
extensions:
  health_check:
    endpoint: 0.0.0.0:13134

  {{- if eq .Values.distribution "eks/fargate" }}
  # k8s_observer w/ pod and node detection for eks/fargate deployment
  k8s_observer:
    auth_type: serviceAccount
    observe_pods: true
    observe_nodes: true
  {{- end }}

receivers:
  # Prometheus receiver scraping metrics from the pod itself
  {{- include "splunk-otel-collector.prometheusInternalMetrics" (dict "receiver" "k8s-cluster-receiver" "port" "8899") | nindent 2}}

  k8s_cluster:
    auth_type: serviceAccount
    {{- if eq (include "splunk-otel-collector.o11yMetricsEnabled" $) "true" }}
    metadata_exporters: [signalfx]
    {{- end }}
    {{- if eq .Values.distribution "openshift" }}
    distribution: openshift
    {{- end }}
  {{- if and (eq (include "splunk-otel-collector.objectsEnabled" .) "true") (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
  k8sobjects:
    auth_type: serviceAccount
    objects: {{ .Values.clusterReceiver.k8sObjects | toYaml | nindent 6 }}
  {{- end }}
  {{- if and .Values.clusterReceiver.eventsEnabled (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
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
  {{- if eq .Values.distribution "eks/fargate" }}
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

  {{- if and (hasPrefix "eks" .Values.distribution) .Values.featureGates.enableEKSApiServerMetrics }}
  prometheus/kubernetes-apiserver:
    config:
      scrape_configs:
        - job_name: 'kubernetes-apiserver'
          tls_config:
            ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
          authorization:
            credentials_file: /var/run/secrets/kubernetes.io/serviceaccount/token
          scheme: https
          static_configs:
            - targets:
              - ${KUBERNETES_SERVICE_HOST}:443
          metric_relabel_configs:
            - source_labels: [__name__]
              action: keep
              regex: "(apiserver_longrunning_requests|\
                apiserver_request_duration_seconds|\
                apiserver_storage_objects|\
                apiserver_response_sizes|\
                apiserver_request_total|\
                rest_client_requests_total|\
                rest_client_request_duration_seconds)(?:_sum|_count|_bucket)?"
  {{- end }}

processors:
  {{- include "splunk-otel-collector.otelMemoryLimiterConfig" . | nindent 2 }}

  {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
  {{- include "splunk-otel-collector.k8sAttributesSplunkPlatformMetrics" . | nindent 2 }}
    filter:
      node_from_env_var: K8S_NODE_NAME
  {{- if .Values.splunkPlatform.sourcetype }}
  {{- include "splunk-otel-collector.resourceMetricsProcessor" . | nindent 2 }}
  {{- end }}
  {{- end }}

  batch:
    send_batch_max_size: 32768

  {{- include "splunk-otel-collector.resourceDetectionProcessor" . | nindent 2 }}
  {{- if eq (include "splunk-otel-collector.autoDetectClusterName" .) "true" }}
  {{- include "splunk-otel-collector.resourceDetectionProcessorKubernetesClusterName" . | nindent 2 }}
  {{- end }}

  {{- if eq (include "splunk-otel-collector.o11yInfraMonEventsEnabled" .) "true" }}
  resource/add_event_k8s:
    attributes:
      - action: insert
        key: kubernetes_cluster
        value: {{ .Values.clusterName }}
  {{- end }}

  {{- if and .Values.clusterReceiver.eventsEnabled (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}

  # Add k8s event attributes - k8s.<kind>.name and k8s.<kind>.uid
  transform/k8sevents:
    error_mode: ignore
    log_statements:
      - conditions:
          - resource.attributes["k8s.object.kind"] == "HorizontalPodAutoscaler"
        statements:
          - set(resource.attributes["k8s.hpa.name"], resource.attributes["k8s.object.name"])
          - set(resource.attributes["k8s.hpa.uid"], resource.attributes["k8s.object.uid"])
      - conditions:
          - resource.attributes["k8s.object.kind"] != "HorizontalPodAutoscaler"
        statements:
          - set(resource.attributes[Concat(["k8s", ConvertCase(resource.attributes["k8s.object.kind"], "lower"), "name"], ".")], resource.attributes["k8s.object.name"])
          - set(resource.attributes[Concat(["k8s", ConvertCase(resource.attributes["k8s.object.kind"], "lower"), "uid"], ".")], resource.attributes["k8s.object.uid"])
      - conditions:
          - resource.attributes["k8s.object.kind"] == "Pod" and IsMatch(resource.attributes["k8s.object.fieldpath"], "spec\\.containers.*")
        statements:
          - merge_maps(resource.cache, ExtractPatterns(resource.attributes["k8s.object.fieldpath"], "spec.containers\\{(?P<k8s_container_name>[^\\}]+)\\}"), "insert")
          - set(resource.attributes["k8s.container.name"], resource.cache["k8s_container_name"])

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

  {{- if or
    (and .Values.clusterReceiver.eventsEnabled (eq (include "splunk-otel-collector.logsEnabled" .) "true"))
    (and (eq (include "splunk-otel-collector.objectsEnabled" .) "true") (eq (include "splunk-otel-collector.logsEnabled" .) "true"))
  }}
  {{- include "splunk-otel-collector.k8sClusterReceiverAttributesProcessor" . | nindent 2 }}
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
      {{- if eq .Values.distribution "eks/fargate" }}
      - action: insert
        key: cloud.platform
        value: aws_eks
      - action: insert
        key: cloud.provider
        value: aws
      {{- end }}

  resource:
    attributes:
      # TODO: Remove once available in mapping service.
      - action: insert
        key: metric_source
        value: kubernetes
      {{- if .Values.clusterName }}
      - action: upsert
        key: k8s.cluster.name
        value: {{ .Values.clusterName }}
      {{- end }}
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

  # The following processor is used to add "otelcol.service.mode" attribute to the internal metrics
  resource/add_mode:
    attributes:
      - action: insert
        value: "clusterReceiver"
        key: otelcol.service.mode

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
    {{- if not (eq .Values.distribution "eks/fargate") }}
    disable_default_translation_rules: true
    {{- end}}
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
  {{- if .Values.clusterReceiver.eventsEnabled }}
    sourcetype: kube:events
  {{- end }}
  {{- end }}

  {{- if and (and (hasPrefix "eks" .Values.distribution) .Values.featureGates.enableEKSApiServerMetrics) (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
  signalfx/histograms:
    ingest_url: {{ include "splunk-otel-collector.o11yIngestUrl" . }}
    api_url: {{ include "splunk-otel-collector.o11yApiUrl" . }}
    access_token: ${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}
    timeout: 10s
    {{- if not (eq .Values.distribution "eks/fargate") }}
    disable_default_translation_rules: true
    {{- end}}
    send_otlp_histograms: true
  {{- end }}

service:
  telemetry:
    resource:
      service.name: otel-k8s-cluster-receiver
    metrics:
      readers:
        - pull:
            exporter:
              prometheus:
                host: localhost
                port: 8899
                without_scope_info: true
                without_units: true
                without_type_suffix: true
  {{- if eq .Values.distribution "eks/fargate" }}
  extensions: [health_check, k8s_observer]
  {{- else }}
  extensions: [health_check]
  {{- end }}
  pipelines:
    {{- if or (eq (include "splunk-otel-collector.o11yMetricsEnabled" $) "true") (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
    # k8s metrics pipeline
    metrics:
      receivers: [k8s_cluster]
      processors:
        - memory_limiter
        - batch
        {{- if eq (include "splunk-otel-collector.autoDetectClusterName" .) "true" }}
        - resourcedetection/k8s_cluster_name
        {{- end }}
        - resource
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - k8sattributes/metrics
        {{- if .Values.splunkPlatform.sourcetype }}
        - resource/metrics
        {{- end }}
        {{- end }}
        - resource/k8s_cluster
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}

    {{- if eq .Values.distribution "eks/fargate" }}
    metrics/eks:
      receivers: [receiver_creator]
      processors:
        - memory_limiter
        - batch
        {{- if eq (include "splunk-otel-collector.autoDetectClusterName" .) "true" }}
        - resourcedetection/k8s_cluster_name
        {{- end }}
        - resource
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - k8sattributes/metrics
        {{- if .Values.splunkPlatform.sourcetype }}
        - resource/metrics
        {{- end }}
        {{- end }}
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
        - resource/add_mode
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - k8sattributes/metrics
        {{- if .Values.splunkPlatform.sourcetype }}
        - resource/metrics
        {{- end }}
        {{- end }}
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}
    {{- end }}

    {{- if and .Values.clusterReceiver.eventsEnabled (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
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
        - transform/k8sevents
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
        - resourcedetection
        - resource
        {{- if .Values.clusterName }}
        - resource/add_event_k8s
        {{- end }}
      exporters:
        - signalfx
    {{- end }}

    {{- if and (and (hasPrefix "eks" .Values.distribution) .Values.featureGates.enableEKSApiServerMetrics) (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
    metrics/histograms:
      receivers:
        - prometheus/kubernetes-apiserver
      processors:
        - memory_limiter
        - batch
        {{- if eq (include "splunk-otel-collector.autoDetectClusterName" .) "true" }}
        - resourcedetection/k8s_cluster_name
        {{- end }}
        - resource
      exporters:
        - signalfx/histograms
    {{- end }}

{{- end }}
