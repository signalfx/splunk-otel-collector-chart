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
  {{- $defaultEndpoint := include "splunk-otel-collector.operator.instrumentation-exporter-endpoint" $ -}}
  {{- /* Iterate over each specified instrumentation library */ -}}
  {{- if .Values.operator.instrumentation.spec -}}
    {{- range $key, $value := .Values.operator.instrumentation.spec -}}
      {{- /* Check for required fields to determine if it is an  instrumentation library */ -}}
      {{- if and $value.repository $value.tag -}}

        {{- /* Generate YAML for each instrumentation library */ -}}
        {{- printf "%s:\n" $key | indent 2 -}}

        {{- printf "image: %s\n" (include "splunk-otel-collector.operator.extract-image-name" $value.repository) | indent 2 -}}

        {{- /* Instrumentation library environment variables */}}
        {{- printf "env:" | indent 2 -}}
        {{- include "splunk-otel-collector.operator.extract-instrumentation-env" (dict "key" $key "env" $value.env "endpoint" $defaultEndpoint "repo" $value.repository) -}}

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

{{- end }}

{{/*
Helper for getting environment variables for each instrumentation library.
- Add users supplied environemtn variables
- Add merged default and user supplied values for for OTEL_RESOURCE_ATTRIBUTES
  - Both inputs will be appended together.
- Add speical case values or user supplied values for OTEL_EXPORTER_OTLP_ENDPOINT
  - User supplied values will overrided any default special cases.
*/}}
{{- define "splunk-otel-collector.operator.extract-instrumentation-env" }}
  {{- /* Splunk default resource attribute, it is always included. */}}
  {{- $otelResourceAttributes := printf "splunk.zc.method=%s:%s" (include "splunk-otel-collector.operator.extract-image-name" .repo) .tag }}

  {{- /* Add custom resource attributes */}}
  {{- range $env := .env }}
    {{- if eq $env.name "OTEL_RESOURCE_ATTRIBUTES" }}
      {{- $otelResourceAttributes = printf "%s,%s" $env.value $otelResourceAttributes }}
    {{- else }}
      {{- printf "- name: %s" $env.name | nindent 6 -}}
      {{- printf "value: %s" $env.value | nindent 6 -}}
    {{- end }}
  {{- end }}

  {{- /* Add resource attributes that container standard default and possibly custom values. */}}
  {{- printf "- name: %s" "OTEL_RESOURCE_ATTRIBUTES" | nindent 6 -}}
  {{- printf "value: %s" $otelResourceAttributes | nindent 6 -}}
  {{- printf "\n" -}}

  {{- /* Add custom or default exporter endpoint */}}
  {{- $customOtelExporterEndpoint := "" }}
  {{- if or (eq .key "dotnet") (eq .key "python") }}
    {{- $customOtelExporterEndpoint = .endpoint | replace ":4317" ":4318" }}
  {{- end }}
  {{- if .env }}
    {{- range $env := .env }}
      {{- if eq $env.name "OTEL_EXPORTER_OTLP_ENDPOINT" }}
        {{- $customOtelExporterEndpoint = $env.value }}
      {{- end }}
    {{- end }}
  {{- end }}
  {{- if $customOtelExporterEndpoint }}
    {{- if contains $customOtelExporterEndpoint "4318" }}
      {{- printf "# %s auto-instrumentation uses http/proto by default, so data must be sent to 4318 instead of 4317.\n # See: https://github.com/open-telemetry/opentelemetry-operator#opentelemetry-auto-instrumentation-injection" .key }}
    {{- end }}
    {{- printf "- name: OTEL_EXPORTER_OTLP_ENDPOINT\n  value: %s\n" $customOtelExporterEndpoint | indent 6 }}
    {{- printf "\n" -}}
  {{- end }}
{{- end }}
