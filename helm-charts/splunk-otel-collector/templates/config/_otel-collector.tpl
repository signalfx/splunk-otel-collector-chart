{{/*
Config for the optional standalone collector
The values can be overridden in .Values.gateway.config
*/}}
{{- define "splunk-otel-collector.gatewayConfig" -}}
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
  {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
  http_forwarder:
    egress:
      endpoint: {{ include "splunk-otel-collector.o11yApiUrl" . }}
  {{- end }}


  zpages:

  headers_setter:
    headers:
      - action: upsert
        key: X-SF-TOKEN
        from_context: X-SF-TOKEN
        default_value: "${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}"

receivers:
  {{- include "splunk-otel-collector.otelReceivers" . | nindent 2 }}

  # Prometheus receiver scraping metrics from the pod itself
  {{- include "splunk-otel-collector.prometheusInternalMetrics" (dict "receiver" "collector") | nindent 2}}

  signalfx:
    endpoint: 0.0.0.0:9943
    access_token_passthrough: true

# By default k8sattributes, memory_limiter and batch processors enabled.
processors:
  {{- include "splunk-otel-collector.k8sAttributesProcessor" . | nindent 2 }}

  {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
  {{- include "splunk-otel-collector.k8sAttributesSplunkPlatformMetrics" . | nindent 2 }}
    filter:
      node_from_env_var: K8S_NODE_NAME
  {{- if .Values.splunkPlatform.sourcetype }}
  {{- include "splunk-otel-collector.resourceMetricsProcessor" . | nindent 2 }}
  {{- end }}
  {{- end }}

  {{- include "splunk-otel-collector.resourceLogsProcessor" . | nindent 2 }}
  {{- if .Values.autodetect.istio }}
  {{- include "splunk-otel-collector.transformLogsProcessor" . | nindent 2 }}
  {{- end }}
  {{- include "splunk-otel-collector.filterLogsProcessors" . | nindent 2 }}

  {{- include "splunk-otel-collector.otelMemoryLimiterConfig" . | nindent 2 }}

  batch:
    metadata_keys:
      - X-SF-Token

  {{- include "splunk-otel-collector.resourceDetectionProcessor" . | nindent 2 }}
  {{- if eq (include "splunk-otel-collector.autoDetectClusterName" .) "true" }}
  {{- include "splunk-otel-collector.resourceDetectionProcessorKubernetesClusterName" . | nindent 2 }}
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

  # It's important to put this processor after resourcedetection to make sure that
  # k8s.name.cluster attribute is always set to "{{ .Values.clusterName }}" when
  # it's declared.
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

  # The following processor is used to add "otelcol.service.mode" attribute to the internal metrics
  resource/add_mode:
    attributes:
      - action: insert
        value: "gateway"
        key: otelcol.service.mode

exporters:
  {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
  signalfx:
    ingest_url: {{ include "splunk-otel-collector.o11yIngestUrl" . }}
    api_url: {{ include "splunk-otel-collector.o11yApiUrl" . }}
    access_token: ${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}
    sending_queue:
      num_consumers: 32
  # To send entities (applicable only if discovery mode is enabled)
  otlphttp/entities:
    logs_endpoint: {{ include "splunk-otel-collector.o11yIngestUrl" . }}/v3/event
    auth:
      authenticator: headers_setter
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.o11yTracesEnabled" .) "true") }}
  {{- include "splunk-otel-collector.otlpHttpExporter" . | nindent 2 }}
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

{{- if and
  (or (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq (include "splunk-otel-collector.profilingEnabled" .) "true"))
  (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true")
}}
connectors:
  # Routing connector to separate entity events from regular logs
  routing/logs:
    default_pipelines: [logs]
    table:
      - context: log
        condition: instrumentation_scope.attributes["otel.entity.event_as_log"] == true
        pipelines: [logs/entities]
{{- end }}

service:
  telemetry:
    resource:
      service.name: otel-collector
    metrics:
      readers:
        - pull:
            exporter:
              prometheus:
                host: localhost
                port: 8889
                without_scope_info: true
                without_units: true
                without_type_suffix: true
  extensions:
    - health_check
    - headers_setter
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
        {{- if eq (include "splunk-otel-collector.autoDetectClusterName" .) "true" }}
        - resourcedetection/k8s_cluster_name
        {{- end }}
        {{- if .Values.clusterName }}
        - resource/add_cluster_name
        {{- end }}
        {{- if .Values.extraAttributes.custom }}
        - resource/add_custom_attrs
        {{- end }}
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yTracesEnabled" .) "true") }}
        - otlphttp
        - signalfx
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
        {{- if eq (include "splunk-otel-collector.autoDetectClusterName" .) "true" }}
        - resourcedetection/k8s_cluster_name
        {{- end }}
        {{- if .Values.clusterName }}
        - resource/add_cluster_name
        {{- end }}
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

    {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
    # entity events
    logs/entities:
      receivers:
        {{- if or (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq (include "splunk-otel-collector.profilingEnabled" .) "true") }}
        - routing/logs
        {{- else }}
        - otlp
        {{- end }}
      processors: [memory_limiter, batch]
      exporters: [otlphttp/entities]
    {{- end }}
    {{- if or (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq (include "splunk-otel-collector.profilingEnabled" .) "true") }}
    {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
    logs/split:
      receivers: [otlp]
      exporters: [routing/logs]
    {{- end }}
    # default logs + profiling data pipeline
    logs:
      receivers:
        {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
        - routing/logs
        {{- else }}
        - otlp
        {{- end }}
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
        - resource/add_mode
        {{- if .Values.clusterName }}
        - resource/add_cluster_name
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - k8sattributes/metrics
        {{- if .Values.splunkPlatform.sourcetype }}
        - resource/metrics
        {{- end }}
        {{- end }}
      exporters:
        {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}
    {{- end }}
{{- end }}
