{{/*
Derived values and validation for experimental Splunk Platform log routing.
The generated Collector components live in _platform-logs-routing.tpl.
*/}}

{{/* Whether any named routes are configured. */}}
{{- define "splunk-otel-collector.platformLogsRoutingEnabled" -}}
{{- $routing := default (dict) .Values.splunkPlatform.logsRouting -}}
{{- if gt (len (default (dict) $routing.routes)) 0 }}true{{- else }}false{{- end }}
{{- end -}}

{{/* Environment variable used for a route token backed by a Secret. */}}
{{- define "splunk-otel-collector.platformLogsRouteTokenEnvVar" -}}
{{- printf "SPLUNK_PLATFORM_HEC_TOKEN_%s" (upper (replace "-" "_" .)) -}}
{{- end -}}

{{/* Generated processor that enforces explicit route metadata. */}}
{{- define "splunk-otel-collector.platformLogsRouteMetadataProcessorName" -}}
{{- printf "transform/platform_logs_route_metadata_%s" (replace "-" "_" .) -}}
{{- end -}}

{{/* Whether a route explicitly pins any Splunk HEC metadata fields. */}}
{{- define "splunk-otel-collector.platformLogsRouteHasMetadata" -}}
{{- if or (hasKey . "index") (hasKey . "source") (hasKey . "sourcetype") }}true{{- else }}false{{- end }}
{{- end -}}

{{/* Collector token reference for a route. */}}
{{- define "splunk-otel-collector.platformLogsRouteTokenRef" -}}
{{- if .route.tokenFile -}}
{{- printf "${file:%s}" .route.tokenFile -}}
{{- else -}}
{{- printf "${%s}" (include "splunk-otel-collector.platformLogsRouteTokenEnvVar" .name) -}}
{{- end -}}
{{- end -}}

{{/* Collector file-storage component for a route persistent queue. */}}
{{- define "splunk-otel-collector.platformLogsRouteStorageName" -}}
{{- printf "file_storage/persistent_queue_%s" (replace "-" "_" .) -}}
{{- end -}}

{{/*
Whether a route uses direct-agent persistent queue storage. A route-level
non-null value overrides the global setting.
*/}}
{{- define "splunk-otel-collector.platformLogsRoutePersistentQueueEnabled" -}}
{{- $enabled := .context.Values.splunkPlatform.sendingQueue.persistentQueue.enabled -}}
{{- $queue := default (dict) .route.sendingQueue -}}
{{- $persistent := default (dict) $queue.persistentQueue -}}
{{- if and (hasKey $persistent "enabled") (ne (toString $persistent.enabled) "<nil>") -}}
  {{- $enabled = $persistent.enabled -}}
{{- end -}}
{{- if $enabled }}true{{- else }}false{{- end }}
{{- end -}}

{{/* Whether the direct-agent workload needs the persistent-queue hostPath. */}}
{{- define "splunk-otel-collector.anyDirectPersistentQueueEnabled" -}}
{{- $enabled := .Values.splunkPlatform.sendingQueue.persistentQueue.enabled -}}
{{- if eq (include "splunk-otel-collector.platformLogsRoutingEnabled" .) "true" -}}
  {{- range $name, $route := .Values.splunkPlatform.logsRouting.routes -}}
    {{- if eq (include "splunk-otel-collector.platformLogsRoutePersistentQueueEnabled" (dict "context" $ "route" $route)) "true" -}}
      {{- $enabled = true -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- if $enabled }}true{{- else }}false{{- end }}
{{- end -}}

{{/* Whether a referenced processor has a non-null definition. */}}
{{- define "splunk-otel-collector.platformLogsRoutingProcessorDefined" -}}
{{- if and (hasKey .processors .name) (ne (toString (get .processors .name)) "<nil>") }}true{{- else }}false{{- end }}
{{- end -}}

{{/* Validate the Splunk Platform log-routing contract. */}}
{{- define "splunk-otel-collector.validatePlatformLogsRouting" -}}
{{- $routing := default (dict) .Values.splunkPlatform.logsRouting -}}
{{- $routes := default (dict) $routing.routes -}}
{{- if gt (len $routes) 0 -}}
  {{/* Feature prerequisites. */}}
  {{- if eq (trim (toString (default "" $routing.attribute))) "" -}}
    {{- fail "splunkPlatform.logsRouting.attribute is required when routes are configured" -}}
  {{- end -}}
  {{- if ne (include "splunk-otel-collector.platformLogsEnabled" .) "true" -}}
    {{- fail "splunkPlatform.logsRouting.routes requires Splunk Platform logs over HEC" -}}
  {{- end -}}
  {{- if eq (include "splunk-otel-collector.platformLogsViaOtlpEnabled" .) "true" -}}
    {{- fail "splunkPlatform.logsRouting.routes cannot be combined with splunkPlatform.otlpIngest.enabled" -}}
  {{- end -}}
  {{- if eq (toString (default "" .Values.splunkPlatform.endpoint)) "" -}}
    {{- fail "splunkPlatform.logsRouting.routes requires splunkPlatform.endpoint" -}}
  {{- end -}}
  {{- if and (not .Values.gateway.enabled) (or (not .Values.agent.enabled) (eq .Values.distribution "eks/fargate")) -}}
    {{- fail "splunkPlatform.logsRouting.routes requires gateway.enabled=true or an enabled non-Fargate agent" -}}
  {{- end -}}

  {{/* Route identity, credentials, and queue constraints. */}}
  {{- $seenValues := dict -}}
  {{- $reserved := list "default" -}}
  {{- range $name, $route := $routes -}}
    {{- if not (regexMatch "^[a-z][a-z0-9-]{0,62}$" $name) -}}
      {{- fail (printf "splunkPlatform.logsRouting.routes route name %q must match ^[a-z][a-z0-9-]{0,62}$" $name) -}}
    {{- end -}}
    {{- if has $name $reserved -}}
      {{- fail (printf "splunkPlatform.logsRouting.routes route name %q is reserved" $name) -}}
    {{- end -}}
    {{- $value := default $name $route.value -}}
    {{- if hasKey $seenValues $value -}}
      {{- fail (printf "splunkPlatform.logsRouting.routes has duplicate effective match value %q" $value) -}}
    {{- end -}}
    {{- $_ := set $seenValues $value true -}}
    {{- $hasSecret := and (hasKey $route "tokenSecret") $route.tokenSecret (ne (toString (default "" $route.tokenSecret.name)) "") (ne (toString (default "" $route.tokenSecret.key)) "") -}}
    {{- $hasFile := and (hasKey $route "tokenFile") (ne (toString (default "" $route.tokenFile)) "") -}}
    {{- if eq $hasSecret $hasFile -}}
      {{- fail (printf "splunkPlatform.logsRouting.routes.%s must set exactly one of tokenSecret or tokenFile" $name) -}}
    {{- end -}}
    {{- if and $hasFile (not (hasPrefix "/" $route.tokenFile)) -}}
      {{- fail (printf "splunkPlatform.logsRouting.routes.%s.tokenFile must be an absolute path" $name) -}}
    {{- end -}}
    {{- $queue := default (dict) $route.sendingQueue -}}
    {{- $persistent := default (dict) $queue.persistentQueue -}}
    {{- $persistentExplicit := and (hasKey $persistent "enabled") (ne (toString $persistent.enabled) "<nil>") -}}
    {{- $persistentEnabled := eq (include "splunk-otel-collector.platformLogsRoutePersistentQueueEnabled" (dict "context" $ "route" $route)) "true" -}}
    {{- $queueEnabled := $.Values.splunkPlatform.sendingQueue.enabled -}}
    {{- if and (hasKey $queue "enabled") (ne (toString $queue.enabled) "<nil>") -}}
      {{- $queueEnabled = $queue.enabled -}}
    {{- end -}}
    {{- if and (not $.Values.gateway.enabled) $persistentEnabled (not $queueEnabled) -}}
      {{- fail (printf "splunkPlatform.logsRouting.routes.%s cannot enable persistence when its sending queue is disabled" $name) -}}
    {{- end -}}
    {{- if and $.Values.gateway.enabled $persistentExplicit $persistent.enabled -}}
      {{- fail (printf "splunkPlatform.logsRouting.routes.%s cannot enable persistence in gateway mode" $name) -}}
    {{- end -}}
  {{- end -}}

  {{/* Processor references must belong to the workload that owns routing. */}}
  {{- $ownerConfig := ternary (default (dict) .Values.gateway.config) (default (dict) .Values.agent.config) .Values.gateway.enabled -}}
  {{- $processors := default (dict) $ownerConfig.processors -}}
  {{- range $processor := default (list) $routing.preRoutingProcessors -}}
    {{- if ne (include "splunk-otel-collector.platformLogsRoutingProcessorDefined" (dict "processors" $processors "name" $processor)) "true" -}}
      {{- fail (printf "splunkPlatform.logsRouting.preRoutingProcessors references %q, which is not defined in %s.config.processors" $processor (ternary "gateway" "agent" $.Values.gateway.enabled)) -}}
    {{- end -}}
  {{- end -}}
  {{- range $processor := default (list) $routing.defaultProcessors -}}
    {{- if ne (include "splunk-otel-collector.platformLogsRoutingProcessorDefined" (dict "processors" $processors "name" $processor)) "true" -}}
      {{- fail (printf "splunkPlatform.logsRouting.defaultProcessors references %q, which is not defined in %s.config.processors" $processor (ternary "gateway" "agent" $.Values.gateway.enabled)) -}}
    {{- end -}}
  {{- end -}}
  {{- range $name, $route := $routes -}}
    {{- $metadataProcessor := include "splunk-otel-collector.platformLogsRouteMetadataProcessorName" $name -}}
    {{- if and (eq (include "splunk-otel-collector.platformLogsRouteHasMetadata" $route) "true") (hasKey $processors $metadataProcessor) -}}
      {{- fail (printf "splunkPlatform.logsRouting.routes.%s reserves processor %q; remove it from %s.config.processors" $name $metadataProcessor (ternary "gateway" "agent" $.Values.gateway.enabled)) -}}
    {{- end -}}
    {{- range $processor := default (list) $route.processors -}}
      {{- if ne (include "splunk-otel-collector.platformLogsRoutingProcessorDefined" (dict "processors" $processors "name" $processor)) "true" -}}
        {{- fail (printf "splunkPlatform.logsRouting.routes.%s.processors references %q, which is not defined in %s.config.processors" $name $processor (ternary "gateway" "agent" $.Values.gateway.enabled)) -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- end -}}
