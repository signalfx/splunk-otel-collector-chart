{{- define "validation-rules" -}}
{{- $tracesEnabled := or (include "splunk-otel-collector.platformTracesEnabled" .) (include "splunk-otel-collector.o11yTracesEnabled" .) -}}
{{- $endpointOverridden := and .Values.operator.instrumentation.spec .Values.operator.instrumentation.spec.exporter .Values.operator.instrumentation.spec.exporter.endpoint (ne .Values.operator.instrumentation.spec.exporter.endpoint "") -}}
{{- if and .Values.operator.enabled $tracesEnabled (not $endpointOverridden) -}}
  {{/*  If no endpoint override was provided, the environment variable/tag must be set by the agent, gateway, or instrumentation.*/}}
  {{- if or (not .Values.environment) (eq .Values.environment "") -}}
    {{- $envSet := false -}}
    {{- if and .Values.operator.instrumentation.spec .Values.operator.instrumentation.spec.env -}}
      {{- range .Values.operator.instrumentation.spec.env -}}
        {{- if and (eq .name "OTEL_RESOURCE_ATTRIBUTES") .value -}}
          {{- $envSet = true -}}
        {{- end -}}
      {{- end -}}
    {{- end -}}
    {{- if not $envSet -}}
      {{- fail "When operator.enabled=true, (splunkPlatform.tracesEnabled=true or splunkObservability.tracesEnabled=true), (agent.enabled=true or gateway.enabled=true), and .Values.operator.instrumentation.spec.exporter.endpoint is not set, either environment must be a non-empty string or operator.instrumentation.spec.env must contain an item with {name: OTEL_RESOURCE_ATTRIBUTES, value: non-empty string}" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{- define "splunk-otel-collector.operator.instrumentation.exporter.endpoint" -}}
{{- if and .Values.operator.instrumentation.spec .Values.operator.instrumentation.spec.exporter .Values.operator.instrumentation.spec.exporter.endpoint (ne .Values.operator.instrumentation.spec.exporter.endpoint "") }}
  "{{ .Values.operator.instrumentation.spec.exporter.endpoint }}"
{{- else if .Values.agent.enabled }}
  "http://$(SPLUNK_OTEL_AGENT):4317"
{{- else if .Values.gateway.enabled }}
  "http://{{ include "splunk-otel-collector.fullname" . }}:4317"
{{- else -}}
  {{- fail "When operator.enabled=true, (splunkPlatform.tracesEnabled=true or splunkObservability.tracesEnabled=true), (agent.enabled=true or gateway.enabled=true), and .Values.operator.instrumentation.spec.exporter.endpoint is not set, either environment must be a non-empty string or operator.instrumentation.spec.env must contain an item with {name: OTEL_RESOURCE_ATTRIBUTES, value: non-empty string}" -}}
{{- end }}
{{- end }}
