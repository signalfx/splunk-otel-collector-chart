{{/*
Helper to ensure the correct usage of the Splunk OpenTelemetry Collector Operator.
- Checks for a valid endpoint for exporting telemetry data.
- Validates that the operator is configured correctly according to user input and default settings.
*/}}
{{- define "splunk-otel-collector.operator.validation-rules" -}}
  {{- /* Check if traces are enabled either through the platform or through observability */ -}}
  {{- $tracesEnabled := or
      (include "splunk-otel-collector.platformTracesEnabled" .)
      (include "splunk-otel-collector.o11yTracesEnabled" .)
  -}}

  {{- /* Check if the endpoint is overridden in the Helm values */ -}}
  {{- $endpointOverridden := and
      .Values.operator.instrumentation
      .Values.operator.instrumentation.spec
      .Values.operator.instrumentation.spec.exporter
      .Values.operator.instrumentation.spec.exporter.endpoint
      (ne .Values.operator.instrumentation.spec.exporter.endpoint "")
  -}}

  {{- /* Validate the configuration */ -}}
  {{- if and
      .Values.operator.enabled
      $tracesEnabled
      (not $endpointOverridden)
      (not (default "" .Values.environment))
  -}}
      {{- fail "When operator.enabled=true, (splunkPlatform.tracesEnabled=true or splunkObservability.tracesEnabled=true), (agent.enabled=true or gateway.enabled=true), then environment must be a non-empty string" -}}
  {{- end -}}
{{- end -}}

{{/*
Helper to define an endpoint for exporting telemetry data related to auto-instrumentation.
- Determines the endpoint based on user-defined values or default agent/gateway settings.
- Order of precedence: User-defined > Agent endpoint > Gateway endpoint
*/}}
{{- define "splunk-otel-collector.operator.instrumentation-exporter-endpoint" -}}
  {{- /* Initialize endpoint variable */ -}}
  {{- $endpoint := "" -}}

  {{- /* Use the user-defined endpoint if specified in the Helm values */ -}}
  {{- if and
    .Values.operator.instrumentation.spec
    .Values.operator.instrumentation.spec.exporter
    .Values.operator.instrumentation.spec.exporter.endpoint
    (ne .Values.operator.instrumentation.spec.exporter.endpoint "")
  }}
  {{- $endpoint = .Values.operator.instrumentation.spec.exporter.endpoint -}}
  {{- /* Use the agent endpoint if the agent is enabled */ -}}
  {{- else if .Values.agent.enabled -}}
    {{- $endpoint = "http://$(SPLUNK_OTEL_AGENT):4317" -}}
  {{- /* Use the gateway endpoint if the gateway is enabled */ -}}
  {{- else if .Values.gateway.enabled -}}
    {{- $endpoint = printf "http://%s:4317" (include "splunk-otel-collector.fullname" .) -}}
  {{- /* Fail if no valid endpoint is available */ -}}
  {{- else -}}
    {{- fail "When operator.enabled=true, (splunkPlatform.tracesEnabled=true or splunkObservability.tracesEnabled=true), either agent.enabled=true, gateway.enabled=true, or .Values.operator.instrumentation.spec.exporter.endpoint must be set" -}}
  {{- end -}}

  {{- /* Return the determined endpoint */ -}}
  {{- printf "%s" $endpoint -}}
{{- end -}}

{{/*
Helper to define entries for instrumentation libraries.
- Iterates over user-defined and default configuration settings for each library.
- Generates a YAML configuration block for each library, containing:
  - The library name.
  - The image repository and tag.
  - Environment variables, including special handling for 'OTEL_RESOURCE_ATTRIBUTES' and 'OTEL_EXPORTER_OTLP_ENDPOINT'.
*/}}
{{- define "splunk-otel-collector.operator.instrumentation-libraries" -}}
  {{- /* Store the endpoint in a variable to avoid context changes in nested loops.  */ -}}
  {{- /* Helm template loops change the context, making direct access to variables in parent scopes unreliable. */ -}}
  {{- $endpoint := include "splunk-otel-collector.operator.instrumentation-exporter-endpoint" $ -}}
  {{- /* Define a map (versions.txt name -> Instrumentation spec name) for instrumentation library names */ -}}
  {{- /* This is a simple workaround to accommodate one unique case that should be removed in the future */ -}}
  {{- $instLibAliases := dict "apache-httpd" "apacheHttpd" -}}

  {{- /* Iterate over each specified instrumentation library */ -}}
  {{- if .Values.operator.instrumentation.spec -}}
    {{- range $key, $value := .Values.operator.instrumentation.spec -}}
      {{- $instLibName := get $instLibAliases $key | default $key -}}

      {{- /* Check for required fields to determine if it is an instrumentation library */ -}}
      {{- if and $value.repository $value.tag -}}

        {{- /* Generate YAML keys for each instrumentation library */ -}}
        {{- printf "%s:" $instLibName | indent 2 -}}
        {{- printf "\n" -}}

        {{- /* Generate YAML for the image field */ -}}
        {{- printf "image: %s:%s" $value.repository $value.tag | indent 4 -}}
        {{- printf "\n" -}}

        {{- /* Output environment variables for the instrumentation library */ -}}
        {{- printf "env:" | indent 4 -}}
        {{- include "splunk-otel-collector.operator.extract-instrumentation-env" (dict "endpoint" $endpoint "instLibName" $instLibName "env" $value.env  "repository" $value.repository "tag" $value.tag) -}}

      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{/*
Helper to convert a list of dictionaries into a list of keys.
- Iterates through a list of dictionaries and collects the 'name' field from each.
- Returns a list of these 'name' keys.
*/}}
{{- define "splunk-otel-collector.operator.extract-name-keys-from-dict-list" -}}
  {{- /* Initialize variables */ -}}
  {{- $listOfDicts := . -}}
  {{- $keyList := list -}}

  {{- /* Collect 'name' field from each dictionary */ -}}
  {{- range $listOfDicts -}}
    {{- $keyList = append $keyList .name -}}
  {{- end -}}

  {{- /* Return the list of 'name' keys */ -}}
  {{- $keyList -}}
{{- end -}}

{{/*
Helper for generating environment variables for each instrumentation library.
- Prioritizes user-supplied environment variables over defaults.
- For OTEL_RESOURCE_ATTRIBUTES, combines default attributes with any user-supplied values.
- For OTEL_EXPORTER_OTLP_ENDPOINT, applies special case values based on the library ('dotnet', 'python'), but user-supplied values will override these.
*/}}
{{- define "splunk-otel-collector.operator.extract-instrumentation-env" }}
  {{- /* Initialize Splunk default Otel resource attribute; always included */ -}}
  {{- $imageShortName := printf "%s:%s" (splitList "/" .repository | last) .tag  -}}
  {{- $otelResourceAttributes := printf "splunk.zc.method=%s" $imageShortName }}

  {{- /* Loop through user-supplied environment variables */ -}}
  {{- range $env := .env }}
    {{- if eq $env.name "OTEL_RESOURCE_ATTRIBUTES" }}
      {{- $otelResourceAttributes = printf "%s,%s" $env.value $otelResourceAttributes }}
    {{- else }}
      {{- printf "- name: %s" $env.name | nindent 6 -}}
      {{- printf "  value: %s" $env.value | nindent 6 -}}
    {{- end }}
  {{- end }}

  {{- /* Output OTEL_RESOURCE_ATTRIBUTES with merged values */ -}}
  {{- printf "- name: %s" "OTEL_RESOURCE_ATTRIBUTES" | nindent 6 -}}
  {{- printf "  value: %s" $otelResourceAttributes | nindent 6 -}}
  {{- printf "\n" -}}

  {{- /* Handle custom or default exporter endpoint */ -}}
  {{- $customOtelExporterEndpoint := "" }}
  {{- if or (eq .instLibName "dotnet") (eq .instLibName "python") }}
    {{- $customOtelExporterEndpoint = .endpoint | replace ":4317" ":4318" }}
  {{- end }}
  {{- if .env }}
    {{- range $env := .env }}
      {{- if eq $env.name "OTEL_EXPORTER_OTLP_ENDPOINT" }}
        {{- $customOtelExporterEndpoint = $env.value }}
      {{- end }}
    {{- end }}
  {{- end }}

  {{- /* Output final OTEL_EXPORTER_OTLP_ENDPOINT, if applicable based on input conditions */ -}}
  {{- if $customOtelExporterEndpoint }}
    {{- if contains "4318" $customOtelExporterEndpoint }}
      {{- printf "# %s auto-instrumentation uses http/proto by default, so data must be sent to 4318 instead of 4317." .instLibName | indent 6 -}}
      {{- printf "\n" -}}
      {{- printf "# See: https://github.com/open-telemetry/opentelemetry-operator#opentelemetry-auto-instrumentation-injection" | indent 6 -}}
    {{- end }}
    {{- printf "- name: %s" "OTEL_EXPORTER_OTLP_ENDPOINT" | nindent 6 -}}
    {{- printf "  value: %s" $customOtelExporterEndpoint | nindent 6 -}}
    {{- printf "\n" -}}
  {{- end }}
{{- end }}
