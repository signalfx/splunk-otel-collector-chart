{{/*
Create the OpAMP Bridge resource name.
*/}}
{{- define "splunk-otel-collector.opampBridgeName" -}}
{{- if .Values.remoteManagement.opampBridge.name -}}
{{- .Values.remoteManagement.opampBridge.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-opamp-bridge" (include "splunk-otel-collector.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Create the OpAMP Bridge configmap name.
*/}}
{{- define "splunk-otel-collector.opampBridgeConfigMapName" -}}
{{- printf "%s-config" (include "splunk-otel-collector.opampBridgeName" . | trunc 56 | trimSuffix "-") | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create the OpAMP Bridge service account name.
*/}}
{{- define "splunk-otel-collector.opampBridgeServiceAccountName" -}}
{{- default (include "splunk-otel-collector.opampBridgeName" .) .Values.remoteManagement.opampBridge.serviceAccount.name -}}
{{- end -}}

{{/*
Whether collector workloads should open direct OpAMP sessions.
*/}}
{{- define "splunk-otel-collector.directO11yOpampEnabled" -}}
{{- and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") (not .Values.remoteManagement.enabled) -}}
{{- end -}}

{{/*
Remote management marker for collector ConfigMaps whose relay config is managed
by OpAMP Bridge after Helm creates the bootstrap config.
*/}}
{{- define "splunk-otel-collector.remoteManagementConfigAnnotation" -}}
splunk.com/remote-management-config
{{- end -}}

{{/*
Render collector ConfigMap relay config.

When remote management manages this workload, preserve live data.relay on
upgrades only after the live ConfigMap is marked with the remote management
annotation. This lets existing installs render the new Helm bootstrap config
when remote management is enabled for the first time, then preserves OpAMP
remote config on later upgrades.
*/}}
{{- define "splunk-otel-collector.collectorConfigMapRelay" -}}
{{- $root := .root -}}
{{- $configMapName := .configMapName -}}
{{- $chartConfig := .chartConfig -}}
{{- $managed := .managed -}}
{{- $annotation := include "splunk-otel-collector.remoteManagementConfigAnnotation" $root -}}
{{- $upgradeStrategy := dig "collectorConfig" "upgradeStrategy" "preserve" $root.Values.remoteManagement -}}
{{- if and $managed (ne $upgradeStrategy "resetFromHelm") -}}
{{- $namespace := include "splunk-otel-collector.namespace" $root -}}
{{- $live := lookup "v1" "ConfigMap" $namespace $configMapName -}}
{{- $alreadyManaged := false -}}
{{- if and $live $live.metadata $live.metadata.annotations (eq (index $live.metadata.annotations $annotation) "true") -}}
{{- $alreadyManaged = true -}}
{{- end -}}
{{- if and $alreadyManaged $live.data (hasKey $live.data "relay") -}}
{{- index $live.data "relay" -}}
{{- else -}}
{{- $chartConfig -}}
{{- end -}}
{{- else -}}
{{- $chartConfig -}}
{{- end -}}
{{- end -}}

{{/*
Get OpAMP Bridge endpoint.
*/}}
{{- define "splunk-otel-collector.opampBridgeEndpoint" -}}
{{- if .Values.remoteManagement.opampBridge.endpoint -}}
{{- .Values.remoteManagement.opampBridge.endpoint -}}
{{- else if eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true" -}}
{{- printf "%s/v1/opamp" (include "splunk-otel-collector.o11yIngestUrl" .) -}}
{{- else -}}
{{- fail "/remoteManagement/opampBridge/endpoint is required when remoteManagement.enabled is true and splunkObservability.realm is not set" -}}
{{- end -}}
{{- end -}}

{{/*
Whether the chart-created agent workload should be managed by the OpAMP Bridge.
*/}}
{{- define "splunk-otel-collector.opampBridge.agentManaged" -}}
{{- $bridge := .Values.remoteManagement.opampBridge -}}
{{- $agentEnabled := and .Values.agent.enabled (ne .Values.distribution "eks/fargate") -}}
{{- if and .Values.remoteManagement.enabled $agentEnabled $bridge.workloads.agent.enabled -}}true{{- end -}}
{{- end -}}

{{/*
Whether the chart-created gateway workload should be managed by the OpAMP Bridge.
*/}}
{{- define "splunk-otel-collector.opampBridge.gatewayManaged" -}}
{{- $bridge := .Values.remoteManagement.opampBridge -}}
{{- if and .Values.remoteManagement.enabled .Values.gateway.enabled $bridge.workloads.gateway.enabled -}}true{{- end -}}
{{- end -}}

{{/*
Whether the chart-created cluster receiver workload should be managed by the OpAMP Bridge.
*/}}
{{- define "splunk-otel-collector.opampBridge.clusterReceiverManaged" -}}
{{- $bridge := .Values.remoteManagement.opampBridge -}}
{{- $clusterReceiverEnabled := eq (include "splunk-otel-collector.clusterReceiverEnabled" .) "true" -}}
{{- if and .Values.remoteManagement.enabled $clusterReceiverEnabled $bridge.workloads.clusterReceiver.enabled -}}true{{- end -}}
{{- end -}}

{{/*
Whether the OpAMP Bridge has at least one managed workload.
*/}}
{{- define "splunk-otel-collector.opampBridge.hasManagedWorkload" -}}
{{- $bridge := .Values.remoteManagement.opampBridge -}}
{{- $agentManaged := eq (include "splunk-otel-collector.opampBridge.agentManaged" .) "true" -}}
{{- $gatewayManaged := eq (include "splunk-otel-collector.opampBridge.gatewayManaged" .) "true" -}}
{{- $clusterReceiverManaged := eq (include "splunk-otel-collector.opampBridge.clusterReceiverManaged" .) "true" -}}
{{- if or $agentManaged $gatewayManaged $clusterReceiverManaged (gt (len $bridge.extraAgents) 0) -}}true{{- end -}}
{{- end -}}

{{/*
Require at least one managed workload when the OpAMP Bridge is enabled.
*/}}
{{- define "splunk-otel-collector.opampBridge.requireManagedWorkload" -}}
{{- if .Values.remoteManagement.enabled -}}
{{- if ne (include "splunk-otel-collector.opampBridge.hasManagedWorkload" .) "true" -}}
{{- fail "/remoteManagement requires at least one managed collector workload: enable agent, gateway, clusterReceiver, or set remoteManagement.opampBridge.extraAgents" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Render standalone OpAMP Bridge non-identifying attributes.
*/}}
{{- define "splunk-otel-collector.opampBridge.descriptionAttributes" -}}
{{- $attrs := deepCopy .Values.remoteManagement.opampBridge.description.non_identifying_attributes -}}
{{- with .Values.clusterName -}}
{{- $_ := set $attrs "k8s.cluster.name" . -}}
{{- end -}}
{{- with .Values.environment -}}
{{- $_ := set $attrs "deployment.environment.name" . -}}
{{- end -}}
{{- if $attrs -}}
{{- toYaml $attrs -}}
{{- end -}}
{{- end -}}

{{/*
Render standalone OpAMP Bridge non-identifying attributes for a chart-managed workload.
*/}}
{{- define "splunk-otel-collector.opampBridgeWorkloadAttributes" -}}
{{- $attrs := deepCopy (dig "description" "non_identifying_attributes" (dict) .workload) -}}
{{- $_ := set $attrs "service.version" (.root.Values.image.otelcol.tag | default .root.Chart.AppVersion) -}}
{{- $_ := set $attrs "otelcol.service.mode" .workload.serviceMode -}}
{{- with .root.Values.clusterName -}}
{{- $_ := set $attrs "k8s.cluster.name" . -}}
{{- end -}}
{{- with .root.Values.environment -}}
{{- $_ := set $attrs "deployment.environment.name" . -}}
{{- end -}}
{{- toYaml $attrs -}}
{{- end -}}
