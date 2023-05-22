{{- define "splunk-otel-collector.operator.instrumentation.spec" -}}
{{- if .Values.operator.enabled }}
exporter:
  endpoint: {{- include "splunk-otel-collector.operator.instrumentation.exporter.endpoint" . | nindent 4 }}
propagators:
  - tracecontext
  - baggage
  - b3
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
python:
  env:
    # Required if endpoint is set to 4317.
    # Python auto-instrumentation uses http/proto by default, so data must be sent to 4318 instead of 4317.
    # See: https://github.com/open-telemetry/opentelemetry-operator#opentelemetry-auto-instrumentation-injection
    - name: OTEL_EXPORTER_OTLP_ENDPOINT
      value: {{- include "splunk-otel-collector.operator.instrumentation.exporter.endpoint" . | replace "4317" "4318" | nindent 6 }}
dotnet:
  env:
    # Required if endpoint is set to 4317.
    # Dotnet auto-instrumentation uses http/proto by default, so data must be sent to 4318 instead of 4317.
    # See: https://github.com/open-telemetry/opentelemetry-operator#opentelemetry-auto-instrumentation-injection
    - name: OTEL_EXPORTER_OTLP_ENDPOINT
      value: {{- include "splunk-otel-collector.operator.instrumentation.exporter.endpoint" . | replace "4317" "4318" | nindent 6 }}
{{- end }}
{{- end }}
