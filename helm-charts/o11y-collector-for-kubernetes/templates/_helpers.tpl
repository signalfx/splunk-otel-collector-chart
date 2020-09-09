{{/* vim: set filetype=mustache: */}}

{{/*
Expand the name of the chart.
*/}}
{{- define "o11y-collector.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "o11y-collector.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "o11y-collector.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "o11y-collector.secret" -}}
{{- if .Values.secret.name -}}
{{- printf "%s" .Values.secret.name -}}
{{- else -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "o11y-collector.serviceAccountName" -}}
    {{ default (include "o11y-collector.fullname" .) .Values.serviceAccount.name }}
{{- end -}}

{{/*
Get Signalfx ingest host
*/}}
{{- define "o11y-collector.ingestHost" -}}
{{- $_ := required "signalfx.realm or signalfx.ingestHost must be provided" (or .Values.signalfx.ingestHost .Values.signalfx.realm) }}
{{- .Values.signalfx.ingestHost | default (printf "ingest.%s.signalfx.com" .Values.signalfx.realm) }}
{{- end -}}

{{/*
Get Signalfx ingest URL
*/}}
{{- define "o11y-collector.ingestUrl" -}}
{{- $host := include "o11y-collector.ingestHost" . }}
{{- $endpoint := printf "%s://%s" .Values.signalfx.protocol $host }}
{{- if or (and (eq .Values.signalfx.protocol "http") (ne (toString .Values.signalfx.port) "80")) (and (eq .Values.signalfx.protocol "https") (ne (toString .Values.signalfx.port) "443")) }}
{{- printf "%s:%s" $endpoint (toString .Values.signalfx.port) }}
{{- else }}
{{- $endpoint }}
{{- end }}
{{- end -}}

{{/*
Get Signalfx API host.
*/}}
{{- define "o11y-collector.apiHost" -}}
{{- $_ := required "signalfx.realm or signalfx.apiHost must be provided" (or .Values.signalfx.apiHost .Values.signalfx.realm) }}
{{- .Values.signalfx.apiHost | default (printf "api.%s.signalfx.com" .Values.signalfx.realm) }}
{{- end -}}

{{/*
Get Signalfx API URL.
*/}}
{{- define "o11y-collector.apiUrl" -}}
{{- $host := include "o11y-collector.apiHost" . -}}
{{- $endpoint := printf "%s://%s" .Values.signalfx.protocol $host }}
{{- if or (and (eq .Values.signalfx.protocol "http") (ne (toString .Values.signalfx.port) "80")) (and (eq .Values.signalfx.protocol "https") (ne (toString .Values.signalfx.port) "443")) }}
{{- printf "%s:%s" $endpoint (toString .Values.signalfx.port) }}
{{- else }}
{{- $endpoint }}
{{- end }}
{{- end -}}

{{/*
Get signalfx.accessToken.
*/}}
{{- define "o11y-collector.accessToken" -}}
{{- required "signalfx.accessToken value must be provided" .Values.signalfx.accessToken -}}
{{- end -}}

{{/*
Create the fluentd image name.
*/}}
{{- define "o11y-collector.image.fluentd" -}}
{{- printf "%s/%s:%s" .Values.image.fluentd.registry .Values.image.fluentd.name .Values.image.fluentd.tag -}}
{{- end -}}

{{/*
Create the opentelemetry collector image name.
*/}}
{{- define "o11y-collector.image.otelcol" -}}
{{- printf "%s/%s:%s" .Values.image.otelcol.registry .Values.image.otelcol.name .Values.image.otelcol.tag -}}
{{- end -}}

{{/*
Convert memory value from resources.limit to numeric value in MiB to be used by otel memory_limiter processor.
*/}}
{{- define "o11y-collector.convertMemToMib" -}}
{{- $mem := lower . -}}
{{- if hasSuffix "e" $mem -}}
{{- trimSuffix "e" $mem | atoi | mul 1000 | mul 1000 | mul 1000 | mul 1000 -}}
{{- else if hasSuffix "ei" $mem -}}
{{- trimSuffix "ei" $mem | atoi | mul 1024 | mul 1024 | mul 1024 | mul 1024 -}}
{{- else if hasSuffix "p" $mem -}}
{{- trimSuffix "p" $mem | atoi | mul 1000 | mul 1000 | mul 1000 -}}
{{- else if hasSuffix "pi" $mem -}}
{{- trimSuffix "pi" $mem | atoi | mul 1024 | mul 1024 | mul 1024 -}}
{{- else if hasSuffix "t" $mem -}}
{{- trimSuffix "t" $mem | atoi | mul 1000 | mul 1000 -}}
{{- else if hasSuffix "ti" $mem -}}
{{- trimSuffix "ti" $mem | atoi | mul 1024 | mul 1024 -}}
{{- else if hasSuffix "g" $mem -}}
{{- trimSuffix "g" $mem | atoi | mul 1000 -}}
{{- else if hasSuffix "gi" $mem -}}
{{- trimSuffix "gi" $mem | atoi | mul 1024 -}}
{{- else if hasSuffix "m" $mem -}}
{{- div (trimSuffix "m" $mem | atoi | mul 1000) 1024 -}}
{{- else if hasSuffix "mi" $mem -}}
{{- trimSuffix "mi" $mem | atoi -}}
{{- else if hasSuffix "k" $mem -}}
{{- div (trimSuffix "k" $mem | atoi) 1000 -}}
{{- else if hasSuffix "ki" $mem -}}
{{- div (trimSuffix "ki" $mem | atoi) 1024 -}}
{{- else -}}
{{- div (div ($mem | atoi) 1024) 1024 -}}
{{- end -}}
{{- end -}}

{{/*
Get otel memory_limiter limit_mib value based on 80% of resources.limit.
*/}}
{{- define "o11y-collector.getOtelMemLimitMib" -}}
{{- $resourseLimit := include "o11y-collector.convertMemToMib" .resources.limits.memory }}
{{- .config.processors.memory_limiter.limit_mib | default (div (mul $resourseLimit 80) 100) }}
{{- end -}}

{{/*
Get otel memory_limiter spike_limit_mib value based on 25% of resources.limit.
*/}}
{{- define "o11y-collector.getOtelMemSpikeLimitMib" -}}
{{- $resourseLimit := include "o11y-collector.convertMemToMib" .resources.limits.memory }}
{{- .config.processors.memory_limiter.spike_limit_mib | default (div (mul $resourseLimit 25) 100) }}
{{- end -}}

{{/*
Get otel memory_limiter ballast_size_mib value based on 40% of resources.limit.
*/}}
{{- define "o11y-collector.getOtelMemBallastSizeMib" }}
{{- $resourseLimit := include "o11y-collector.convertMemToMib" .resources.limits.memory }}
{{- .config.processors.memory_limiter.ballast_size_mib | default (div (mul $resourseLimit 40) 100) }}
{{- end -}}

{{/*
Create the opentelemetry collector agent configmap with applied default values.
*/}}
{{- define "o11y-collector.otelAgent.config" -}}
{{- $config := .Values.otelAgent.config | deepCopy -}}
{{- $processors := index $config "processors" }}
{{- $exporters := index $config "exporters" }}
{{- $service := index $config "service" }}
{{- $pipelines := index $service "pipelines" }}
{{- $tracesPipeline := index $pipelines "traces" }}
{{- $metricsPipeline := index $pipelines "metrics" }}

{{- /*
Setup default traces and metrics exporters.
If standalone collector deployment is enabled, it's used to send OTLP traces and metrics to.
Otherwise traces and metrics are sent directly to Signalfx backend.
*/}}
{{- if hasKey $exporters "otlp" }}
  {{- $otlpExporter := index $exporters "otlp" }}
  {{- if not (index $otlpExporter "endpoint") }}
    {{- if .Values.otelCollector.enabled }}
      {{- $_ := set $otlpExporter "endpoint" (printf "%s:55680" (include "o11y-collector.fullname" .)) }}
    {{- else }}
      {{- $_ := unset $exporters "otlp" }}
    {{- end }}
  {{- end }}
{{- end }}
{{- if hasKey $exporters "sapm" }}
  {{- $sapmExporter := index $exporters "sapm" }}
  {{- if not (index $sapmExporter "endpoint") | or (not (index $sapmExporter "access_token")) }}
    {{- if .Values.otelCollector.enabled }}
      {{- $_ := unset $exporters "sapm" }}
    {{- else }}
      {{- $_ := set $sapmExporter "endpoint" (printf "%s/v2/trace" (include "o11y-collector.ingestUrl" .)) }}
      {{- $_ := set $sapmExporter "access_token" (include "o11y-collector.accessToken" .) }}
    {{- end }}
  {{- end }}
{{- end }}
{{- if hasKey $exporters "signalfx" }}
  {{- $signalfxExporter := index $exporters "signalfx" }}
  {{- if not (index $signalfxExporter "ingest_url") | or (not (index $signalfxExporter "api_url")) | or (not (index $signalfxExporter "access_token")) }}
    {{- if .Values.otelCollector.enabled }}
      {{- $_ := unset $exporters "signalfx" }}
    {{- else }}
      {{- $_ := set $signalfxExporter "ingest_url" (printf "%s/v2/datapoint" (include "o11y-collector.ingestUrl" .)) }}
      {{- $_ := set $signalfxExporter "api_url" (include "o11y-collector.apiUrl" .) }}
      {{- $_ := set $signalfxExporter "access_token" (include "o11y-collector.accessToken" .) }}
    {{- end }}
  {{- end }}
{{- end }}
{{- if $tracesPipeline | and (not (index $tracesPipeline "exporters")) }}
  {{- if .Values.otelCollector.enabled }}
    {{- $_ := set $tracesPipeline "exporters" (list "otlp") }}
  {{- else }}
    {{- $_ := set $tracesPipeline "exporters" (list "sapm") }}
  {{- end }}
{{- end }}
{{- if $metricsPipeline | and (not (index $metricsPipeline "exporters")) }}
  {{- if .Values.otelCollector.enabled }}
    {{- $_ := set $metricsPipeline "exporters" (list "otlp") }}
  {{- else }}
    {{- $_ := set $metricsPipeline "exporters" (list "signalfx") }}
  {{- end }}
{{- end }}

{{- /* Setup default memory_limiter processor configuration based of otel container limits */}}
{{- if hasKey $processors "memory_limiter" }}
{{- $memoryLimiter := index $processors "memory_limiter" }}
{{- $_ := set $memoryLimiter "limit_mib" (include "o11y-collector.getOtelMemLimitMib" .Values.otelAgent) }}
{{- $_ := set $memoryLimiter "spike_limit_mib" (include "o11y-collector.getOtelMemSpikeLimitMib" .Values.otelAgent) }}
{{- $_ := set $memoryLimiter "ballast_size_mib" (include "o11y-collector.getOtelMemBallastSizeMib" .Values.otelAgent) }}
{{- end }}

{{- /* Setup "resource/add_cluster_name" processor that set .Values.clusterName attribute value to all traces and metrics. */}}
{{ if hasKey $processors "resource/add_cluster_name" }}
{{- $resourceProcessor := index $processors "resource/add_cluster_name" }}
{{- $resourceAttributes := index $resourceProcessor "attributes" }}
{{- $insertClusterNameAction := (dict "action" "upsert" "key" "k8s.cluster.name" "value" .Values.clusterName) -}}
{{- $_ := set $resourceProcessor "attributes" (append $resourceAttributes $insertClusterNameAction) -}}
{{- end }}

{{- /* Set "passthrough" mode in k8s_tagger processor if collector enabled */}}
{{ if and (hasKey $processors "k8s_tagger") .Values.otelCollector.enabled }}
{{- $k8sProcessor := index $processors "k8s_tagger" }}
{{- $_ := set $k8sProcessor "passthrough" true -}}
{{- end }}

{{- /* Set "processors.resourcedetection.detectors" based on "platform" */}}
{{ if index $processors "resourcedetection" }}
  {{- $resourcedetectionProcessor := index $processors "resourcedetection" }}
  {{- if not (index $resourcedetectionProcessor "detectors") }}
    {{- if eq .Values.platform "gcp" }}
      {{- $_ := set $resourcedetectionProcessor "detectors" (list "env" "gce" ) }}
    {{- else if eq .Values.platform "aws" }}
      {{- $_ := set $resourcedetectionProcessor "detectors" (list "env" "ec2") }}
    {{- else }}
      {{- $_ := set $resourcedetectionProcessor "detectors" (list "env") }}
    {{- end }}
  {{- end }}
{{- end }}

{{- $config | toYaml | nindent 4 }}

{{- end -}}

{{/*
Create the opentelemetry collector configmap with applied default values.
*/}}
{{- define "o11y-collector.otelCollector.config" -}}
{{- $config := .Values.otelCollector.config | deepCopy -}}
{{- $processors := index $config "processors" }}
{{- $exporters := index $config "exporters" }}

{{- /* Set sapm exporter values based on .Values.signalfx configurations */}}
{{- if hasKey $exporters "sapm" }}
{{- $sapmExporter := index $exporters "sapm" }}
{{- $_ := set $sapmExporter "endpoint" (printf "%s/v2/trace" (include "o11y-collector.ingestUrl" .)) }}
{{- $_ := set $sapmExporter "access_token" (include "o11y-collector.accessToken" .) }}
{{- end }}

{{- /* Set signalfx exporter values based on .Values.signalfx configurations */}}
{{- if hasKey $exporters "signalfx" }}
{{- $signalfxExporter := index $exporters "signalfx" }}
{{- $_ := set $signalfxExporter "ingest_url" (printf "%s/v2/datapoint" (include "o11y-collector.ingestUrl" .)) }}
{{- $_ := set $signalfxExporter "api_url" (include "o11y-collector.apiUrl" .) }}
{{- $_ := set $signalfxExporter "access_token" (include "o11y-collector.accessToken" .) }}
{{- end }}

{{- /* Setup default memory_limiter processor configuration */}}
{{- if hasKey $processors "memory_limiter" }}
{{- $memoryLimiter := index $processors "memory_limiter" }}
{{- $_ := set $memoryLimiter "limit_mib" (include "o11y-collector.getOtelMemLimitMib" .Values.otelCollector) }}
{{- $_ := set $memoryLimiter "spike_limit_mib" (include "o11y-collector.getOtelMemSpikeLimitMib" .Values.otelCollector) }}
{{- $_ := set $memoryLimiter "ballast_size_mib" (include "o11y-collector.getOtelMemBallastSizeMib" .Values.otelCollector) }}
{{- end }}

{{- /* Add an attributes action setting clusterName value to resource/add_cluster_name processor */}}
{{ if hasKey $processors "resource/add_cluster_name" }}
{{- $resourceProcessor := index $processors "resource/add_cluster_name" }}
{{- $resourceAttributes := index $resourceProcessor "attributes" }}
{{- $insertClusterNameAction := (dict "action" "upsert" "key" "k8s.cluster.name" "value" .Values.clusterName) -}}
{{- $_ := set $resourceProcessor "attributes" (append $resourceAttributes $insertClusterNameAction) -}}
{{- end }}

{{- $config | toYaml | nindent 4 }}

{{- end -}}

{{/*
Create the k8s сluster receiver configmap with applied default values.
*/}}
{{- define "o11y-collector.otelK8sClusterReceiver.config" -}}
{{- $config := .Values.otelK8sClusterReceiver.config | deepCopy -}}
{{- $processors := index $config "processors" }}
{{- $exporters := index $config "exporters" }}

{{- /* Set signalfx exporter values based on .Values.signalfx configurations */}}
{{- if hasKey $exporters "signalfx" }}
{{- $signalfxExporter := index $exporters "signalfx" }}
{{- $_ := set $signalfxExporter "ingest_url" (printf "%s/v2/datapoint" (include "o11y-collector.ingestUrl" .)) }}
{{- $_ := set $signalfxExporter "api_url" (include "o11y-collector.apiUrl" .) }}
{{- $_ := set $signalfxExporter "access_token" (include "o11y-collector.accessToken" .) }}
{{- end }}

{{- /* Add an attributes action setting clusterName value to resource/add_cluster_name processor */}}
{{ if hasKey $processors "resource/add_cluster_name" }}
{{- $resourceProcessor := index $processors "resource/add_cluster_name" }}
{{- $resourceAttributes := index $resourceProcessor "attributes" }}
{{- $insertClusterNameAction := (dict "action" "upsert" "key" "k8s.cluster.name" "value" .Values.clusterName) -}}
{{- $_ := set $resourceProcessor "attributes" (append $resourceAttributes $insertClusterNameAction) -}}
{{- end }}

{{- /* Setup default memory_limiter processor configuration */}}
{{- if hasKey $processors "memory_limiter" }}
{{- $memoryLimiter := index $processors "memory_limiter" }}
{{- $_ := set $memoryLimiter "limit_mib" (include "o11y-collector.getOtelMemLimitMib" .Values.otelK8sClusterReceiver) }}
{{- $_ := set $memoryLimiter "spike_limit_mib" (include "o11y-collector.getOtelMemSpikeLimitMib" .Values.otelK8sClusterReceiver) }}
{{- $_ := set $memoryLimiter "ballast_size_mib" (include "o11y-collector.getOtelMemBallastSizeMib" .Values.otelK8sClusterReceiver) }}
{{- end }}

{{- $config | toYaml | nindent 4 }}

{{- end -}}
