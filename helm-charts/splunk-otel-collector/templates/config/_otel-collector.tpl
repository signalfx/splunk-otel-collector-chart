{{/*
Config for the optional standalone collector
The values can be overridden in .Values.gateway.config
*/}}
{{- define "splunk-otel-collector.gatewayConfig" -}}
extensions:
  {{- include "splunk-otel-collector.opampExtension" (merge (dict "forceDirectEndpoint" true) .) | nindent 2 }}
  {{- include "splunk-otel-collector.o11yIngestHttpForwarderExtension" (merge (dict "forceDirectEndpoint" true) .) | nindent 2 }}
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
  {{- include "splunk-otel-collector.renderTenantFileStorage" . | nindent 2 }}

receivers:
  {{- include "splunk-otel-collector.traceReceivers" . | nindent 2 }}

  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
        include_metadata: true
      http:
        # https://github.com/open-telemetry/opentelemetry-collector/blob/9d3a8a4608a7dbd9f787867226a78356ace9b5e4/receiver/otlpreceiver/otlp.go#L140-L152
        endpoint: 0.0.0.0:4318
        include_metadata: true

  # Prometheus receiver scraping metrics from the pod itself
  {{- include "splunk-otel-collector.prometheusInternalMetrics" (dict "receiver" "collector") | nindent 2}}

# By default k8s_attributes, memory_limiter and batch processors enabled.
processors:
  {{- include "splunk-otel-collector.k8sAttributesProcessor" . | nindent 2 }}

  {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
  {{- include "splunk-otel-collector.k8sAttributesSplunkPlatformMetrics" . | nindent 2 }}
    filter:
      node_from_env_var: K8S_NODE_NAME
  {{- if or .Values.splunkPlatform.metricsSourcetype .Values.splunkPlatform.sourcetype }}
  {{- include "splunk-otel-collector.resourceMetricsProcessor" . | nindent 2 }}
  {{- end }}
  {{- end }}

  {{- include "splunk-otel-collector.resourceLogsProcessor" . | nindent 2 }}
  {{- if .Values.autodetect.istio }}
  {{- include "splunk-otel-collector.transformLogsProcessor" . | nindent 2 }}
  {{- end }}
  {{- if eq (include "splunk-otel-collector.containerSourcetypeEnabled" .) "true" }}
  {{- include "splunk-otel-collector.transformSourcetypeProcessor" . | nindent 2 }}
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
  otlp_http/entities:
    logs_endpoint: {{ include "splunk-otel-collector.o11yIngestUrl" . }}/v3/event
    auth:
      authenticator: headers_setter
  {{- if .Values.splunkObservability.secureAppEnabled }}
  otlp_http/secureapp:
    logs_endpoint: {{ include "splunk-otel-collector.o11yIngestUrl" . }}/v3/event
    headers:
      "X-SF-TOKEN": "${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}"
      "X-Splunk-Instrumentation-Library": secureapp
  {{- end }}
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.o11yTracesEnabled" .) "true") }}
  {{- include "splunk-otel-collector.otlpHttpExporter" . | nindent 2 }}
    sending_queue:
      num_consumers: 32
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.o11yProfilingEnabled" .) "true") }}
  splunk_hec/o11y:
    endpoint: {{ include "splunk-otel-collector.o11yIngestUrl" . }}/v1/log
    token: "${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}"
    log_data_enabled: false
    profiling_data_enabled: {{ .Values.splunkObservability.profilingEnabled }}
    sending_queue:
      num_consumers: 32
    # TODO: Performance testing must be done before enabling compression
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

  {{- if eq (include "splunk-otel-collector.multiTenantLogsEnabled" .) "true" }}
  {{- include "splunk-otel-collector.validateTenants" . }}
  {{- range $_, $t := .Values.splunkPlatform.additionalLogsExporters }}
  {{- if eq (default "hec" $t.protocol) "hec" }}
  {{- include "splunk-otel-collector.renderAdditionalHecExporter" (dict "context" $ "tenant" $t) | nindent 2 }}
  {{- else }}
  {{- include "splunk-otel-collector.renderAdditionalOtlpExporter" (dict "context" $ "tenant" $t) | nindent 2 }}
  {{- end }}
  {{- end }}
  {{- end }}

{{- $gwO11yRouting := and
  (or (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq (include "splunk-otel-collector.profilingEnabled" .) "true") (.Values.splunkObservability.secureAppEnabled))
  (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") -}}
{{- $gwMultiTenant := eq (include "splunk-otel-collector.multiTenantLogsEnabled" .) "true" -}}
{{- if or $gwO11yRouting $gwMultiTenant }}
connectors:
  {{- if $gwO11yRouting }}
  # Routing connector to separate entity events from regular logs
  routing/logs:
    default_pipelines: [logs]
    table:
      - context: log
        condition: instrumentation_scope.attributes["otel.entity.event_as_log"] == true
        pipelines: [logs/entities]
      {{- if .Values.splunkObservability.secureAppEnabled }}
      - context: log
        condition: instrumentation_scope.name == "secureapp"
        pipelines: [logs/secureapp]
      {{- end }}
  {{- end }}
  {{- if $gwMultiTenant }}
  {{- $routing := .Values.splunkPlatform.logsRouting }}
  {{- $attrSrc := default "resource" $routing.attributeSource }}
  routing/tenants:
    default_pipelines: [logs/export_{{ replace "-" "_" (default "default" $routing.defaultExporter) }}]
    table:
      {{- range $i, $e := default (list) $routing.table }}
      - context: {{ $attrSrc }}
        {{- if eq $attrSrc "resource" }}
        condition: 'attributes["{{ $routing.fromAttribute }}"] == "{{ $e.value }}"'
        {{- else }}
        condition: 'resource.attributes["{{ $routing.fromAttribute }}"] == "{{ $e.value }}"'
        {{- end }}
        pipelines: [logs/export_{{ replace "-" "_" $e.exporter }}]
      {{- end }}
  {{- end }}
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
    - http_forwarder/opamp_splunk_o11y
    - opamp/splunk_o11y
    {{- end }}
    {{- if and (eq (include "splunk-otel-collector.multiTenantLogsEnabled" .) "true") .Values.splunkPlatform.sendingQueue.persistentQueue.enabled }}
    {{- range $_, $t := .Values.splunkPlatform.additionalLogsExporters }}
    - file_storage/persistent_queue_{{ replace "-" "_" $t.name }}
    {{- end }}
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
        - k8s_attributes
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
        - otlp_http
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformTracesEnabled" .) "true") }}
        - splunk_hec/platform_traces
        {{- end }}
    {{- end }}

    {{- if (eq (include "splunk-otel-collector.metricsEnabled" .) "true") }}
    # default metrics pipeline
    metrics:
      receivers: [otlp]
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
        - k8s_attributes/metrics
        {{- if or .Values.splunkPlatform.metricsSourcetype .Values.splunkPlatform.sourcetype }}
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

    {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
    # entity events
    logs/entities:
      receivers:
        {{- if or (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq (include "splunk-otel-collector.profilingEnabled" .) "true")  (.Values.splunkObservability.secureAppEnabled) }}
        - routing/logs
        {{- else }}
        - otlp
        {{- end }}
      processors: [memory_limiter, batch]
      exporters: [otlp_http/entities]
    {{- if .Values.splunkObservability.secureAppEnabled }}
    # secureapp events
    logs/secureapp:
      receivers:
        - routing/logs
      processors: [memory_limiter, batch]
      exporters: [otlp_http/secureapp]
    {{- end }}
    {{- end }}
    {{- if or (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq (include "splunk-otel-collector.profilingEnabled" .) "true") (.Values.splunkObservability.secureAppEnabled)}}
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
        - k8s_attributes
        - filter/logs
        - batch
        {{- if .Values.autodetect.istio }}
        - transform/istio_service_name
        {{- end }}
        {{- if eq (include "splunk-otel-collector.containerSourcetypeEnabled" .) "true" }}
        - transform/sourcetype
        {{- end }}
        - resource/logs
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yProfilingEnabled" .) "true") }}
        - splunk_hec/o11y
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") }}
        {{- if eq (include "splunk-otel-collector.multiTenantLogsEnabled" .) "true" }}
        - routing/tenants
        {{- else }}
        - splunk_hec/platform_logs
        {{- end }}
        {{- end }}
    {{- end }}

    {{- if eq (include "splunk-otel-collector.multiTenantLogsEnabled" .) "true" }}
    {{- /* Per-tenant export pipelines fed by the routing/tenants connector. */}}
    logs/export_default:
      receivers: [routing/tenants]
      processors: [memory_limiter]
      exporters: [splunk_hec/platform_logs]
    {{- range $_, $t := .Values.splunkPlatform.additionalLogsExporters }}
    logs/export_{{ replace "-" "_" $t.name }}:
      receivers: [routing/tenants]
      processors: [memory_limiter]
      exporters: [{{ include "splunk-otel-collector.tenantExporterName" $t }}]
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
        - k8s_attributes/metrics
        {{- if or .Values.splunkPlatform.metricsSourcetype .Values.splunkPlatform.sourcetype }}
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
