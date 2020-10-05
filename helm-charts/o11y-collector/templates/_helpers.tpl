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
{{- $_ := required "splunkRealm or ingestHost must be provided" (or .Values.ingestHost .Values.splunkRealm) }}
{{- .Values.ingestHost | default (printf "ingest.%s.signalfx.com" .Values.splunkRealm) }}
{{- end -}}

{{/*
Get Signalfx ingest URL
*/}}
{{- define "o11y-collector.ingestUrl" -}}
{{- $host := include "o11y-collector.ingestHost" . }}
{{- $endpoint := printf "%s://%s" .Values.ingestProtocol $host }}
{{- if or (and (eq .Values.ingestProtocol "http") (ne (toString .Values.ingestPort) "80")) (and (eq .Values.ingestProtocol "https") (ne (toString .Values.ingestPort) "443")) }}
{{- printf "%s:%s" $endpoint (toString .Values.ingestPort) }}
{{- else }}
{{- $endpoint }}
{{- end }}
{{- end -}}

{{/*
Get Signalfx API URL.
*/}}
{{- define "o11y-collector.apiUrl" -}}
{{- $_ := required "splunkRealm or apiUrl must be provided" (or .Values.apiUrl .Values.splunkRealm) }}
{{- .Values.apiUrl | default (printf "https://api.%s.signalfx.com" .Values.splunkRealm) }}
{{- end -}}

{{/*
Get splunkAccessToken.
*/}}
{{- define "o11y-collector.accessToken" -}}
{{- required "splunkAccessToken value must be provided" .Values.splunkAccessToken -}}
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
Get otel memory_limiter limit_mib value based on 80% of resources.memory.limit.
*/}}
{{- define "o11y-collector.getOtelMemLimitMib" -}}
{{- div (mul (include "o11y-collector.convertMemToMib" .resources.limits.memory) 80) 100 }}
{{- end -}}

{{/*
Get otel memory_limiter spike_limit_mib value based on 25% of resources.memory.limit.
*/}}
{{- define "o11y-collector.getOtelMemSpikeLimitMib" -}}
{{- div (mul (include "o11y-collector.convertMemToMib" .resources.limits.memory) 25) 100 }}
{{- end -}}

{{/*
Get otel memory_limiter ballast_size_mib value based on 40% of resources.memory.limit.
*/}}
{{- define "o11y-collector.getOtelMemBallastSizeMib" }}
{{- div (mul (include "o11y-collector.convertMemToMib" .resources.limits.memory) 40) 100 }}
{{- end -}}
