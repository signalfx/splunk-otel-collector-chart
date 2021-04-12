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
  prometheus:
    config:
      scrape_configs:
      - job_name: 'otel-collector'
        scrape_interval: 10s
        static_configs:
        - targets: ["${K8S_POD_IP}:8888"]

  fluentforward:
    endpoint: 0.0.0.0:8006

  signalfx:
    endpoint: 0.0.0.0:9943

# By default k8s_tagger, memory_limiter and batch processors enabled.
processors:
  k8s_tagger:
    extract:
      metadata:
        - namespace
        - node
        - podName
        - podUID
      {{- with .Values.extraAttributes.podLabels }}
      labels:
        {{- range . }}
        - key: {{ . }}
        {{- end }}
      {{- end }}

  {{- include "splunk-otel-collector.otelMemoryLimiterConfig" .Values.otelCollector | nindent 2 }}

  batch:

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
  signalfx:
    ingest_url: {{ include "splunk-otel-collector.ingestUrl" . }}
    api_url: {{ include "splunk-otel-collector.apiUrl" . }}
    access_token: ${SPLUNK_ACCESS_TOKEN}

service:
  extensions: [health_check, http_forwarder, zpages]

  # The default pipelines should not need to be changed. You can add any custom pipeline instead.
  # In order to disable a default pipeline just set it to `null` in otelCollector.config overrides.
  pipelines:

    # default traces pipeline
    traces:
      receivers: [otlp, jaeger, zipkin, sapm]
      processors:
        - memory_limiter
        - batch
        - k8s_tagger
        - resource/add_cluster_name
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
      exporters: [sapm]

    # default metrics pipeline
    metrics:
      receivers: [otlp, prometheus]
      processors: [memory_limiter, batch, resource/add_cluster_name]
      exporters: [signalfx]

    # default logs pipeline
    logs:
      receivers: [signalfx]
      processors: [memory_limiter, batch]
      exporters: [signalfx]
{{- end }}
