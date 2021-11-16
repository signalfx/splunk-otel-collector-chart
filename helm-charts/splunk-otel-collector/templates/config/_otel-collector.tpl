{{/*
Config for the optional standalone collector
The values can be overridden in .Values.otelCollector.config
*/}}
{{- define "splunk-otel-collector.otelCollectorConfig" -}}
extensions:
  health_check:

  {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
  http_forwarder:
    egress:
      endpoint: {{ include "splunk-otel-collector.o11yApiUrl" . }}
  {{- end }}

  memory_ballast:
    size_mib: ${SPLUNK_BALLAST_SIZE_MIB}

  zpages:

receivers:
  {{- include "splunk-otel-collector.otelReceivers" . | nindent 2 }}
  # Prometheus receiver scraping metrics from the pod itself
  prometheus/collector:
    config:
      scrape_configs:
      - job_name: 'otel-collector'
        scrape_interval: 10s
        static_configs:
        - targets: ["${K8S_POD_IP}:8889"]
  signalfx:
    endpoint: 0.0.0.0:9943
    access_token_passthrough: true

# By default k8sattributes, memory_limiter and batch processors enabled.
processors:
  k8sattributes:
    pod_association:
      - from: resource_attribute
        name: k8s.pod.uid
      - from: resource_attribute
        name: k8s.pod.ip
      - from: resource_attribute
        name: ip
      - from: connection
      - from: resource_attribute
        name: host.name
    extract:
      metadata:
        - k8s.namespace.name
        - k8s.node.name
        - k8s.pod.name
        - k8s.pod.uid
      annotations:
        - key: splunk.com/sourcetype
          from: pod
        - key: splunk.com/exclude
          tag_name: splunk.com/exclude
          from: namespace
        - key: splunk.com/exclude
          tag_name: splunk.com/exclude
          from: pod
        - key: splunk.com/index
          tag_name: com.splunk.index
          from: namespace
        - key: splunk.com/index
          tag_name: com.splunk.index
          from: pod
      {{- with .Values.extraAttributes.podLabels }}
      labels:
        {{- range . }}
        - key: {{ . }}
        {{- end }}
      {{- end }}

  {{- include "splunk-otel-collector.resourceLogsProcessor" . | nindent 2 }}
  {{- include "splunk-otel-collector.filterLogsProcessors" . | nindent 2 }}

  {{- include "splunk-otel-collector.otelMemoryLimiterConfig" .Values.otelCollector | nindent 2 }}

  batch:

  {{- include "splunk-otel-collector.resourceDetectionProcessor" . | nindent 2 }}

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

  {{ if or .Values.clusterName }}
  resource/add_cluster_name:
    attributes:
      - action: upsert
        value: {{ .Values.clusterName }}
        key: k8s.cluster.name
  {{- end }}

  {{- if .Values.extraAttributes.custom }}
  resource/add_custom_attrs:
    attributes:
      {{- range .Values.extraAttributes.custom }}
      - action: upsert
        value: {{ .value }}
        key: {{ .name }}
      {{- end }}
  {{- end }}

  {{- if .Values.environment }}
  resource/add_environment:
    attributes:
      - action: insert
        value: {{ .Values.environment }}
        key: deployment.environment
  {{- end }}

exporters:
  {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
  signalfx:
    ingest_url: {{ include "splunk-otel-collector.o11yIngestUrl" . }}
    api_url: {{ include "splunk-otel-collector.o11yApiUrl" . }}
    access_token: ${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.o11yTracesEnabled" .) "true") }}
  {{- include "splunk-otel-collector.otelSapmExporter" . | nindent 2 }}
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.o11yLogsEnabled" .) "true") }}
  splunk_hec/o11y:
    endpoint: {{ include "splunk-otel-collector.o11yIngestUrl" . }}/v1/log
    token: "${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}"
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformLogsExporter" . | nindent 2 }}
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformMetricsExporter" . | nindent 2 }}
  {{- end }}

service:
  extensions:
    - health_check
    - memory_ballast
    - zpages
    {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
    - http_forwarder
    {{- end }}

  # The default pipelines should not need to be changed. You can add any custom pipeline instead.
  # In order to disable a default pipeline just set it to `null` in otelCollector.config overrides.
  pipelines:
    {{- if (eq (include "splunk-otel-collector.o11yTracesEnabled" $) "true") }}
    # default traces pipeline
    traces:
      receivers: [otlp, jaeger, zipkin]
      processors:
        - memory_limiter
        - batch
        - k8sattributes
        {{- if .Values.clusterName }}
        - resource/add_cluster_name
        {{- end }}
        {{- if .Values.extraAttributes.custom }}
        - resource/add_custom_attrs
        {{- end }}
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
      exporters: [sapm]
    {{- end }}

    {{- if (eq (include "splunk-otel-collector.metricsEnabled" .) "true") }}
    # default metrics pipeline
    metrics:
      receivers: [otlp, signalfx]
      processors:
        - memory_limiter
        - batch
        {{- if .Values.clusterName }}
        - resource/add_cluster_name
        {{- end }}
        {{- if .Values.extraAttributes.custom }}
        - resource/add_custom_attrs
        {{- end }}
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}
    {{- end }}

    {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
    # logs pipeline for receiving and exporting SignalFx events
    logs/signalfx-events:
      receivers: [signalfx]
      processors: [memory_limiter, batch]
      exporters: [signalfx]
    {{- end }}

    {{- if (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
    # default logs pipeline
    logs:
      receivers: [otlp]
      processors:
        - memory_limiter
        - k8sattributes
        - batch
        - filter/logs
        - resource/logs
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yLogsEnabled" .) "true") }}
        - splunk_hec/o11y
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") }}
        - splunk_hec/platform_logs
        {{- end }}
    {{- end }}

    {{- if or (eq (include "splunk-otel-collector.splunkO11yEnabled" $) "true") (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
    # Pipeline for metrics collected about the collector pod itself.
    metrics/collector:
      receivers: [prometheus/collector]
      processors:
        - memory_limiter
        - batch
        {{- if .Values.clusterName }}
        - resource/add_cluster_name
        {{- end }}
        - resource/add_collector_k8s
        - resourcedetection
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}
    {{- end }}
{{- end }}
