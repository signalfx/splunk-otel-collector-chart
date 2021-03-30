{{/* vim: set filetype=mustache: */}}

{{/*
Expand the name of the chart.
*/}}
{{- define "splunk-otel-collector.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "splunk-otel-collector.fullname" -}}
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
{{- define "splunk-otel-collector.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "splunk-otel-collector.secret" -}}
{{- if .Values.secret.name -}}
{{- printf "%s" .Values.secret.name -}}
{{- else -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "splunk-otel-collector.serviceAccountName" -}}
    {{ default (include "splunk-otel-collector.fullname" .) .Values.serviceAccount.name }}
{{- end -}}

{{/*
Get Splunk ingest host
*/}}
{{- define "splunk-otel-collector.ingestHost" -}}
{{- $_ := required "splunkRealm or ingestHost must be provided" (or .Values.ingestHost .Values.splunkRealm) }}
{{- .Values.ingestHost | default (printf "ingest.%s.signalfx.com" .Values.splunkRealm) }}
{{- end -}}

{{/*
Get Splunk log URL
*/}}
{{- define "splunk-otel-collector.logUrl" -}}
{{- $host := include "splunk-otel-collector.ingestHost" . }}
{{- $endpoint := printf "%s://%s" .Values.ingestProtocol $host }}
{{- if or (and (eq .Values.ingestProtocol "http") (ne (toString .Values.ingestPort) "80")) (and (eq .Values.ingestProtocol "https") (ne (toString .Values.ingestPort) "443")) }}
{{- printf "%s:%s/v1/log" $endpoint (toString .Values.ingestPort) }}
{{- else }}
{{- $endpoint }}
{{- end }}
{{- end -}}

{{/*
Get Splunk ingest URL
*/}}
{{- define "splunk-otel-collector.ingestUrl" -}}
{{- $host := include "splunk-otel-collector.ingestHost" . }}
{{- $endpoint := printf "%s://%s" .Values.ingestProtocol $host }}
{{- if or (and (eq .Values.ingestProtocol "http") (ne (toString .Values.ingestPort) "80")) (and (eq .Values.ingestProtocol "https") (ne (toString .Values.ingestPort) "443")) }}
{{- printf "%s:%s" $endpoint (toString .Values.ingestPort) }}
{{- else }}
{{- $endpoint }}
{{- end }}
{{- end -}}

{{/*
Get Splunk API URL.
*/}}
{{- define "splunk-otel-collector.apiUrl" -}}
{{- $_ := required "splunkRealm or apiUrl must be provided" (or .Values.apiUrl .Values.splunkRealm) }}
{{- .Values.apiUrl | default (printf "https://api.%s.signalfx.com" .Values.splunkRealm) }}
{{- end -}}

{{/*
Get splunkAccessToken.
*/}}
{{- define "splunk-otel-collector.accessToken" -}}
{{- required "splunkAccessToken value must be provided" .Values.splunkAccessToken -}}
{{- end -}}

{{/*
Create the fluentd image name.
*/}}
{{- define "splunk-otel-collector.image.fluentd" -}}
{{- printf "%s:%s" .Values.image.fluentd.repository .Values.image.fluentd.tag | trimSuffix ":" -}}
{{- end -}}

{{/*
Create the opentelemetry collector image name.
*/}}
{{- define "splunk-otel-collector.image.otelcol" -}}
{{- printf "%s:%s" .Values.image.otelcol.repository .Values.image.otelcol.tag | trimSuffix ":" -}}
{{- end -}}

{{/*
Convert memory value from resources.limit to numeric value in MiB to be used by otel memory_limiter processor.
*/}}
{{- define "splunk-otel-collector.convertMemToMib" -}}
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
