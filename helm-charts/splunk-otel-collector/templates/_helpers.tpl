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
{{- include "splunk-otel-collector.platformLogsEnabled" . }}
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
{{- if not (hasKey .securityContext "windowsOptions")}}
{{- $_ := set .securityContext "windowsOptions" dict }}
{{- end }}
{{- if and (not (hasKey .securityContext.windowsOptions "runAsUserName")) (.setRunAsUser) }}
{{- $_ := set .securityContext.windowsOptions "runAsUserName" "ContainerAdministrator"}}
{{- end }}
{{- else }}
{{- if and (eq (toString .securityContext.runAsUser) "<nil>") (.setRunAsUser) }}
{{- $_ := set .securityContext "runAsUser" 0 }}
{{- end }}
{{- end }}
{{- toYaml .securityContext }}
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
Helper used to define a namspace.
- Returns namespace from a release
- If namespaceOverride value is filled in it will replace the namespace
*/}}
{{- define "splunk-otel-collector.namespace" -}}
  {{- default .Release.Namespace .Values.namespaceOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create the name of the target allocator service account to use
*/}}
{{- define "splunk-otel-collector.targetAllocatorServiceAccountName" -}}
{{- default (printf "%s-ta" ( include "splunk-otel-collector.fullname" .) | trunc 63 | trimSuffix "-") .Values.targetAllocator.serviceAccount.name -}}
{{- end -}}

{{/*
Create the name of the target allocator cluster role to use
*/}}
{{- define "splunk-otel-collector.targetAllocatorClusterRoleName" -}}
{{- printf "%s-ta-clusterRole" ( include "splunk-otel-collector.fullname" . ) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create the name of the target allocator cluster config map to use
*/}}
{{- define "splunk-otel-collector.targetAllocatorConfigMapName" -}}
{{- printf "%s-ta-configmap" ( include "splunk-otel-collector.fullname" . ) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create the name of the target allocator cluster role binding to use
*/}}
{{- define "splunk-otel-collector.targetAllocatorClusterRoleBindingName" -}}
{{- printf "%s-ta-clusterRoleBinding" ( include "splunk-otel-collector.fullname" . ) | trunc 63 | trimSuffix "-" -}}
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
Whether logsCollection.containers.sourcetype is set and should drive a
transform/sourcetype processor in the logs pipeline. The user-supplied value is
a raw OTTL value expression and is wrapped by the chart in
  set(resource.attributes["com.splunk.sourcetype"], <value>)
inside a transform processor inserted after k8s_attributes.
*/}}
{{- define "splunk-otel-collector.containerSourcetypeEnabled" -}}
{{- if (default "" .Values.logsCollection.containers.sourcetype) -}}true{{- else -}}false{{- end -}}
{{- end -}}

{{/*
Whether multi-tenant logs routing is enabled. True only when
splunkPlatform.logsRouting.enabled is true AND logs are enabled to the
Splunk Platform destination.
*/}}
{{- define "splunk-otel-collector.multiTenantLogsEnabled" -}}
{{- and (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") (default false .Values.splunkPlatform.logsRouting.enabled) -}}
{{- end -}}

{{/*
Translate a tenant name like "team-a" into the name of the environment variable
that holds its HEC token, e.g. "SPLUNK_PLATFORM_HEC_TOKEN_TEAM_A".
*/}}
{{- define "splunk-otel-collector.tenantEnvVarName" -}}
{{- printf "SPLUNK_PLATFORM_HEC_TOKEN_%s" (upper (replace "-" "_" .)) -}}
{{- end -}}

{{/*
Translate a tenant name into a Kubernetes Secret data key for inline tokens
that the chart stores in its managed secret, e.g. "hec_token_team_a".
*/}}
{{- define "splunk-otel-collector.tenantInlineSecretKey" -}}
{{- printf "hec_token_%s" (lower (replace "-" "_" .)) -}}
{{- end -}}

{{/*
Directory name (not full path) for a tenant's isolated persistent-queue
file_storage subdirectory.
*/}}
{{- define "splunk-otel-collector.tenantPersistentQueueSubdir" -}}
{{- printf "exporter_queue_%s" (replace "-" "_" .) -}}
{{- end -}}

{{/*
Validate splunkPlatform.additionalLogsExporters and splunkPlatform.logsRouting.
Fails helm-template invocation with a clear message on any of the following:
  - Duplicate tenant name.
  - HEC tenant that does not provide exactly one of token/tokenSecret.
  - OTLP tenant that mis-uses HEC-only fields.
  - logsRouting.enabled without a fromAttribute.
  - logsRouting.table entry that references an unknown exporter.
Usage: {{- include "splunk-otel-collector.validateTenants" . -}}
*/}}
{{- define "splunk-otel-collector.validateTenants" -}}
{{- $tenants := default (list) .Values.splunkPlatform.additionalLogsExporters -}}
{{- $routing := default (dict) .Values.splunkPlatform.logsRouting -}}
{{- $seen := dict -}}
{{- $validNames := list "default" -}}
{{- range $i, $t := $tenants -}}
  {{- if not (hasKey $t "name") -}}
    {{- fail (printf "splunkPlatform.additionalLogsExporters[%d]: 'name' is required" $i) -}}
  {{- end -}}
  {{- $name := $t.name -}}
  {{- if hasKey $seen $name -}}
    {{- fail (printf "splunkPlatform.additionalLogsExporters: duplicate tenant name %q" $name) -}}
  {{- end -}}
  {{- $seen = merge $seen (dict $name true) -}}
  {{- $validNames = append $validNames $name -}}
  {{- $protocol := default "hec" $t.protocol -}}
  {{- if not (has $protocol (list "hec" "otlp_grpc" "otlp_http")) -}}
    {{- fail (printf "splunkPlatform.additionalLogsExporters[%q].protocol must be one of hec, otlp_grpc, otlp_http; got %q" $name $protocol) -}}
  {{- end -}}
  {{- if eq $protocol "hec" -}}
    {{- $tokenModes := 0 -}}
    {{- if and (hasKey $t "token") (ne (toString $t.token) "") -}}{{- $tokenModes = add1 $tokenModes -}}{{- end -}}
    {{- if hasKey $t "tokenSecret" -}}
      {{- $ts := $t.tokenSecret -}}
      {{- if and $ts (hasKey $ts "name") (ne (toString $ts.name) "") -}}{{- $tokenModes = add1 $tokenModes -}}{{- end -}}
    {{- end -}}
    {{- if ne $tokenModes 1 -}}
      {{- fail (printf "splunkPlatform.additionalLogsExporters[%q]: HEC tenants must set exactly one of token or tokenSecret.name+key (got %d)" $name $tokenModes) -}}
    {{- end -}}
  {{- else -}}
    {{- range $field := list "token" "tokenSecret" "index" "sourcetype" "source" -}}
      {{- if hasKey $t $field -}}
        {{- $v := index $t $field -}}
        {{- if and $v (ne (toString $v) "") (ne (toString $v) "map[]") -}}
          {{- fail (printf "splunkPlatform.additionalLogsExporters[%q]: field %q is HEC-only and must not be set for protocol %q" $name $field $protocol) -}}
        {{- end -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- if default false $routing.enabled -}}
  {{- if eq (toString (default "" $routing.fromAttribute)) "" -}}
    {{- fail "splunkPlatform.logsRouting.enabled=true requires splunkPlatform.logsRouting.fromAttribute to be set" -}}
  {{- end -}}
  {{- $defaultExp := default "default" $routing.defaultExporter -}}
  {{- if not (has $defaultExp $validNames) -}}
    {{- fail (printf "splunkPlatform.logsRouting.defaultExporter=%q does not resolve to 'default' or an entry in splunkPlatform.additionalLogsExporters" $defaultExp) -}}
  {{- end -}}
  {{- $tableValues := dict -}}
  {{- range $i, $entry := default (list) $routing.table -}}
    {{- if not (has $entry.exporter $validNames) -}}
      {{- fail (printf "splunkPlatform.logsRouting.table[%d].exporter=%q does not resolve to 'default' or an entry in splunkPlatform.additionalLogsExporters" $i $entry.exporter) -}}
    {{- end -}}
    {{- if hasKey $tableValues $entry.value -}}
      {{- fail (printf "splunkPlatform.logsRouting.table[%d]: duplicate value %q" $i $entry.value) -}}
    {{- end -}}
    {{- $tableValues = merge $tableValues (dict $entry.value true) -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{/*
Resolve a single tenant's value for a field, falling back to the supplied
default when the tenant value is missing or zero. Returns the resolved value.
Args: (dict "tenant" $t "field" "<key>" "default" <defaultValue>)
*/}}
{{- define "splunk-otel-collector.tenantField" -}}
{{- $t := .tenant -}}
{{- $field := .field -}}
{{- $default := .default -}}
{{- if hasKey $t $field -}}
  {{- $v := index $t $field -}}
  {{- if eq (kindOf $v) "string" -}}
    {{- if ne $v "" -}}{{- $v -}}{{- else -}}{{- $default -}}{{- end -}}
  {{- else if eq (kindOf $v) "int" -}}
    {{- if ne (int $v) 0 -}}{{- $v -}}{{- else -}}{{- $default -}}{{- end -}}
  {{- else if eq (kindOf $v) "float64" -}}
    {{- if ne (float64 $v) 0.0 -}}{{- $v -}}{{- else -}}{{- $default -}}{{- end -}}
  {{- else if eq (kindOf $v) "bool" -}}
    {{- $v -}}
  {{- else if eq (kindOf $v) "invalid" -}}
    {{- $default -}}
  {{- else -}}
    {{- $v -}}
  {{- end -}}
{{- else -}}
{{- $default -}}
{{- end -}}
{{- end -}}

{{/*
Render the "token" field value for a HEC tenant in the exporter config.
References an env var (${env:<NAME>}) on the collector container so that
secrets are never embedded in the rendered config map. The env var is
populated by the chart from either the inline token (chart-managed Secret)
or the user-supplied tokenSecret reference.
*/}}
{{- define "splunk-otel-collector.tenantTokenRef" -}}
{{- printf "${env:%s}" (include "splunk-otel-collector.tenantEnvVarName" .name) -}}
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
{{- $depProcessors := dict "k8sattributes" "k8s_attributes" -}}
{{- $depReceivers := dict "filelog" "file_log" "hostmetrics" "host_metrics" "k8sobjects" "k8s_objects" -}}
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
