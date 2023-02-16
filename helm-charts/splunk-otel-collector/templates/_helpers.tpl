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
Whether to send data to Splunk Platform endpoint
*/}}
{{- define "splunk-otel-collector.splunkPlatformEnabled" -}}
{{- not (eq .Values.splunkPlatform.endpoint "") }}
{{- end -}}

{{/*
Whether to send data to Splunk Observability endpoint
*/}}
{{- define "splunk-otel-collector.splunkO11yEnabled" -}}
{{- not (eq (include "splunk-otel-collector.o11yRealm" .) "") }}
{{- end -}}

{{/*
Whether metrics enabled for Splunk Observability, backward compatible.
*/}}
{{- define "splunk-otel-collector.o11yMetricsEnabled" -}}
{{- if eq (toString .Values.metricsEnabled) "<nil>" }}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.splunkObservability.metricsEnabled }}
{{- else }}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.metricsEnabled }}
{{- end -}}
{{- end -}}

{{/*
Whether traces enabled for Splunk Observability, backward compatible.
*/}}
{{- define "splunk-otel-collector.o11yTracesEnabled" -}}
{{- if eq (toString .Values.tracesEnabled) "<nil>" }}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.splunkObservability.tracesEnabled }}
{{- else }}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.tracesEnabled }}
{{- end -}}
{{- end -}}

{{/*
Whether logs enabled for Splunk Observability, backward compatible.
*/}}
{{- define "splunk-otel-collector.o11yLogsEnabled" -}}
{{- if eq (toString .Values.logsEnabled) "<nil>" }}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.splunkObservability.logsEnabled }}
{{- else }}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.logsEnabled }}
{{- end -}}
{{- end -}}

{{/*
Whether Splunk Observability Profiling is enabled.
*/}}
{{- define "splunk-otel-collector.o11yProfilingEnabled" -}}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.splunkObservability.profilingEnabled }}
{{- end -}}

{{/*
Whether logs or profiling is enabled for Splunk Observability.
*/}}
{{- define "splunk-otel-collector.o11yLogsOrProfilingEnabled" -}}
{{- or (eq (include "splunk-otel-collector.o11yLogsEnabled" .) "true") (eq (include "splunk-otel-collector.o11yProfilingEnabled" .) "true") }}
{{- end -}}

{{/*
Whether logs enabled for Splunk Platform.
*/}}
{{- define "splunk-otel-collector.platformLogsEnabled" -}}
{{- and (eq (include "splunk-otel-collector.splunkPlatformEnabled" .) "true") .Values.splunkPlatform.logsEnabled }}
{{- end -}}

{{/*
Whether metrics enabled for Splunk Platform.
*/}}
{{- define "splunk-otel-collector.platformMetricsEnabled" -}}
{{- and (eq (include "splunk-otel-collector.splunkPlatformEnabled" .) "true") .Values.splunkPlatform.metricsEnabled }}
{{- end -}}

{{/*
Whether traces enabled for Splunk Platform.
*/}}
{{- define "splunk-otel-collector.platformTracesEnabled" -}}
{{- and (eq (include "splunk-otel-collector.splunkPlatformEnabled" .) "true") .Values.splunkPlatform.tracesEnabled }}
{{- end -}}

{{/*
Whether metrics enabled for any destination.
*/}}
{{- define "splunk-otel-collector.metricsEnabled" -}}
{{- or (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true") }}
{{- end -}}

{{/*
Whether traces enabled for any destination.
*/}}
{{- define "splunk-otel-collector.tracesEnabled" -}}
{{- or (eq (include "splunk-otel-collector.o11yTracesEnabled" .) "true") (eq (include "splunk-otel-collector.platformTracesEnabled" .) "true") }}
{{- end -}}

{{/*
Whether logs enabled for any destination.
*/}}
{{- define "splunk-otel-collector.logsEnabled" -}}
{{- or (eq (include "splunk-otel-collector.o11yLogsEnabled" .) "true") (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") }}
{{- end -}}

{{/*
Whether profiling data is enabled (applicable to Splunk Observability only).
*/}}
{{- define "splunk-otel-collector.profilingEnabled" -}}
{{- include "splunk-otel-collector.o11yProfilingEnabled" . }}
{{- end -}}

{{/*
Define name for the Splunk Secret
*/}}
{{- define "splunk-otel-collector.secret" -}}
{{- if .Values.secret.name -}}
{{- printf "%s" .Values.secret.name -}}
{{- else -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Define name for the etcd Secret
*/}}
{{- define "splunk-otel-collector.etcdSecret" -}}
{{- if .Values.agent.controlPlaneMetrics.etcd.secret.name -}}
{{- printf "%s" .Values.agent.controlPlaneMetrics.etcd.secret.name -}}
{{- else -}}
{{- default .Chart.Name .Values.nameOverride | printf "%s-etcd" | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "splunk-otel-collector.serviceAccountName" -}}
    {{ default (include "splunk-otel-collector.fullname" .) .Values.serviceAccount.name }}
{{- end -}}

{{/*
Get Splunk Observability Realm.
*/}}
{{- define "splunk-otel-collector.o11yRealm" -}}
{{- .Values.splunkObservability.realm | default .Values.splunkRealm | default "" }}
{{- end -}}


{{/*
Get Splunk ingest URL
*/}}
{{- define "splunk-otel-collector.o11yIngestUrl" -}}
{{- $realm := (include "splunk-otel-collector.o11yRealm" .) }}
{{- .Values.splunkObservability.ingestUrl | default .Values.ingestUrl | default (printf "https://ingest.%s.signalfx.com" $realm) }}
{{- end -}}

{{/*
Get Splunk API URL.
*/}}
{{- define "splunk-otel-collector.o11yApiUrl" -}}
{{- $realm := (include "splunk-otel-collector.o11yRealm" .) }}
{{- .Values.splunkObservability.apiUrl | default .Values.apiUrl | default (printf "https://api.%s.signalfx.com" $realm) }}
{{- end -}}

{{/*
Get Splunk Observability Access Token.
*/}}
{{- define "splunk-otel-collector.o11yAccessToken" -}}
{{- .Values.splunkObservability.accessToken | default .Values.splunkAccessToken | default "" -}}
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
{{- printf "%s:%s" .Values.image.otelcol.repository (.Values.image.otelcol.tag | default .Chart.AppVersion) -}}
{{- end -}}

{{/*
Create the patch-log-dirs image name.
*/}}
{{- define "splunk-otel-collector.image.initPatchLogDirs" -}}
{{- printf "%s:%s" .Values.image.initPatchLogDirs.repository .Values.image.initPatchLogDirs.tag | trimSuffix ":" -}}
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

{{/*
Create a filter expression for multiline logs configuration.
*/}}
{{- define "splunk-otel-collector.newlineExpr" }}
{{- $expr := "" }}
{{- if .namespaceName }}
{{- $useRegexp := eq (toString .namespaceName.useRegexp | default "false") "true" }}
{{- $expr = cat "(resource[\"k8s.namespace.name\"])" (ternary "matches" "==" $useRegexp) (quote .namespaceName.value) "&&" }}
{{- end }}
{{- if .podName }}
{{- $useRegexp := eq (toString .podName.useRegexp | default "false") "true" }}
{{- $expr = cat $expr "(resource[\"k8s.pod.name\"])" (ternary "matches" "==" $useRegexp) (quote .podName.value) "&&" }}
{{- end }}
{{- if .containerName }}
{{- $useRegexp := eq (toString .containerName.useRegexp | default "false") "true" }}
{{- $expr = cat $expr "(resource[\"k8s.container.name\"])" (ternary "matches" "==" $useRegexp) (quote .containerName.value) "&&" }}
{{- end }}
{{- $expr | trimSuffix "&&" | trim }}
{{- end -}}

{{/*
Create an identifier for multiline logs configuration.
*/}}
{{- define "splunk-otel-collector.newlineKey" }}
{{- $key := "" }}
{{- if .namespaceName }}
{{- $key = printf "%s_" .namespaceName.value }}
{{- end }}
{{- if .podName }}
{{- $key = printf "%s%s_" $key .podName.value }}
{{- end }}
{{- if .containerName }}
{{- $key = printf "%s%s" $key .containerName.value }}
{{- end }}
{{- $key | trimSuffix "_" }}
{{- end -}}

{{/*
Common labels shared by all Kubernetes objects in this chart.
*/}}
{{- define "splunk-otel-collector.commonLabels" -}}
app.kubernetes.io/name: {{ include "splunk-otel-collector.name" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end -}}

{{/*
cloudProvider helper to support backward compatibility with the deprecated name.
*/}}
{{- define "splunk-otel-collector.cloudProvider" -}}
{{- .Values.cloudProvider | default .Values.provider | default "" -}}
{{- end -}}

{{/*
distribution helper to support backward compatibility with the deprecated name.
*/}}
{{- define "splunk-otel-collector.distribution" -}}
{{- .Values.distribution | default .Values.distro | default "" -}}
{{- end -}}

{{/*
Helper that returns "agent" parameter group yaml taking care of backward
compatibility with the old config group name: "otelAgent".
*/}}
{{- define "splunk-otel-collector.agent" -}}
{{- if eq (toString .Values.otelAgent) "<nil>" }}
{{- .Values.agent | toYaml }}
{{- else }}
{{- deepCopy .Values.otelAgent | mustMergeOverwrite (deepCopy .Values.agent) | toYaml }}
{{- end }}
{{- end -}}

{{/*
The name of the gateway service.
*/}}
{{- define "splunk-otel-collector.gatewayServiceName" -}}
{{  (include "splunk-otel-collector.fullname" . ) | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{/*
Whether the gateway is enabled, either through network explorer, or through its own flag.
*/}}
{{- define "splunk-otel-collector.gatewayEnabled" -}}
{{- $gateway := fromYaml (include "splunk-otel-collector.gateway" .) }}
{{- or $gateway.enabled .Values.networkExplorer.enabled }}
{{- end -}}

{{/*
Helper that returns "gateway" parameter group yaml taking care of backward
compatibility with the old config group name: "otelCollector".
*/}}
{{- define "splunk-otel-collector.gateway" -}}
{{- if eq (toString .Values.otelCollector) "<nil>" }}
{{- .Values.gateway | toYaml }}
{{- else }}
{{- deepCopy .Values.otelCollector | mustMergeOverwrite (deepCopy .Values.gateway) | toYaml }}
{{- end }}
{{- end -}}

{{/*
Helper that returns "clusterReceiver" parameter group yaml taking care of backward
compatibility with the old config group name: "otelK8sClusterReceiver".
*/}}
{{- define "splunk-otel-collector.clusterReceiver" -}}
{{- if eq (toString .Values.otelK8sClusterReceiver) "<nil>" }}
{{- .Values.clusterReceiver | toYaml }}
{{- else }}
{{- deepCopy .Values.otelK8sClusterReceiver | mustMergeOverwrite (deepCopy .Values.clusterReceiver) | toYaml }}
{{- end }}
{{- end -}}

{{/*
"clusterReceiverTruncatedName" for the eks/fargate cluster receiver statefulSet name accounting for 11 appended random chars
*/}}
{{- define "splunk-otel-collector.clusterReceiverTruncatedName" -}}
{{ printf "%s-k8s-cluster-receiver" ( include "splunk-otel-collector.fullname" . ) | trunc 52 | trimSuffix "-" }}
{{- end -}}

{{/*
"clusterReceiverServiceName" for the eks/fargate cluster receiver statefulSet headless service
*/}}
{{- define "splunk-otel-collector.clusterReceiverServiceName" -}}
{{ printf "%s-k8s-cluster-receiver" ( include "splunk-otel-collector.fullname" . ) | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{/*
"clusterReceiverNodeDiscovererScript" for the eks/fargate cluster receiver statefulSet initContainer
*/}}
{{- define "splunk-otel-collector.clusterReceiverNodeDiscovererScript" -}}
{{ printf "%s-cr-node-discoverer-script" ( include "splunk-otel-collector.fullname" . ) | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{/*
"o11yInfraMonEventsEnabled" helper defines whether Observability Infrastructure monitoring events are enabled
*/}}
{{- define "splunk-otel-collector.o11yInfraMonEventsEnabled" -}}
{{- $clusterReceiver := fromYaml (include "splunk-otel-collector.clusterReceiver" .) }}
{{- if eq (toString $clusterReceiver.k8sEventsEnabled) "<nil>" }}
{{- .Values.splunkObservability.infrastructureMonitoringEventsEnabled }}
{{- else }}
{{- $clusterReceiver.k8sEventsEnabled }}
{{- end }}
{{- end -}}


{{/*
Whether object collection by k8s object receiver is enabled
*/}}
{{- define "splunk-otel-collector.objectsEnabled" -}}
{{- $clusterReceiver := fromYaml (include "splunk-otel-collector.clusterReceiver" .) }}
{{- gt (len $clusterReceiver.k8sObjects) 0 }}
{{- end -}}

{{/*
Whether object collection by k8s object receiver or/and event collection by k8s event receiver is enabled
*/}}
{{- define "splunk-otel-collector.objectsOrEventsEnabled" -}}
{{- $clusterReceiver := fromYaml (include "splunk-otel-collector.clusterReceiver" .) }}
{{- or $clusterReceiver.eventsEnabled (eq (include "splunk-otel-collector.objectsEnabled" .) "true") -}}
{{- end -}}


{{/*
Whether clusterReceiver should be enabled
*/}}
{{- define "splunk-otel-collector.clusterReceiverEnabled" -}}
{{- $clusterReceiver := fromYaml (include "splunk-otel-collector.clusterReceiver" .) }}
{{- and $clusterReceiver.enabled (or (eq (include "splunk-otel-collector.metricsEnabled" .) "true") (eq (include "splunk-otel-collector.objectsOrEventsEnabled" .) "true")) -}}
{{- end -}}


{{/*
Whether persistentQueue should be enabled
*/}}
{{- define "splunk-otel-collector.persistentQueueEnabled" -}}
{{- or .Values.splunkPlatform.persistentQueueEnabled.logs .Values.splunkPlatform.persistentQueueEnabled.metrics  -}}
{{- end -}}
