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

{{/*
Define entries for instrumentation libraries with the following key features:
- Dynamic Value Generation: Allows for easy addition of new libraries.
- Custom Environment Variables: Each library can be customized with specific attributes or use-cases.
- Broad Support: Compatible with both native OpenTelemetry and Splunk-specific libraries.
- Comprehensive Output: The final output combines user input with chart defaults for a complete configuration.
*/}}
{{- define "splunk-otel-collector.operator.instrumentation-libraries" -}}
  {{- /* Include instrumentation libraries with environment variables */ -}}
  {{- if .Values.operator.instrumentation.spec }}
    {{- range $key, $value := .Values.operator.instrumentation.spec }}
      {{- if and $value.repository $value.tag }}
        {{- $imageName := include "splunk-otel-collector.operator.extract-image-name" $value.repository }}
        {{- /* Needed to add user supplied and chart default otel resource attributes */ -}}
        {{- $instEnvList := list }}
        {{- $defaultOtelResourceAttributes := printf "splunk.zc.method=%s:%s" $imageName $value.tag }}
        {{- $customOtelResourceAttributes := "" }}
        {{- if $value.env }}
          {{- $instEnvList = $value.env }}
          {{- range $env := $value.env }}
            {{- if eq $env.name "OTEL_RESOURCE_ATTRIBUTES" }}
              {{- $customOtelResourceAttributes = printf "%s,%s" $env.value $defaultOtelResourceAttributes }}
            {{- end }}
          {{- end }}
        {{- end }}
        {{- if not $customOtelResourceAttributes }}
          {{- $customOtelResourceAttributes = $defaultOtelResourceAttributes }}
        {{- end }}
  {{ $key }}:
    image: {{ printf "%s:%s" $value.repository $value.tag }}
    env:
      {{- /* Append additional user supplied env variables */ -}}
      {{- range $env := $instEnvList }}
      {{- if ne $env.name "OTEL_RESOURCE_ATTRIBUTES" }}
      - name: {{ $env.name }}
        value: {{ $env.value }}
      {{- end }}
      {{- end }}
      - name: OTEL_RESOURCE_ATTRIBUTES
        value: {{ $customOtelResourceAttributes }}
      {{- /* Append special instrumentation library env variables */ -}}
      {{- /* Insert a special endpoint value if not overriden by the user */ -}}
      {{- if and (eq $key "dotnet") (not (hasKey (include "splunk-otel-collector.operator.get-dict-keys" $instEnvList) "OTEL_EXPORTER_OTLP_ENDPOINT" )) }}
      # Dotnet auto-instrumentation uses http/proto by default, so data must be sent to 4318 instead of 4317.
      # See: https://github.com/open-telemetry/opentelemetry-operator#opentelemetry-auto-instrumentation-injection
      - name: OTEL_EXPORTER_OTLP_ENDPOINT
        value: {{ include "splunk-otel-collector.operator.instrumentation.exporter.endpoint" $ | trim | replace ":4317" ":4318" }}
      {{- end }}
      {{- /* Insert a special endpoint value if not overriden by the user */ -}}
      {{- if and (eq $key "python") (not (hasKey (include "splunk-otel-collector.operator.get-dict-keys" $instEnvList) "OTEL_EXPORTER_OTLP_ENDPOINT" )) }}
      # Python auto-instrumentation uses http/proto by default, so data must be sent to 4318 instead of 4317.
      # See: https://github.com/open-telemetry/opentelemetry-operator#opentelemetry-auto-instrumentation-injection
      - name: OTEL_EXPORTER_OTLP_ENDPOINT
        value: {{ include "splunk-otel-collector.operator.instrumentation.exporter.endpoint" $ | trim | replace ":4317" ":4318" }}
      {{- end }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}

{{/*
Define a helper to extract the image name from a repository URL.
- Takes the repository URL as input and returns the last part as the image name.
*/}}
{{- define "splunk-otel-collector.operator.extract-image-name" -}}
{{- $repository := . -}}
{{- (splitList "/" $repository) | last -}}
{{- end -}}

{{/*
Define a helper to convert a list of dictionaries into a list of keys.
- Iterates through a list of dictionaries and collects the 'name' field from each.
- Returns a list of these 'name' keys.
*/}}
{{- define "splunk-otel-collector.operator.get-dict-keys" -}}
  {{- $listOfDicts := . }}
  {{- $keyList := list }}
  {{- range $listOfDicts }}
    {{- $keyList = append $keyList .name }}
  {{- end }}
  {{- $keyList }}
{{- end }}
