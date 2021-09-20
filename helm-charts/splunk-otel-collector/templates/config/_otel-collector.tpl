{{/*
Config for the optional standalone collector
The values can be overridden in .Values.otelCollector.config
*/}}
{{- define "splunk-otel-collector.otelCollectorConfig" -}}
extensions:
  health_check:

  http_forwarder:
    egress:
      endpoint: {{ include "splunk-otel-collector.apiUrl" . }}

  zpages:

receivers:
  {{- include "splunk-otel-collector.otelTraceReceivers" . | nindent 2 }}
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

# By default k8s_tagger, memory_limiter and batch processors enabled.
processors:
  k8s_tagger:
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

  resource/add_cluster_name:
    attributes:
      - action: upsert
        value: {{ .Values.clusterName }}
        key: k8s.cluster.name
      {{- range .Values.extraAttributes.custom }}
      - action: upsert
        value: {{ .value }}
        key: {{ .name }}
      {{- end }}

  {{- if .Values.environment }}
  resource/add_environment:
    attributes:
      - action: insert
        value: {{ .Values.environment }}
        key: deployment.environment
  {{- end }}

exporters:
  {{- include "splunk-otel-collector.otelSapmExporter" . | nindent 2 }}
  {{- if .Values.logsEnabled }}
  splunk_hec:
    endpoint: {{ include "splunk-otel-collector.ingestUrl" . }}/v1/log
    token: "${SPLUNK_ACCESS_TOKEN}"
  {{- end }}
  signalfx:
    ingest_url: {{ include "splunk-otel-collector.ingestUrl" . }}
    api_url: {{ include "splunk-otel-collector.apiUrl" . }}
    access_token: ${SPLUNK_ACCESS_TOKEN}

service:
  extensions: [health_check, http_forwarder, zpages]

  # The default pipelines should not need to be changed. You can add any custom pipeline instead.
  # In order to disable a default pipeline just set it to `null` in otelCollector.config overrides.
  pipelines:
    {{- if .Values.tracesEnabled }}
    # default traces pipeline
    traces:
      receivers: [otlp, jaeger, zipkin]
      processors:
        - memory_limiter
        - batch
        - k8s_tagger
        - resource/add_cluster_name
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
      exporters: [sapm]
    {{- end }}

    # default metrics pipeline
    metrics:
      receivers: [otlp, signalfx]
      processors: [memory_limiter, batch, resource/add_cluster_name]
      exporters: [signalfx]

    # logs pipeline for receiving and exporting SignalFx events
    logs/signalfx-events:
      receivers: [signalfx]
      processors: [memory_limiter, batch]
      exporters: [signalfx]

    {{- if .Values.logsEnabled }}
    # default logs pipeline
    logs:
      receivers: [otlp]
      processors:
        - memory_limiter
        - k8s_tagger
        - batch
        - filter/logs
        - resource/logs
      exporters: [splunk_hec]
    {{- end }}

    # Pipeline for metrics collected about the collector pod itself.
    metrics/collector:
      receivers: [prometheus/collector]
      processors:
        - memory_limiter
        - batch
        - resource/add_cluster_name
        - resource/add_collector_k8s
        - resourcedetection
      exporters: [signalfx]
{{- end }}
