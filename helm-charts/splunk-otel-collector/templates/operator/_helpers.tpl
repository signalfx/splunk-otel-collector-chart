{{- define "validation-rules" -}}
{{- $tracesEnabled := or (include "splunk-otel-collector.platformTracesEnabled" .) (include "splunk-otel-collector.o11yTracesEnabled" .) -}}
{{- $endpointOverridden := and .Values.operator.instrumentation.spec .Values.operator.instrumentation.spec.exporter .Values.operator.instrumentation.spec.exporter.endpoint (ne .Values.operator.instrumentation.spec.exporter.endpoint "") -}}
{{- if and .Values.operator.enabled $tracesEnabled (not $endpointOverridden) (not (default "" .Values.environment)) -}}
  {{- fail "When operator.enabled=true, (splunkPlatform.tracesEnabled=true or splunkObservability.tracesEnabled=true), (agent.enabled=true or gateway.enabled=true), then environment must be a non-empty string" -}}
{{- end -}}
{{- end -}}

{{/*
Define an endpoint for auto-instrumentation related telemetry data exporting
*/}}
{{- define "splunk-otel-collector.operator.instrumentation.exporter.endpoint" -}}
  {{- if and
    .Values.operator.instrumentation.spec
    .Values.operator.instrumentation.spec.exporter
    .Values.operator.instrumentation.spec.exporter.endpoint
    (ne .Values.operator.instrumentation.spec.exporter.endpoint "")
  }}
    {{ .Values.operator.instrumentation.spec.exporter.endpoint | trim }}
  {{- else if .Values.agent.enabled }}
    http://$(SPLUNK_OTEL_AGENT):4317
  {{- else if .Values.gateway.enabled }}
    http://{{ include "splunk-otel-collector.fullname" . }}:4317
  {{- else }}
    {{- fail "When operator.enabled=true, (splunkPlatform.tracesEnabled=true or splunkObservability.tracesEnabled=true), either agent.enabled=true, gateway.enabled=true, or .Values.operator.instrumentation.spec.exporter.endpoint must be set" -}}
  {{- end }}
{{- end }}


{{- define "splunk-otel-collector.operator.instrumentation.spec-base" -}}
exporter:
  endpoint: {{- include "splunk-otel-collector.operator.instrumentation.exporter.endpoint" . | nindent 4 }}
env:
  {{- if .Values.agent.enabled }}
  - name: SPLUNK_OTEL_AGENT
    valueFrom:
      fieldRef:
        apiVersion: v1
        fieldPath: status.hostIP
  {{- end }}
  {{- if .Values.splunkObservability.profilingEnabled }}
  - name: SPLUNK_PROFILER_ENABLED
    value: "true"
  - name: SPLUNK_PROFILER_MEMORY_ENABLED
    value: "true"
  {{- end }}
{{- if include "splunk-otel-collector.operator.instrumentation.exporter.endpoint" . | toString | hasSuffix ":4317" }}
# Required if endpoint is set to 4317.
# Python and dotnet auto-instrumentation uses http/proto by default, so data must be sent to 4318 instead of 4317.
# # See: https://github.com/open-telemetry/opentelemetry-operator#opentelemetry-auto-instrumentation-injection
python:
  env:
    - name: OTEL_EXPORTER_OTLP_ENDPOINT
      value: {{- include "splunk-otel-collector.operator.instrumentation.exporter.endpoint" . | replace ":4317" ":4318" | nindent 6 }}
dotnet:
  env:
    - name: OTEL_EXPORTER_OTLP_ENDPOINT
      value: {{- include "splunk-otel-collector.operator.instrumentation.exporter.endpoint" . | replace ":4317" ":4318" | nindent 6 }}
{{- end }}
{{- end }}

{{- define "extract-image-name" -}}
{{- $repository := . -}}
{{- (splitList "/" $repository) | last -}}
{{- end -}}

{{/* Helper to construct the OTEL_RESOURCE_ATTRIBUTES value */}}
{{- define "construct-otel-resource-attributes" -}}
{{- $repository := index . "repository" -}}
{{- $tag := index . "tag" -}}
{{- $customAttributes := index . "customAttributes" -}}
{{- $defaultAttribute := printf "splunk.zc.method=%s:%s" (include "extract-image-name" $repository) $tag -}}
{{- if $customAttributes -}}
{{- print (printf "%s,%s" $customAttributes $defaultAttribute) -}}
{{- else -}}
{{- print $defaultAttribute -}}
{{- end -}}
{{- end -}}

{{/* Helper to merge custom env variables with default OTEL_RESOURCE_ATTRIBUTES */}}
{{- define "merge-custom-env" -}}
{{- $envList := index . "envList" -}}
{{- $otelResourceAttr := index . "otelResourceAttr" -}}
- name: OTEL_RESOURCE_ATTRIBUTES
  value: {{ include "construct-otel-resource-attributes" $otelResourceAttr }}
{{- range $env := $envList }}
{{- if ne $env.name "OTEL_RESOURCE_ATTRIBUTES" }}
- name: {{ $env.name }}
  value: {{ $env.value }}
{{- end -}}
{{- end -}}
{{- end -}}

{{/* Helper to convert a list of dictionaries to a list of keys */}}
{{- define "get-dict-keys" -}}
  {{- $listOfDicts := . }}
  {{- $keyList := list }}
  {{- range $listOfDicts }}
    {{- $keyList = append $keyList .name }}
  {{- end }}
  {{- $keyList }}
{{- end }}
