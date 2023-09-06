{{/*
Define validation rules to ensure the correct usage of the operator.
- Check for a valid endpoint: The endpoint can either be derived from the agent/gateway or provided by the user.
*/}}
{{- define "splunk-otel-collector.operator.validation-rules" -}}
{{- $tracesEnabled := or (include "splunk-otel-collector.platformTracesEnabled" .) (include "splunk-otel-collector.o11yTracesEnabled" .) -}}
{{- $endpointOverridden := and .Values.operator.instrumentation.spec .Values.operator.instrumentation.spec.exporter .Values.operator.instrumentation.spec.exporter.endpoint (ne .Values.operator.instrumentation.spec.exporter.endpoint "") -}}
{{- if and .Values.operator.enabled $tracesEnabled (not $endpointOverridden) (not (default "" .Values.environment)) -}}
  {{- fail "When operator.enabled=true, (splunkPlatform.tracesEnabled=true or splunkObservability.tracesEnabled=true), (agent.enabled=true or gateway.enabled=true), then environment must be a non-empty string" -}}
{{- end -}}
{{- end -}}

{{/*
Define an endpoint for exporting telemetry data related to auto-instrumentation.
- Order of precedence for the endpoint value:
  1. User-defined value
  2. Agent endpoint
  3. Gateway endpoint
*/}}
{{- define "splunk-otel-collector.operator.instrumentation-exporter-endpoint" -}}
  {{- $endpoint := "" }}
  {{- if and
    .Values.operator.instrumentation.spec
    .Values.operator.instrumentation.spec.exporter
    .Values.operator.instrumentation.spec.exporter.endpoint
    (ne .Values.operator.instrumentation.spec.exporter.endpoint "")
  }}
    {{ $endpoint = .Values.operator.instrumentation.spec.exporter.endpoint }}
  {{- else if .Values.agent.enabled }}
    {{- $endpoint = "http://$(SPLUNK_OTEL_AGENT):4317" }}
  {{- else if .Values.gateway.enabled }}
     {{- $endpoint = printf "http://%s:4317" (include "splunk-otel-collector.fullname" .) }}
  {{- else }}
    {{- fail "When operator.enabled=true, (splunkPlatform.tracesEnabled=true or splunkObservability.tracesEnabled=true), either agent.enabled=true, gateway.enabled=true, or .Values.operator.instrumentation.spec.exporter.endpoint must be set" -}}
  {{- end }}
  {{- printf "%s" $endpoint }}
{{- end }}

{{/*
Define entries for instrumentation libraries with the following key features:
- Dynamic Value Generation: Allows for easy addition of new libraries.
- Custom Environment Variables: Each library can be customized with specific attributes or use-cases.
- Broad Support: Compatible with both native OpenTelemetry and Splunk-specific libraries.
- Comprehensive Output: The final output combines user input with chart defaults for a complete configuration.
*/}}
{{- define "splunk-otel-collector.operator.instrumentation-libraries" }}
  {{- $defaultEndpoint := include "splunk-otel-collector.operator.instrumentation-exporter-endpoint" $ }}
  {{/* Iterate over each specified instrumentation library */}}
  {{- if .Values.operator.instrumentation.spec }}
    {{- range $key, $value := .Values.operator.instrumentation.spec }}
      {{/* Check for required fields */}}
      {{- if and $value.repository $value.tag }}

        {{/* Generate YAML for each library */}}
        {{- printf "%s:\n" $key | indent 2 }}

        {{- printf "image: %s\n" (include "splunk-otel-collector.operator.extract-image-name" $value.repository) | indent 4 }}

        {{- printf "env:\n" | indent 4 }}

        {{/* Instrumentation library environment variables */}}
        {{- include "splunk-otel-collector.operator.extract-instrumentation-env" $value | indent 6 }}

        {{/* OTEL exporter endpoint */}}
        {{- $customEndpoint := include "splunk-otel-collector.operator.custom-exporter-endpoint" (dict "key" $key "env" $value.env "default" $defaultEndpoint) }}
        {{- if $customEndpoint }}
          {{- printf "- name: OTEL_EXPORTER_OTLP_ENDPOINT\n  value: %s\n" $customEndpoint | indent 6 }}
        {{- end }}

      {{- end }}
    {{- end }}
  {{- end }}

{{- end }}

{{/*
Helper to extract the image name from a repository URL.
- Takes the repository URL as input and returns the last part as the image name.
*/}}
{{- define "splunk-otel-collector.operator.extract-image-name" -}}
{{- $repository := . -}}
{{- (splitList "/" $repository) | last -}}
{{- end -}}

{{/*
Helper to convert a list of dictionaries into a list of keys.
- Iterates through a list of dictionaries and collects the 'name' field from each.
- Returns a list of these 'name' keys.
*/}}
{{- define "splunk-otel-collector.operator.extract-name-keys-from-dict-list" -}}
  {{- $listOfDicts := . }}
  {{- $keyList := list }}
  {{- range $listOfDicts }}
    {{- $keyList = append $keyList .name }}
  {{- end }}
  {{- $keyList }}
{{- end }}

{{/*
Helper for generating custom OTEL exporter endpoint.
*/}}
{{- define "splunk-otel-collector.operator.custom-exporter-endpoint" }}
  {{- $customOtelExporterEndpoint := "" }}
  {{- if or (eq .key "dotnet") (eq .key "python") }}
    {{- $customOtelExporterEndpoint = .default | replace ":4317" ":4318" }}
  {{- end }}
  {{- if .env }}
    {{- range $env := .env }}
      {{- if eq $env.name "OTEL_EXPORTER_OTLP_ENDPOINT" }}
        {{- $customOtelExporterEndpoint = $env.value }}
      {{- end }}
    {{- end }}
  {{- end }}
  {{- if $customOtelExporterEndpoint }}
    {{- $customOtelExporterEndpoint }}
  {{- end }}
{{- end }}

{{/*
Helper for including user-supplied environment variables.
*/}}
{{- define "splunk-otel-collector.operator.extract-instrumentation-env" }}
  {{- $otelResourceAttributes := printf "splunk.zc.method=%s:%s" (include "splunk-otel-collector.operator.extract-image-name" .repository) .tag }}
  {{- range $env := .env }}
    {{- if eq $env.name "OTEL_RESOURCE_ATTRIBUTES" }}
      {{- $otelResourceAttributes = printf "%s,%s" $env.value $otelResourceAttributes }}
    {{- else }}
      {{- printf "- name: %s\n  value: %s\n" $env.name $env.value }}
    {{- end }}
  {{- end }}
  {{- $otelResourceAttributes }}
{{- end }}
