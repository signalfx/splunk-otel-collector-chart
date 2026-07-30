{{/*
Experimental Splunk Platform log-routing Collector components.

Generated data flow:
  existing logs pipeline -> routing/platform_logs -> one output pipeline -> HEC

Review and verification:
  values:    examples/multi-tenant-platform-logs/*-values.yaml
  output:    examples/multi-tenant-platform-logs/rendered_manifests/configmap-agent.yaml
  assertions: unittests/logs_routing*_test.yaml

Names, inherited values, and validation live in
_platform-logs-routing-helpers.tpl. This file contains only emitted Collector
configuration.
*/}}

{{/* Explicit route metadata is applied last so it is authoritative. */}}
{{- define "splunk-otel-collector.platformLogsRouteMetadataProcessors" -}}
{{- range $name, $route := .Values.splunkPlatform.logsRouting.routes }}
{{- if eq (include "splunk-otel-collector.platformLogsRouteHasMetadata" $route) "true" }}
{{ include "splunk-otel-collector.platformLogsRouteMetadataProcessorName" $name }}:
  log_statements:
    - context: log
      statements:
        {{- if hasKey $route "index" }}
        - {{ printf "set(resource.attributes[\"com.splunk.index\"], %s)" ($route.index | toJson) | quote }}
        - {{ printf "set(attributes[\"com.splunk.index\"], %s)" ($route.index | toJson) | quote }}
        {{- end }}
        {{- if hasKey $route "source" }}
        - {{ printf "set(resource.attributes[\"com.splunk.source\"], %s)" ($route.source | toJson) | quote }}
        - {{ printf "set(attributes[\"com.splunk.source\"], %s)" ($route.source | toJson) | quote }}
        {{- end }}
        {{- if hasKey $route "sourcetype" }}
        - {{ printf "set(resource.attributes[\"com.splunk.sourcetype\"], %s)" ($route.sourcetype | toJson) | quote }}
        - {{ printf "set(attributes[\"com.splunk.sourcetype\"], %s)" ($route.sourcetype | toJson) | quote }}
        {{- end }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Additional Splunk Platform HEC exporter for one log route.
*/}}
{{- define "splunk-otel-collector.splunkPlatformLogsRouteExporter" -}}
{{- $ctx := .context -}}
{{- $name := .name -}}
{{- $route := .route -}}
{{- $platform := $ctx.Values.splunkPlatform -}}
{{- $endpoint := default $platform.endpoint $route.endpoint -}}
{{- $index := default $platform.index $route.index -}}
{{- $source := default $platform.source $route.source -}}
{{- $sourcetype := default $platform.sourcetype $route.sourcetype -}}
{{- $queue := default (dict) $route.sendingQueue -}}
{{- $queueEnabled := $platform.sendingQueue.enabled -}}
{{- if and (hasKey $queue "enabled") (ne (toString $queue.enabled) "<nil>") -}}
  {{- $queueEnabled = $queue.enabled -}}
{{- end -}}
{{- $queueConsumers := default $platform.sendingQueue.numConsumers $queue.numConsumers -}}
{{- $queueSize := default $platform.sendingQueue.queueSize $queue.queueSize -}}
{{- $retry := default (dict) $route.retryOnFailure -}}
{{- $retryEnabled := $platform.retryOnFailure.enabled -}}
{{- if and (hasKey $retry "enabled") (ne (toString $retry.enabled) "<nil>") -}}
  {{- $retryEnabled = $retry.enabled -}}
{{- end -}}
{{- $retryInitial := default $platform.retryOnFailure.initialInterval $retry.initialInterval -}}
{{- $retryMax := default $platform.retryOnFailure.maxInterval $retry.maxInterval -}}
{{- $retryElapsed := default $platform.retryOnFailure.maxElapsedTime $retry.maxElapsedTime -}}
splunk_hec/platform_logs/{{ $name }}:
  endpoint: {{ $endpoint | quote }}
  token: {{ include "splunk-otel-collector.platformLogsRouteTokenRef" (dict "name" $name "route" $route) | quote }}
  index: {{ $index | quote }}
  source: {{ $source | quote }}
  {{- if $sourcetype }}
  sourcetype: {{ $sourcetype | quote }}
  {{- end }}
  max_idle_conns: {{ $platform.maxConnections }}
  max_idle_conns_per_host: {{ $platform.maxConnections }}
  disable_compression: {{ $platform.disableCompression }}
  timeout: {{ $platform.timeout }}
  idle_conn_timeout: {{ $platform.idleConnTimeout }}
  splunk_app_name: {{ $ctx.Chart.Name }}-chart
  splunk_app_version: {{ $ctx.Chart.Version }}
  profiling_data_enabled: false
  tls:
    insecure_skip_verify: {{ $platform.insecureSkipVerify }}
    {{- if $platform.clientCert }}
    cert_file: /otel/etc/splunk_platform_hec_client_cert
    {{- end }}
    {{- if $platform.clientKey }}
    key_file: /otel/etc/splunk_platform_hec_client_key
    {{- end }}
    {{- if $platform.caFile }}
    ca_file: /otel/etc/splunk_platform_hec_ca_file
    {{- end }}
  retry_on_failure:
    enabled: {{ $retryEnabled }}
    initial_interval: {{ $retryInitial }}
    max_interval: {{ $retryMax }}
    {{- if $ctx.Values.featureGates.noDropLogsPipeline }}
    max_elapsed_time: 0s
    {{- else }}
    max_elapsed_time: {{ $retryElapsed }}
    {{- end }}
  sending_queue:
    enabled: {{ $queueEnabled }}
    {{- if .addPersistentStorage }}
    storage: {{ include "splunk-otel-collector.platformLogsRouteStorageName" $name }}
    {{- end }}
    num_consumers: {{ $queueConsumers }}
    {{- if $ctx.Values.featureGates.noDropLogsPipeline }}
    sizer: items
    {{- end }}
    {{- if $ctx.Values.featureGates.noDropLogsPipeline }}
    block_on_overflow: true
    {{- end }}
    {{- if $ctx.Values.featureGates.noDropLogsPipeline }}
    batch:
      flush_timeout: 200ms
      min_size: 2048
    queue_size: {{ max 10000 $queueSize }}
    {{- else }}
    queue_size: {{ $queueSize }}
    {{- end }}
{{- end }}

{{/*
Routing connector for normal Splunk Platform logs.
*/}}
{{- define "splunk-otel-collector.platformLogsRoutingConnector" -}}
routing/platform_logs:
  default_pipelines: [logs/platform/default]
  table:
    {{- $attribute := .Values.splunkPlatform.logsRouting.attribute }}
    {{- range $name, $route := .Values.splunkPlatform.logsRouting.routes }}
    - context: log
      condition: resource.attributes[{{ $attribute | toJson }}] == {{ default $name $route.value | toJson }}
      pipelines: [logs/platform/{{ $name }}]
    {{- end }}
{{- end }}

{{/*
Export-only pipelines fed by routing/platform_logs.
*/}}
{{- define "splunk-otel-collector.platformLogsRoutingPipelines" -}}
logs/platform/default:
  receivers: [routing/platform_logs]
  processors: {{ default (list) .Values.splunkPlatform.logsRouting.defaultProcessors | toJson }}
  exporters: [splunk_hec/platform_logs]
{{- range $name, $route := .Values.splunkPlatform.logsRouting.routes }}
logs/platform/{{ $name }}:
  receivers: [routing/platform_logs]
  {{- $processors := default (list) $route.processors }}
  {{- if eq (include "splunk-otel-collector.platformLogsRouteHasMetadata" $route) "true" }}
    {{- $processors = append $processors (include "splunk-otel-collector.platformLogsRouteMetadataProcessorName" $name) }}
  {{- end }}
  processors: {{ $processors | toJson }}
  exporters: [splunk_hec/platform_logs/{{ $name }}]
{{- end }}
{{- end }}

{{/*
Isolated direct-agent file storage for one route queue.
*/}}
{{- define "splunk-otel-collector.platformLogsRouteFileStorage" -}}
{{ include "splunk-otel-collector.platformLogsRouteStorageName" .name }}:
  directory: {{ .context.Values.splunkPlatform.sendingQueue.persistentQueue.storagePath }}/agent/{{ .name }}
  create_directory: true
  timeout: 0
  {{- if not (eq (toString .context.Values.splunkPlatform.fsyncEnabled) "<nil>") }}
  fsync: {{ .context.Values.splunkPlatform.fsyncEnabled }}
  {{- end }}
  compaction:
    on_rebound: true
    rebound_needed_threshold_mib: 200
    rebound_trigger_threshold_mib: 100
    directory: {{ .context.Values.splunkPlatform.sendingQueue.persistentQueue.storagePath }}/agent/{{ .name }}
    cleanup_on_start: true
{{- end }}
