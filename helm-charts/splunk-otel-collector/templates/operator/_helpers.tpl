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
      .Values.instrumentation
      .Values.instrumentation.spec
      .Values.instrumentation.spec.exporter
      .Values.instrumentation.spec.exporter.endpoint
      (ne .Values.instrumentation.spec.exporter.endpoint "")
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
Helper to build exporter spec
- Determines the endpoint based on user-defined values or default agent/gateway settings.
- Order of precedence: User-defined endpoint > Agent service endpoint > Agent host port endpoint > Gateway endpoint
- exporter property is a required field and user can't disable it in the custom values.yaml.
*/}}
{{- define "splunk-otel-collector.operator.instrumentation-exporter" -}}
  {{- $endpoint := include "splunk-otel-collector.operator.instrumentation-exporter-endpoint" $ -}}
  {{- $exporter := .Values.instrumentation.spec.exporter | default (dict) -}}
  {{- $mergedExporter := merge $exporter (dict "endpoint" $endpoint) -}}
exporter:
  {{- $mergedExporter | toYaml | nindent 2 -}}
{{- end -}}

{{/*
Helper to build propagators spec
- It uses the with directive, which allows the user to disable it in the custom values.yaml.
*/}}
{{- define "splunk-otel-collector.operator.instrumentation-propagators" -}}
  {{- with .Values.instrumentation.spec.propagators }}
propagators:
{{- toYaml . | nindent 2 }}
  {{- end -}}
{{- end -}}

{{/* Helper to build sampler spec
- It uses the with directive, which allows the user to disable it in the custom values.yaml.
*/}}
{{- define "splunk-otel-collector.operator.instrumentation-sampler" -}}
  {{- with .Values.instrumentation.spec.sampler }}
sampler:
{{- toYaml . | nindent 2 }}
  {{- end }}
{{- end -}}

{{/* Helper to build environment variables
- If profiling is enabled, it adds SPLUNK_PROFILER_ENABLED and SPLUNK_PROFILER_MEMORY_ENABLED which are necessary environment variables for profiling.
- If the exporter endpoint contains "SPLUNK_OTEL_AGENT", it sets the SPLUNK_OTEL_AGENT environment variable to the host IP of the collector.
*/}}
{{- define "splunk-otel-collector.operator.instrumentation-env" -}}
  {{- $env := .Values.instrumentation.spec.env | default list -}}
  {{- if .Values.splunkObservability.profilingEnabled -}}
    {{- if eq (include "splunk-otel-collector.operator.env-has" (dict "env" .Values.instrumentation.env "envName" "SPLUNK_PROFILER_ENABLED")) "false" }}
      {{- $env = append $env (dict "name" "SPLUNK_PROFILER_ENABLED" "value" "true") -}}
    {{- end }}
    {{- if eq (include "splunk-otel-collector.operator.env-has" (dict "env" .Values.instrumentation.env "envName" "SPLUNK_PROFILER_MEMORY_ENABLED")) "false" }}
      {{- $env = append $env (dict "name" "SPLUNK_PROFILER_MEMORY_ENABLED" "value" "true") -}}
    {{- end }}
  {{- end -}}
  {{- if contains "SPLUNK_OTEL_AGENT" (include "splunk-otel-collector.operator.instrumentation-exporter-endpoint" .) }}
    {{- $env = append $env (dict "name" "SPLUNK_OTEL_AGENT" "valueFrom" (dict "fieldRef" (dict "apiVersion" "v1" "fieldPath" "status.hostIP"))) -}}
  {{- end -}}
  {{- with $env }}
env:
{{- toYaml . | nindent 2 }}
  {{- end }}
{{- end -}}

{{/*
Helper to build entries for instrumentation libraries
- Iterates over the list of supported libraries.
- Generates a YAML configuration block for each library, containing:
  - The image name and tag.
  - Environment variables, including special handling for 'OTEL_RESOURCE_ATTRIBUTES' and 'OTEL_EXPORTER_OTLP_ENDPOINT'.
  - Any additional properties defined in the custom Helm values.
*/}}
{{- define "splunk-otel-collector.operator.instrumentation-libraries" -}}
  {{- /* Store the endpoint in a variable to avoid context changes in nested loops.  */ -}}
  {{- /* Helm template loops change the context, making direct access to variables in parent scopes unreliable. */ -}}
  {{- $endpoint := include "splunk-otel-collector.operator.instrumentation-exporter-endpoint" $ -}}

  {{- $libraries := list "apacheHttpd" "dotnet" "go" "java" "nginx" "nodejs" "python" -}}
  {{- range $lib := $libraries }}
    {{- if and (eq (kindOf $.Values.instrumentation.spec) "map") (hasKey $.Values.instrumentation.spec $lib) -}}
      {{- $libSpec := get $.Values.instrumentation.spec $lib }}

      {{- /* Instead of including as a string, use a template function that returns proper data structure */ -}}
      {{- $envData := dict "endpoint" $endpoint "instLibName" $lib "env" $libSpec.env "image" $libSpec.image -}}
      {{- $envVars := include "splunk-otel-collector.operator.extract-instrumentation-env" $envData | fromYaml -}}

      {{- /* Now merge the structured data instead of a string */ -}}
      {{- $mergedLibSpec := merge (dict "env" $envVars.env) $libSpec -}}

      {{- printf "\n%s:" $lib }}
      {{- toYaml $mergedLibSpec | nindent 2 -}}
    {{- end -}}
  {{- end }}
{{- end -}}

{{/* Helper to build instrumentation spec */}}
{{- define "splunk-otel-collector.operator.instrumentation-spec" -}}
  {{- printf "%s%s%s%s%s"
      (include "splunk-otel-collector.operator.instrumentation-exporter" .)
      (include "splunk-otel-collector.operator.instrumentation-propagators" .)
      (include "splunk-otel-collector.operator.instrumentation-sampler" .)
      (include "splunk-otel-collector.operator.instrumentation-env" .)
      (include "splunk-otel-collector.operator.instrumentation-libraries" .)
  | indent 2 -}}
{{- end -}}

{{/* Helper to determine exporter endpoint
- Determines the endpoint based on user-defined values or default agent/gateway settings.
- Order of precedence: User-defined endpoint > Agent service endpoint > Agent host port endpoint > Gateway endpoint
*/}}
{{- define "splunk-otel-collector.operator.instrumentation-exporter-endpoint" -}}
  {{- /* Initialize endpoint variable */ -}}
  {{- $endpoint := "" -}}

  {{- /* Use the user-defined endpoint if specified in the Helm values */ -}}
  {{- if and
    .Values.instrumentation
    .Values.instrumentation.spec
    .Values.instrumentation.spec.exporter
    .Values.instrumentation.spec.exporter.endpoint
    (ne .Values.instrumentation.spec.exporter.endpoint "")
  }}
    {{- $endpoint = .Values.instrumentation.spec.exporter.endpoint -}}
  {{- /* Use the agent service endpoint if the agent is enabled */ -}}
  {{- else if .Values.agent.service.enabled -}}
    {{- $endpoint = printf "http://%s-agent.%s.svc.cluster.local:4317" (include "splunk-otel-collector.fullname" .) .Release.Namespace -}}
  {{- /* Use the agent host port endpoint if the agent is enabled */ -}}
  {{- else if .Values.agent.enabled -}}
    {{- $endpoint = "http://$(SPLUNK_OTEL_AGENT):4317" -}}
  {{- /* Use the gateway endpoint if the gateway is enabled */ -}}
  {{- else if .Values.gateway.enabled -}}
    {{- $endpoint = printf "http://%s:4317" (include "splunk-otel-collector.fullname" .) -}}
  {{- /* Fail if no valid endpoint is available */ -}}
  {{- else -}}
    {{- fail "When operator.enabled=true, (splunkPlatform.tracesEnabled=true or splunkObservability.tracesEnabled=true), either agent.enabled=true, gateway.enabled=true, or .Values.instrumentation.exporter.endpoint must be set" -}}
  {{- end -}}

  {{- /* Return the determined endpoint */ -}}
  {{- printf "%s" $endpoint -}}
{{- end -}}

{{/*
Helper to check if env (list of dictionaries) has an environment variable (dictionary) with a specific name.
- Takes a list of dictionaries (env) and a string (envName).
- Returns "true" if a dictionary in the list has a 'name' key that matches envName, otherwise "false".
*/}}
{{- define "splunk-otel-collector.operator.env-has" -}}
  {{- /* Extract parameters */ -}}
  {{- $env := default list .env -}}
  {{- $envName := .envName -}}
  {{- $found := false -}}

  {{- /* Check if envName exists in the list of dictionaries */ -}}
  {{- range $env -}}
    {{- if eq .name $envName -}}
      {{- $found = true -}}
    {{- end -}}
  {{- end -}}

  {{- $found -}}
{{- end -}}

{{/*
Helper for generating environment variables for each instrumentation library.
- Prioritizes user-supplied environment variables over defaults.
- For OTEL_RESOURCE_ATTRIBUTES, combines default attributes with any user-supplied values.
- For OTEL_EXPORTER_OTLP_ENDPOINT, applies special case values based on the library ('dotnet', 'python', `java`), but user-supplied values will override these.
*/}}
{{- define "splunk-otel-collector.operator.extract-instrumentation-env" }}
  {{- println "env:" -}}
  {{- /* Initialize Splunk default Otel resource attribute; always included */ -}}
  {{- $imageShortName := printf "%s" (splitList "/" .image | last) -}}
  {{- $otelResourceAttributes := printf "splunk.zc.method=%s" $imageShortName }}

  {{- $customOtelExporterEndpoint := "" }}
  {{- if or (eq .instLibName "dotnet") (eq .instLibName "java") (eq .instLibName "nodejs") (eq .instLibName "python") }}
    {{- $customOtelExporterEndpoint = .endpoint | replace ":4317" ":4318" }}
  {{- end }}

  {{- /* Loop through user-supplied environment variables */ -}}
  {{- range $env := .env }}
    {{- if eq $env.name "OTEL_RESOURCE_ATTRIBUTES" }}
      {{- $otelResourceAttributes = printf "%s,%s" $env.value $otelResourceAttributes }}
    {{- else if eq $env.name "OTEL_EXPORTER_OTLP_ENDPOINT" }}
      {{- $customOtelExporterEndpoint = $env.value }}
    {{- else -}}
      {{- printf "- name: %s" $env.name -}}
      {{- printf "\n" -}}
      {{- printf "  value: %s" ($env.value | quote) -}}
      {{- printf "\n" -}}
    {{- end }}
  {{- end }}

  {{- /* Output OTEL_RESOURCE_ATTRIBUTES with merged values */ -}}
  {{- printf "- name: %s" "OTEL_RESOURCE_ATTRIBUTES" -}}
  {{- printf "\n" -}}
  {{- printf "  value: %s" $otelResourceAttributes -}}
  {{- printf "\n" -}}

  {{- /* Output final OTEL_EXPORTER_OTLP_ENDPOINT, if applicable based on input conditions */ -}}
  {{- if $customOtelExporterEndpoint }}
    {{- /* Ensure the SPLUNK_OTEL_AGENT env var is set with per language env vars to successfully use it in env var substitution */ -}}
    {{- if contains "SPLUNK_OTEL_AGENT" $customOtelExporterEndpoint -}}
      {{- printf "- name: SPLUNK_OTEL_AGENT\n  valueFrom:\n    fieldRef:\n      apiVersion: v1\n      fieldPath: status.hostIP" -}}
      {{- printf "\n" -}}
    {{- end -}}
    {{- printf "- name: %s" "OTEL_EXPORTER_OTLP_ENDPOINT" -}}
    {{- printf "\n" -}}
    {{- printf "  value: %s" $customOtelExporterEndpoint -}}
    {{- printf "\n" -}}
  {{- end }}
{{- end }}
