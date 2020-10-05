{{/*
Config for the optional standalone collector
The values can be overridden in .Values.otelCollector.config
*/}}
{{- define "o11y-collector.otelCollectorConfig" -}}
extensions:
  health_check: {}

receivers:
  {{- include "o11y-collector.otelTraceReceivers" . | nindent 2 }}
  # Prometheus receiver scraping metrics from the pod itself
  prometheus:
    config:
      scrape_configs:
      - job_name: 'otel-collector'
        scrape_interval: 10s
        static_configs:
        - targets: ["${K8S_POD_IP}:8888"]


# By default k8s_tagger, memory_limiter, queued_retry and batch processors enabled.
processors:
  k8s_tagger: {}

  {{- include "o11y-collector.otelMemoryLimiterConfig" .Values.otelCollector | nindent 2 }}

  queued_retry: {}
  batch:
    timeout: 1s
    send_batch_size: 1024

  resource/add_cluster_name:
    attributes:
    - action: upsert
      value: {{ .Values.clusterName }}
      key: k8s.cluster.name

exporters:
  {{- include "o11y-collector.otelSapmExporter" . | nindent 2 }}
  signalfx:
    ingest_url: {{ include "o11y-collector.ingestUrl" . }}/v2/datapoint
    api_url: {{ include "o11y-collector.apiUrl" . }}
    access_token: {{ include "o11y-collector.accessToken" . }}
    send_compatible_metrics: true

service:
  extensions: [health_check]

  # By default there are two pipelines sending metrics and traces to signalfx backend.
  # The default pipelines should to be changed. You can add any custom pipeline instead.
  # In order to disable a default pipeline just set it to `null` in otelCollector.config overrides.
  pipelines:

    # default traces pipeline
    traces:
      receivers: [otlp, jaeger, zipkin, opencensus, sapm]
      processors: [memory_limiter, k8s_tagger, resource/add_cluster_name, batch, queued_retry]
      exporters: [sapm]

    # default metrics pipeline
    metrics:
      receivers: [otlp, prometheus]
      processors: [memory_limiter, k8s_tagger, resource/add_cluster_name, queued_retry]
      exporters: [signalfx]
{{- end }}
