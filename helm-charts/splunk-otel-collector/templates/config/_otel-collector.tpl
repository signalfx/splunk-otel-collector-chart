{{/*
Config for the optional standalone collector
The values can be overridden in .Values.gateway.config
*/}}
{{- define "splunk-otel-collector.gatewayConfig" -}}
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
  {{- include "splunk-otel-collector.k8sAttributesProcessor" . | nindent 2 }}
  {{- include "splunk-otel-collector.resourceLogsProcessor" . | nindent 2 }}
  {{- if .Values.autodetect.istio }}
  {{- include "splunk-otel-collector.transformLogsProcessor" . | nindent 2 }}
  {{- end }}
  {{- include "splunk-otel-collector.filterLogsProcessors" . | nindent 2 }}

  {{- include "splunk-otel-collector.otelMemoryLimiterConfig" . | nindent 2 }}

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

  # It's important to put this processor after resourcedetection to make sure that
  # k8s.name.cluster attribute is always set to "{{ .Values.clusterName }}".
  resource/add_cluster_name:
    attributes:
      - action: upsert
        value: {{ .Values.clusterName }}
        key: k8s.cluster.name

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
    sending_queue:
      num_consumers: 32
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.o11yTracesEnabled" .) "true") }}
  {{- include "splunk-otel-collector.otelSapmExporter" . | nindent 2 }}
    sending_queue:
      num_consumers: 32
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.o11yLogsOrProfilingEnabled" .) "true") }}
  splunk_hec/o11y:
    endpoint: {{ include "splunk-otel-collector.o11yIngestUrl" . }}/v1/log
    token: "${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}"
    log_data_enabled: {{ .Values.splunkObservability.logsEnabled }}
    profiling_data_enabled: {{ .Values.splunkObservability.profilingEnabled }}
    sending_queue:
      num_consumers: 32
    # Temporary disable compression until 0.68.0 to workaround a compression bug
    disable_compression: true
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformLogsExporter" . | nindent 2 }}
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformMetricsExporter" . | nindent 2 }}
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.platformTracesEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformTracesExporter" . | nindent 2 }}
  {{- end }}
service:
  telemetry:
    metrics:
      address: 0.0.0.0:8889
  extensions:
    - health_check
    - memory_ballast
    - zpages
    {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
    - http_forwarder
    {{- end }}

  # The default pipelines should not need to be changed. You can add any custom pipeline instead.
  # In order to disable a default pipeline just set it to `null` in gateway.config overrides.
  pipelines:
    {{- if (eq (include "splunk-otel-collector.tracesEnabled" $) "true") }}
    # default traces pipeline
    traces:
      receivers: [otlp, jaeger, zipkin]
      processors:
        - memory_limiter
        - k8sattributes
        - batch
        - resource/add_cluster_name
        {{- if .Values.extraAttributes.custom }}
        - resource/add_custom_attrs
        {{- end }}
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yTracesEnabled" .) "true") }}
        - sapm
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformTracesEnabled" .) "true") }}
        - splunk_hec/platform_traces
        {{- end }}
    {{- end }}

    {{- if (eq (include "splunk-otel-collector.metricsEnabled" .) "true") }}
    # default metrics pipeline
    metrics:
      receivers: [otlp, signalfx]
      processors:
        - memory_limiter
        - batch
        - resource/add_cluster_name
        {{- if .Values.extraAttributes.custom }}
        - resource/add_custom_attrs
        {{- end }}
        {{/*
        The attribute `deployment.environment` is not being set on metrics sent to Splunk Observability because it's already synced as the `sf_environment` property.
        More details: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/signalfxexporter#traces-configuration-correlation-only
        */}}
        {{- if (and .Values.splunkPlatform.metricsEnabled .Values.environment) }}
        - resource/add_environment
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

    {{- if or (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq (include "splunk-otel-collector.profilingEnabled" .) "true") }}
    # default logs + profiling data pipeline
    logs:
      receivers: [otlp]
      processors:
        - memory_limiter
        - k8sattributes
        - filter/logs
        - batch
        {{- if .Values.autodetect.istio }}
        - transform/istio_service_name
        {{- end }}
        - resource/logs
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yLogsOrProfilingEnabled" .) "true") }}
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
        - resource/add_collector_k8s
        - resourcedetection
        - resource/add_cluster_name
      exporters:
        {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}
    {{- end }}
{{- end }}
