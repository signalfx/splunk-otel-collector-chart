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
Fail if a collector config override still references Splunk token environment variables
when token file mounting is enabled.
*/}}
{{- define "splunk-otel-collector.failOnTokenEnvVarRefs" -}}
{{- if and .enabled .config -}}
{{- $source := .source -}}
{{- if regexMatch "\\$\\{SPLUNK_[A-Z0-9_]*_TOKEN\\}" (toYaml .config) }}
{{- fail (printf "%s references a Splunk token environment variable (${SPLUNK_*_TOKEN}) while featureGates.mountSplunkSecretAsFile is enabled. Built-in chart configuration reads tokens from mounted Secret files and no longer injects token environment variables. Please update custom collector config to use ${file:/otel/etc/splunk_observability_access_token} or ${file:/otel/etc/splunk_platform_hec_token}." $source) -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Whether Splunk tokens should be mounted and read as files instead of environment variables.
*/}}
{{- define "splunk-otel-collector.splunkSecretTokenFileEnabled" -}}
{{- if .Values.featureGates.mountSplunkSecretAsFile }}true{{- else }}false{{- end }}
{{- end -}}

{{/*
Collector config value for the Splunk Observability access token.
*/}}
{{- define "splunk-otel-collector.splunkObservabilityAccessToken" -}}
{{- if eq (include "splunk-otel-collector.splunkSecretTokenFileEnabled" .) "true" -}}
${file:/otel/etc/splunk_observability_access_token}
{{- else -}}
${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}
{{- end -}}
{{- end -}}

{{/*
Collector config value for the Splunk Platform HEC token.
*/}}
{{- define "splunk-otel-collector.splunkPlatformHecToken" -}}
{{- if eq (include "splunk-otel-collector.splunkSecretTokenFileEnabled" .) "true" -}}
${file:/otel/etc/splunk_platform_hec_token}
{{- else -}}
${SPLUNK_PLATFORM_HEC_TOKEN}
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
Whether logs should be sent via OTLP to Splunk Connect for OTLP instead of HEC.
*/}}
{{- define "splunk-otel-collector.platformLogsViaOtlpEnabled" -}}
{{- and ((.Values.splunkPlatform.otlpIngest).enabled) .Values.splunkPlatform.logsEnabled }}
{{- end -}}

{{/*
The exporter name for platform logs sent via OTLP (otlp/platform_logs or otlp_http/platform_logs).
*/}}
{{- define "splunk-otel-collector.otlpPlatformLogsExporterName" -}}
{{- if eq .Values.splunkPlatform.otlpIngest.protocol "http" }}otlp_http{{- else }}otlp{{- end }}/platform_logs
{{- end -}}

{{/*
Whether the Splunk Platform secret must be mounted as files for HEC or OTLP TLS.
*/}}
{{- define "splunk-otel-collector.platformTlsSecretMountRequired" -}}
{{- if or
      .Values.splunkPlatform.clientCert
      .Values.splunkPlatform.clientKey
      .Values.splunkPlatform.caFile
      .Values.splunkPlatform.otlpIngest.clientCert
      .Values.splunkPlatform.otlpIngest.clientKey
      .Values.splunkPlatform.otlpIngest.caFile }}true{{- else }}false{{- end }}
{{- end -}}

{{/*
Whether the Splunk Secret must be mounted as files for tokens (i.e. o11y access token or HEC token) or platform TLS.
*/}}
{{- define "splunk-otel-collector.secretMountRequired" -}}
{{- if or
      (and
        (eq (include "splunk-otel-collector.splunkSecretTokenFileEnabled" .) "true")
        (or
          (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true")
          (eq (include "splunk-otel-collector.platformHecTokenRequired" .) "true")))
      (eq (include "splunk-otel-collector.platformTlsSecretMountRequired" .) "true") }}true{{- else }}false{{- end }}
{{- end -}}

{{/*
Whether the Splunk Secret should be created by the chart.
*/}}
{{- define "splunk-otel-collector.secretCreateRequired" -}}
{{- if and .Values.secret.create (or
      (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true")
      (eq (include "splunk-otel-collector.platformHecTokenRequired" .) "true")
      .Values.splunkPlatform.clientCert
      .Values.splunkPlatform.clientKey
      .Values.splunkPlatform.caFile
      .Values.splunkPlatform.otlpIngest.clientCert
      .Values.splunkPlatform.otlpIngest.clientKey
      .Values.splunkPlatform.otlpIngest.caFile) }}true{{- else }}false{{- end }}
{{- end -}}

{{/*
Whether data is sent to the Splunk Platform HEC endpoint.
*/}}
{{- define "splunk-otel-collector.platformHecEndpointEnabled" -}}
{{- or
      (and
        (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true")
        (not (eq (include "splunk-otel-collector.platformLogsViaOtlpEnabled" .) "true")))
      (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true")
      (eq (include "splunk-otel-collector.platformTracesEnabled" .) "true") }}
{{- end -}}

{{/*
Whether Splunk Platform HEC token is required.
*/}}
{{- define "splunk-otel-collector.platformHecTokenRequired" -}}
{{- include "splunk-otel-collector.platformHecEndpointEnabled" . }}
{{- end -}}

{{/*
Whether to send data to Splunk Observability endpoint
*/}}
{{- define "splunk-otel-collector.splunkO11yEnabled" -}}
{{- not (eq .Values.splunkObservability.realm "") }}
{{- end -}}

{{/*
Whether metrics enabled for Splunk Observability, backward compatible.
*/}}
{{- define "splunk-otel-collector.o11yMetricsEnabled" -}}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.splunkObservability.metricsEnabled }}
{{- end -}}

{{/*
Whether traces enabled for Splunk Observability, backward compatible.
*/}}
{{- define "splunk-otel-collector.o11yTracesEnabled" -}}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.splunkObservability.tracesEnabled }}
{{- end -}}

{{/*
Whether Splunk Observability Profiling is enabled.
*/}}
{{- define "splunk-otel-collector.o11yProfilingEnabled" -}}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.splunkObservability.profilingEnabled }}
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
{{- or (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") (eq (include "splunk-otel-collector.platformLogsViaOtlpEnabled" .) "true") }}
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
{{- default (include "splunk-otel-collector.fullname" .) .Values.secret.name }}
{{- end -}}

{{/*
Define name for the etcd Secret
*/}}
{{- define "splunk-otel-collector.etcdSecret" -}}
{{- if .Values.agent.controlPlaneMetrics.etcd.secret.name -}}
{{- printf "%s" .Values.agent.controlPlaneMetrics.etcd.secret.name -}}
{{- else -}}
{{- $name := (include "splunk-otel-collector.fullname" .) -}}
{{- printf "%s-etcd" $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "splunk-otel-collector.serviceAccountName" -}}
    {{ default (include "splunk-otel-collector.fullname" .) .Values.serviceAccount.name }}
{{- end -}}

{{/*
Get Splunk ingest URL
*/}}
{{- define "splunk-otel-collector.o11yIngestUrl" -}}
{{- $realm := .Values.splunkObservability.realm }}
{{- .Values.splunkObservability.ingestUrl | default (printf "https://ingest.%s.observability.splunkcloud.com" $realm) }}
{{- end -}}

{{/*
Get Splunk API URL.
*/}}
{{- define "splunk-otel-collector.o11yApiUrl" -}}
{{- $realm := .Values.splunkObservability.realm }}
{{- .Values.splunkObservability.apiUrl | default (printf "https://api.%s.observability.splunkcloud.com" $realm) }}
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
Create the validateSecret image name.
*/}}
{{- define "splunk-otel-collector.image.validateSecret" -}}
{{- printf "%s:%s" .Values.image.validateSecret.repository .Values.image.validateSecret.tag | trimSuffix ":" -}}
{{- end -}}

{{/*
Create the OpAMP Bridge image name.
*/}}
{{- define "splunk-otel-collector.image.opampBridge" -}}
{{- printf "%s:%s" .Values.remoteManagement.opampBridge.image.repository .Values.remoteManagement.opampBridge.image.tag | trimSuffix ":" -}}
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
app.kubernetes.io/managed-by: Helm
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end -}}

{{/*
The apiVersion for podDisruptionBudget policies.
*/}}
{{- define "splunk-otel-collector.PDB-apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "policy/v1" -}}
{{- print "policy/v1" -}}
{{- else -}}
{{- print "policy/v1beta1" -}}
{{- end -}}
{{- end -}}

{{/*
The name of the gateway service.
*/}}
{{- define "splunk-otel-collector.gatewayServiceName" -}}
{{  (include "splunk-otel-collector.fullname" . ) | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{/*
The name of the gateway headless service used for StatefulSet identity.
Truncating the base name to 54 characters to allow for the "-headless" suffix, which is 9 characters long, resulting in a total of 63 characters.
This guarantees that `-headless` is part of the name and not truncated, which is important for StatefulSet identity.
*/}}
{{- define "splunk-otel-collector.gatewayHeadlessServiceName" -}}
{{- $base := (include "splunk-otel-collector.gatewayServiceName" .) | trunc 54 | trimSuffix "-" -}}
{{ printf "%s-headless" $base | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{/*
Render gateway service ports using the same filtering rules for all gateway services.
*/}}
{{- define "splunk-otel-collector.gatewayServicePorts" -}}
{{- range $key, $port := .Values.gateway.ports }}
{{- $metricsEnabled := and (eq (include "splunk-otel-collector.metricsEnabled" $) "true") (has "metrics" $port.enabled_for) }}
{{- $tracesEnabled := and (eq (include "splunk-otel-collector.tracesEnabled" $) "true") (has "traces" $port.enabled_for) }}
{{- $logsEnabled := and (eq (include "splunk-otel-collector.logsEnabled" $) "true") (has "logs" $port.enabled_for) }}
{{- $profilingEnabled := and (eq (include "splunk-otel-collector.profilingEnabled" $) "true") (has "profiling" $port.enabled_for) }}
{{- if or $metricsEnabled $tracesEnabled $logsEnabled $profilingEnabled }}
- name: {{ $key }}
  port: {{ $port.containerPort }}
  targetPort: {{ $key }}
  protocol: {{ $port.protocol }}
{{- end }}
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
{{- if eq (toString .Values.clusterReceiver.k8sEventsEnabled) "<nil>" }}
{{- .Values.splunkObservability.infrastructureMonitoringEventsEnabled }}
{{- else }}
{{- .Values.clusterReceiver.k8sEventsEnabled }}
{{- end }}
{{- end -}}

{{/*
Determines whether the k8s events pipeline will be enabled in the k8s cluster receiver config
*/}}
{{- define "splunk-otel-collector.clusterReceiverEventsPipelineEnabled" -}}
{{- and .Values.clusterReceiver.eventsEnabled
        (or
          (eq (include "splunk-otel-collector.logsEnabled" .) "true")
          (eq (include "splunk-otel-collector.splunkO11yEventsEndpointEnabled" .) "true")) }}
{{- end -}}

{{/*
Whether object collection by k8s object receiver is enabled
*/}}
{{- define "splunk-otel-collector.objectsEnabled" -}}
{{- gt (len .Values.clusterReceiver.k8sObjects) 0 }}
{{- end -}}

{{/*
Determines whether the k8s object pipeline will be enabled in the k8s cluster receiver config
*/}}
{{- define "splunk-otel-collector.clusterReceiverObjectsPipelineEnabled" -}}
{{- and (eq (include "splunk-otel-collector.objectsEnabled" .) "true")
        (or
          (eq (include "splunk-otel-collector.logsEnabled" .) "true")
          (eq (include "splunk-otel-collector.splunkO11yEventsEndpointEnabled" .) "true")) }}
{{- end -}}

{{/*
Whether object collection by k8s object receiver or/and event collection by k8s event receiver is enabled
*/}}
{{- define "splunk-otel-collector.objectsOrEventsEnabled" -}}
{{- or .Values.clusterReceiver.eventsEnabled (eq (include "splunk-otel-collector.objectsEnabled" .) "true") -}}
{{- end -}}

{{/*
Whether sending to Splunk Observability v3/event endpoint is enabled
*/}}
{{- define "splunk-otel-collector.splunkO11yEventsEndpointEnabled" -}}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.featureGates.sendK8sEventsToSplunkO11y -}}
{{- end -}}

{{/*
[EXPERIMENTAL] Whether the k8s entities pipeline is enabled.
Sends k8s_cluster receiver data to Splunk Observability v3/event endpoint via otlp_http.
*/}}
{{- define "splunk-otel-collector.k8sEntitiesEnabled" -}}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.featureGates.enableK8sEntities -}}
{{- end -}}

{{/*
Build the collector feature gates string for cluster receiver.
*/}}
{{- define "splunk-otel-collector.clusterReceiverFeatureGates" -}}
{{- $gates := list -}}
{{- with .Values.clusterReceiver.featureGates -}}
{{- $gates = append $gates . -}}
{{- end -}}
{{- if and (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") .Values.featureGates.useEntityEventsForK8sProperties -}}
{{- $gates = append $gates "exporter.signalfx.consumeEntityEvents" -}}
{{- end -}}
{{- join "," $gates -}}
{{- end -}}

{{/*
Whether clusterReceiver should be enabled
*/}}
{{- define "splunk-otel-collector.clusterReceiverEnabled" -}}
{{- and .Values.clusterReceiver.enabled (or (eq (include "splunk-otel-collector.metricsEnabled" .) "true") (eq (include "splunk-otel-collector.objectsOrEventsEnabled" .) "true") (eq (include "splunk-otel-collector.k8sEntitiesEnabled" .) "true")) -}}
{{- end -}}


{{/*
Build the securityContext for Linux and Windows
*/}}
{{- define "splunk-otel-collector.securityContext" -}}
{{- if .isWindows }}
{{- $_ := unset .securityContext "runAsUser" }}
{{- $_ := unset .securityContext "fsGroup" }}
{{- $_ := unset .securityContext "fsGroupChangePolicy" }}
{{- if and (.setRunAsUser) (not (hasKey .securityContext "windowsOptions"))}}
{{- $_ := set .securityContext "windowsOptions" dict }}
{{- end }}
{{- if and (.setRunAsUser) (not (hasKey .securityContext.windowsOptions "runAsUserName")) }}
{{- $_ := set .securityContext.windowsOptions "runAsUserName" "ContainerAdministrator"}}
{{- end }}
{{- else }}
{{- if and (eq (toString .securityContext.runAsUser) "<nil>") (.setRunAsUser) }}
{{- $_ := set .securityContext "runAsUser" 0 }}
{{- end }}
{{- end }}
{{- if .securityContext }}
{{- toYaml .securityContext }}
{{- end }}
{{- end -}}

{{/*
Build a pod securityContext for Linux and Windows.
*/}}
{{- define "splunk-otel-collector.podSecurityContext" -}}
{{- $podSecurityContext := deepCopy (.podSecurityContext | default dict) -}}
{{- include "splunk-otel-collector.securityContext" (dict "isWindows" .isWindows "securityContext" $podSecurityContext) }}
{{- end -}}

{{/*
Whether the clusterName configuration option is optional
*/}}
{{- define "splunk-otel-collector.clusterNameOptional" -}}
{{- or (hasPrefix "gke" .Values.distribution) (eq (include "splunk-otel-collector.isNonFargateEKS" .) "true") (eq .Values.distribution "openshift") }}
{{- end -}}

{{/*
Whether the helm chart should detect the cluster name automatically
*/}}
{{- define "splunk-otel-collector.autoDetectClusterName" -}}
{{- and (eq (include "splunk-otel-collector.clusterNameOptional" .) "true") (not .Values.clusterName) }}
{{- end -}}

{{/*
Helper used to define a namespace.
- Returns namespace from a release
- If namespaceOverride value is filled in it will replace the namespace
*/}}
{{- define "splunk-otel-collector.namespace" -}}
  {{- default .Release.Namespace .Values.namespaceOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Returns true if the distribution is eks but not eks/fargate.
*/}}
{{- define "splunk-otel-collector.isNonFargateEKS" -}}
{{- and (hasPrefix "eks" .Values.distribution) (ne .Values.distribution "eks/fargate") -}}
{{- end -}}

{{/*
Identifies K8s cluster running on AWS but not EKS.
Returns true if the cloud provider is aws and the distribution is not EKS-based.
Examples: Vanilla K8s on AWS EC2, OpenShift on AWS (ROSA)
*/}}
{{- define "splunk-otel-collector.isNonEKSonAWS" -}}
{{- and (eq .Values.cloudProvider "aws") (not (hasPrefix "eks" .Values.distribution)) -}}
{{- end -}}

{{/*
Determine if hostNetwork should be enabled.
If distribution is eks/auto-mode and hostNetwork is not explicitly set, it will be enabled.
*/}}
{{- define "splunk-otel-collector.clusterReceiverHostNetworkEnabled" -}}
{{- if eq (toString .Values.clusterReceiver.hostNetwork) "<nil>" }}
  {{- eq .Values.distribution "eks/auto-mode" }}
{{- else }}
  {{- .Values.clusterReceiver.hostNetwork }}
{{- end -}}
{{- end -}}

{{/*
  Helper to get the effective service config for a component (e.g., gateway, agent).
  If .Values.service is not an empty map, use it for backward compatibility.
  Otherwise, use the nested service config (e.g., .Values.gateway.service).
  Usage: {{ include "splunk-otel-collector.getServiceConfig" (dict "context" . "svc" .Values.gateway.service) | fromYaml }}
*/}}
{{- define "splunk-otel-collector.getServiceConfig" -}}
{{- $svc := .svc -}}
{{- $values := .context.Values }}
{{- $useOldService := and (hasKey $values "service") (gt (len $values.service) 0) }}
{{- toYaml (ternary $values.service $svc $useOldService) -}}
{{- end -}}

{{/*
Return the journald directory path based on useHostJournalctl value.
If useHostJournalctl is true, concatenate root_path and directory.
Otherwise, return directory only.
*/}}
{{- define "splunk-otel-collector.journaldDirectory" -}}
{{- if .Values.logsCollection.journald.useHostJournalctl -}}
  {{- printf "%s%s" .Values.logsCollection.journald.root_path .Values.logsCollection.journald.directory -}}
{{- else -}}
  {{- .Values.logsCollection.journald.directory -}}
{{- end -}}
{{- end -}}

{{/*
Fail if a collector config override uses deprecated component names.
Checks exporter/processor/receiver definitions and pipeline references.

To add a new deprecation, add an entry to $depExporters, $depProcessors, or $depReceivers.

Usage:
  include "splunk-otel-collector.failOnDeprecatedNames" (dict "config" .Values.agent.config "source" "agent.config")
*/}}
{{- define "splunk-otel-collector.failOnDeprecatedNames" -}}
{{- if .config -}}
{{- $source := .source -}}
{{- $depExporters := dict "otlp" "otlp_grpc" "otlphttp" "otlp_http" -}}
{{- $depProcessors := dict "k8sattributes" "k8s_attributes" "resourcedetection" "resource_detection" -}}
{{- $depReceivers := dict "filelog" "file_log" "hostmetrics" "host_metrics" "k8sobjects" "k8s_objects" "kubeletstats" "kubelet_stats" -}}
{{- range $key, $_ := (dig "exporters" (dict) .config) -}}
  {{- range $old, $new := $depExporters -}}
    {{- if or (eq $key $old) (hasPrefix (printf "%s/" $old) $key) -}}
      {{- fail (printf "%s.exporters.%s: \"%s\" has been renamed to \"%s\". Please update your custom configuration." $source $key $old $new) -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- range $key, $_ := (dig "processors" (dict) .config) -}}
  {{- range $old, $new := $depProcessors -}}
    {{- if or (eq $key $old) (hasPrefix (printf "%s/" $old) $key) -}}
      {{- fail (printf "%s.processors.%s: \"%s\" has been renamed to \"%s\". Please update your custom configuration." $source $key $old $new) -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- range $key, $_ := (dig "receivers" (dict) .config) -}}
  {{- range $old, $new := $depReceivers -}}
    {{- if or (eq $key $old) (hasPrefix (printf "%s/" $old) $key) -}}
      {{- fail (printf "%s.receivers.%s: \"%s\" has been renamed to \"%s\". Please update your custom configuration." $source $key $old $new) -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- range $pname, $p := (dig "service" "pipelines" (dict) .config) -}}
  {{- if $p -}}
  {{- range $item := (dig "exporters" (list) $p) -}}
    {{- range $old, $new := $depExporters -}}
      {{- if or (eq $item $old) (hasPrefix (printf "%s/" $old) $item) -}}
        {{- fail (printf "%s.service.pipelines.%s.exporters references \"%s\": \"%s\" has been renamed to \"%s\". Please update your custom configuration." $source $pname $item $old $new) -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
  {{- range $item := (dig "processors" (list) $p) -}}
    {{- range $old, $new := $depProcessors -}}
      {{- if or (eq $item $old) (hasPrefix (printf "%s/" $old) $item) -}}
        {{- fail (printf "%s.service.pipelines.%s.processors references \"%s\": \"%s\" has been renamed to \"%s\". Please update your custom configuration." $source $pname $item $old $new) -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
    {{- range $item := (dig "receivers" (list) $p) -}}
    {{- range $old, $new := $depReceivers -}}
      {{- if or (eq $item $old) (hasPrefix (printf "%s/" $old) $item) -}}
        {{- fail (printf "%s.service.pipelines.%s.receivers references \"%s\": \"%s\" has been renamed to \"%s\". Please update your custom configuration." $source $pname $item $old $new) -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
  {{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create the target allocator endpoint to match subchart's naming logic
*/}}
{{- define "splunk-otel-collector.targetAllocatorFullname" -}}
{{- if .Values.targetallocator.fullnameOverride -}}
{{- .Values.targetallocator.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default "targetallocator" .Values.targetallocator.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Whether Db2 query events should be routed to Observability Cloud Database Monitoring.
Read defensively: the db2 block always exists so the subchart condition resolves, but it
need not carry a dbMonitoring key.
*/}}
{{- define "splunk-otel-collector.db2DbMonRequested" -}}
{{- $db2 := .Values.db2 | default dict -}}
{{- if and (dig "enabled" false $db2) (dig "dbMonitoring" "enabled" false $db2) -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{/*
Whether the Db2 Database Monitoring config should render. Deliberately narrower than
"requested": the destination is Observability Cloud, so there is nothing to render without
it. Validation keys off the request instead, so asking for this without a destination is an
error rather than a silent no-op.
*/}}
{{- define "splunk-otel-collector.db2DbMonEnabled" -}}
{{- if and (eq (include "splunk-otel-collector.db2DbMonRequested" .) "true") (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{/*
The Db2 Database Monitoring pipeline lives in the gateway config, so refuse at template
time with something actionable rather than rendering a release that silently drops these
records.
*/}}
{{- define "splunk-otel-collector.validateDb2DbMonitoring" -}}
{{- if eq (include "splunk-otel-collector.db2DbMonRequested" .) "true" -}}
{{- /*
Checked against the request, not against what renders. Database Monitoring is an
Observability Cloud feature, so a Platform-only install renders no dbmon exporter and no
logs/dbmon pipeline at all -- asking for it there used to succeed and quietly do nothing.
*/ -}}
{{- if ne (include "splunk-otel-collector.splunkO11yEnabled" .) "true" -}}
{{- fail "db2.dbMonitoring.enabled requires Splunk Observability Cloud to be configured: set splunkObservability.realm and splunkObservability.accessToken. Database Monitoring has no Splunk Platform destination, so there is nothing for this setting to route to." -}}
{{- end -}}
{{- if not .Values.gateway.enabled -}}
{{- fail "db2.dbMonitoring.enabled currently requires gateway.enabled: true. The agent's entity pipeline does not carry the routing rule." -}}
{{- end -}}
{{- /*
The routing rule lives in the gateway config, but the subchart defaults to exporting to
the agent, and in Observability-only mode the agent has no pipeline receiving OTLP logs
(logs/entities reads from nop). OTLP registers a service per signal, so query-sample
events sent there are rejected outright with UNIMPLEMENTED rather than being dropped
quietly. Refuse the combination instead of installing something that cannot deliver.
*/ -}}
{{- $otlp := (.Values.db2).otlp | default dict -}}
{{- $endpoint := dig "endpoint" "" $otlp -}}
{{- $gatewayService := lower (include "splunk-otel-collector.fullname" .) -}}
{{- $gatewayNs := lower (include "splunk-otel-collector.namespace" .) -}}
{{- /* A subchart always installs into the release namespace; only the parent can move. */ -}}
{{- $db2Ns := lower .Release.Namespace -}}
{{- /*
The gateway's OTLP gRPC listener can be re-pointed or removed outright, and the receiver
speaks only gRPC. Read the configured port rather than assuming 4317, and refuse when the
port is gone, since then the gateway has nowhere to receive these events.
*/ -}}
{{- $otlpPort := dig "otlp" dict (.Values.gateway.ports | default dict) -}}
{{- if not $otlpPort -}}
{{- fail "db2.dbMonitoring.enabled requires the gateway's OTLP gRPC port, but gateway.ports.otlp is not configured. The Db2 receiver exports OTLP over gRPC, so without that port the gateway cannot receive these events." -}}
{{- end -}}
{{- $gatewayPort := toString (dig "containerPort" 4317 $otlpPort) -}}
{{- if $endpoint -}}
{{- /*
Parsed as a whole URL. Accepting the pieces loosely let through addresses the exporter
cannot use at all: no scheme, no port, "ftp://", or a stray second port segment.
*/ -}}
{{- $schemeSplit := splitList "://" $endpoint -}}
{{- if ne (len $schemeSplit) 2 -}}
{{- fail (printf "db2.otlp.endpoint must be a complete URL of the form http://host:port, got %q." $endpoint) -}}
{{- end -}}
{{- $scheme := lower (first $schemeSplit) -}}
{{- if not (has $scheme (list "http" "https")) -}}
{{- fail (printf "db2.otlp.endpoint must use the http or https scheme, got %q in %q." $scheme $endpoint) -}}
{{- end -}}
{{- $authority := index $schemeSplit 1 -}}
{{- if contains "/" $authority -}}
{{- fail (printf "db2.otlp.endpoint must be host:port with no path, got %q. The receiver exports OTLP over gRPC, which addresses a host and port only." $endpoint) -}}
{{- end -}}
{{- $hostPort := splitList ":" $authority -}}
{{- if ne (len $hostPort) 2 -}}
{{- fail (printf "db2.otlp.endpoint must specify exactly one host and port, got %q." $endpoint) -}}
{{- end -}}
{{- $host := lower (first $hostPort) -}}
{{- $port := index $hostPort 1 -}}
{{- if not (regexMatch "^[0-9]+$" $port) -}}
{{- fail (printf "db2.otlp.endpoint has a non-numeric port %q in %q." $port $endpoint) -}}
{{- end -}}
{{- /*
The short name only resolves from the Db2 pod when both sit in one namespace, which
namespaceOverride can break. Offer it only then; otherwise require a qualified name.
*/ -}}
{{- $accepted := list
      (printf "%s.%s" $gatewayService $gatewayNs)
      (printf "%s.%s.svc" $gatewayService $gatewayNs)
      (printf "%s.%s.svc.cluster.local" $gatewayService $gatewayNs) -}}
{{- if eq $gatewayNs $db2Ns -}}
{{- $accepted = prepend $accepted $gatewayService -}}
{{- end -}}
{{- if not (has $host $accepted) -}}
{{- fail (printf "db2.dbMonitoring.enabled requires the Db2 receiver to export to this release's gateway Service, but db2.otlp.endpoint resolves to host %q. Accepted forms are: %s. The Database Monitoring routing rule is part of this release's gateway config, so records sent anywhere else never reach it -- and the agent in particular has no OTLP logs pipeline, which rejects them with UNIMPLEMENTED." $host (join ", " $accepted)) -}}
{{- end -}}
{{- if ne $port $gatewayPort -}}
{{- fail (printf "db2.otlp.endpoint must use the gateway's OTLP gRPC port %s, but specifies %s. The Db2 receiver exports OTLP over gRPC; the HTTP port will not accept it." $gatewayPort $port) -}}
{{- end -}}
{{- else -}}
{{- if not (or
      (eq (dig "collectorTarget" "agent" $otlp) "gateway")
      (dig "gateway" "enabled" false (.Values.global | default dict))) -}}
{{- fail "db2.dbMonitoring.enabled requires the Db2 receiver to export to the gateway, but db2.otlp.collectorTarget is \"agent\". Set db2.otlp.collectorTarget: gateway. The agent has no OTLP logs pipeline in Observability-only mode, so query-sample events sent to it are rejected with UNIMPLEMENTED." -}}
{{- end -}}
{{- /*
The subchart rebuilds the gateway address from the release and chart names. Helm passes a
parent's own values down only through `global`, so nameOverride and fullnameOverride are
invisible to it and it derives a Service name that was never created -- an export into a
name that does not resolve, with nothing in the rendered output to show it.
*/ -}}
{{- if or .Values.nameOverride .Values.fullnameOverride -}}
{{- fail (printf "db2.dbMonitoring.enabled cannot derive the gateway address while nameOverride or fullnameOverride is set: the subchart rebuilds it from the release and chart names and cannot see either. Set db2.otlp.endpoint: http://%s.%s.svc.cluster.local:%s explicitly." $gatewayService $gatewayNs $gatewayPort) -}}
{{- end -}}
{{- /*
Checked unconditionally rather than only under namespaceOverride: the subchart falls back
to its own release namespace, so any collectorNamespace disagreeing with where the gateway
actually runs derives an address for a Service that is not there.
*/ -}}
{{- /* The subchart ships collectorNamespace: "", so dig returns that rather than a default. */ -}}
{{- $derivedNs := lower (dig "collectorNamespace" "" $otlp | default $db2Ns) -}}
{{- if ne $derivedNs $gatewayNs -}}
{{- fail (printf "db2.dbMonitoring.enabled would derive the gateway in namespace %q, but it runs in %q. Set db2.otlp.collectorNamespace: %s." $derivedNs $gatewayNs $gatewayNs) -}}
{{- end -}}
{{- /* The subchart always derives port 4317, so a re-pointed gateway needs an endpoint. */ -}}
{{- if ne $gatewayPort "4317" -}}
{{- fail (printf "db2.dbMonitoring.enabled cannot derive the gateway address while gateway.ports.otlp.containerPort is %s: the subchart always derives 4317. Set db2.otlp.endpoint: http://%s.%s.svc.cluster.local:%s explicitly." $gatewayPort $gatewayService $gatewayNs $gatewayPort) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}
