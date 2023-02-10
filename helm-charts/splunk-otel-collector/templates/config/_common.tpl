{{/*
Common config for the otel-collector memory_limiter processor
*/}}
{{- define "splunk-otel-collector.otelMemoryLimiterConfig" -}}
memory_limiter:
  # check_interval is the time between measurements of memory usage.
  check_interval: 2s
  # By default limit_mib is set to 90% of container memory limit
  limit_mib: ${SPLUNK_MEMORY_LIMIT_MIB}
{{- end }}

{{/*
Common config for the otel-collector sapm exporter
*/}}
{{- define "splunk-otel-collector.otelSapmExporter" -}}
{{- if (eq (include "splunk-otel-collector.tracesEnabled" .) "true") }}
sapm:
  endpoint: {{ include "splunk-otel-collector.o11yIngestUrl" . }}/v2/trace
  access_token: ${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}
{{- end }}
{{- end }}

{{/*
Common config for the otel-collector traces receivers
*/}}
{{- define "splunk-otel-collector.otelReceivers" -}}
otlp:
  protocols:
    grpc:
      endpoint: 0.0.0.0:4317
    http:
      # Deprecated 55681 port is also open by default:
      # https://github.com/open-telemetry/opentelemetry-collector/blob/9d3a8a4608a7dbd9f787867226a78356ace9b5e4/receiver/otlpreceiver/otlp.go#L140-L152
      endpoint: 0.0.0.0:4318

{{- if (eq (include "splunk-otel-collector.tracesEnabled" .) "true") }}
jaeger:
  protocols:
    thrift_http:
      endpoint: 0.0.0.0:14268
    grpc:
      endpoint: 0.0.0.0:14250
zipkin:
  endpoint: 0.0.0.0:9411
{{- end }}
{{- end }}

{{/*
Filter Attributes Function
*/}}
{{- define "splunk-otel-collector.filterAttr" -}}
{{- if .Values.logsCollection.containers.useSplunkIncludeAnnotation -}}
splunk.com/include
{{- else -}}
splunk.com/exclude
{{- end }}
{{- end }}

{{/*
Common config for resourcedetection processor
*/}}
{{- define "splunk-otel-collector.resourceDetectionProcessor" -}}
resourcedetection:
  detectors:
    # Note: Kubernetes distro detectors need to come first so they set the proper cloud.platform
    # before it gets set later by the cloud provider detector.
    - env
    {{- if hasPrefix "gke" (include "splunk-otel-collector.distribution" .) }}
    - gke
    {{- else if hasPrefix "eks" (include "splunk-otel-collector.distribution" .) }}
    - eks
    {{- else if eq (include "splunk-otel-collector.distribution" .) "aks" }}
    - aks
    {{- end }}
    {{- if eq (include "splunk-otel-collector.cloudProvider" .) "gcp" }}
    - gce
    {{- else if eq (include "splunk-otel-collector.cloudProvider" .) "aws" }}
    - ec2
    {{- else if eq (include "splunk-otel-collector.cloudProvider" .) "azure" }}
    - azure
    {{- end }}
    # The `system` detector goes last so it can't preclude cloud detectors from setting host/os info.
    # https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor#ordering
    - system
  override: true
  timeout: 10s
{{- end }}

{{/*
Resource processor for logs manipulations
*/}}
{{- define "splunk-otel-collector.resourceLogsProcessor" -}}
resource/logs:
  attributes:
    {{- if .Values.splunkPlatform.sourcetype }}
    - key: com.splunk.sourcetype
      value: "{{.Values.splunkPlatform.sourcetype }}"
      action: upsert
    {{- end }}
    - key: com.splunk.sourcetype
      from_attribute: k8s.pod.annotations.splunk.com/sourcetype
      action: upsert
    - key: k8s.pod.annotations.splunk.com/sourcetype
      action: delete
    - key: {{ include "splunk-otel-collector.filterAttr" . }}
      action: delete
    {{- if .Values.autodetect.istio }}
    - key: service.name
      from_attribute: k8s.pod.labels.app
      action: insert
    - key: service.name
      from_attribute: istio_service_name
      action: insert
    - key: istio_service_name
      action: delete
    {{- end }}
    {{- if .Values.splunkPlatform.fieldNameConvention.renameFieldsSck }}
    - key: container_name
      from_attribute: k8s.container.name
      action: upsert
    - key: cluster_name
      from_attribute: k8s.cluster.name
      action: upsert
    - key: container_id
      from_attribute: container.id
      action: upsert
    - key: pod
      from_attribute: k8s.pod.name
      action: upsert
    - key: pod_uid
      from_attribute: k8s.pod.uid
      action: upsert
    - key: namespace
      from_attribute: k8s.namespace.name
      action: upsert
    - key: label_app
      from_attribute: k8s.pod.labels.app
      action: upsert
    {{- range $_, $label := .Values.extraAttributes.podLabels }}
    - key: {{ printf "label_%s" $label }}
      from_attribute: {{ printf "k8s.pod.labels.%s" $label }}
      action: upsert
    {{- end }}
    {{- if not .Values.splunkPlatform.fieldNameConvention.keepOtelConvention }}
    - key: k8s.container.name
      action: delete
    - key: k8s.cluster.name
      action: delete
    - key: container.id
      action: delete
    - key: k8s.pod.name
      action: delete
    - key: k8s.pod.uid
      action: delete
    - key: k8s.namespace.name
      action: delete
    - key: k8s.pod.labels.app
      action: delete
    {{- range $_, $label := .Values.extraAttributes.podLabels }}
    - key: {{ printf "k8s.pod.labels.%s" $label }}
      action: delete
    {{- end }}
    {{- end }}
    {{- end }}
  
transform/sck_fields:
  log_statements:
    - context: log
      statements:
        - set(resource.attributes["container_image"], Concat([resource.attributes["container.image.name"], resource.attributes["container.image.tag"]], ":"))
{{- end }}

{{/*
Filter logs processor
*/}}
{{- define "splunk-otel-collector.filterLogsProcessors" -}}
# Drop logs coming from pods and namespaces with splunk.com/exclude annotation.
filter/logs:
  logs:
    {{ .Values.logsCollection.containers.useSplunkIncludeAnnotation | ternary "include" "exclude" }}:
      match_type: strict
      resource_attributes:
        - key: {{ include "splunk-otel-collector.filterAttr" . }}
          value: "true"
{{- end }}

{{/*
Splunk Platform Logs exporter
*/}}
{{- define "splunk-otel-collector.splunkPlatformLogsExporter" -}}
splunk_hec/platform_logs:
  endpoint: {{ .Values.splunkPlatform.endpoint | quote }}
  token: "${SPLUNK_PLATFORM_HEC_TOKEN}"
  index: {{ .Values.splunkPlatform.index | quote }}
  source: {{ .Values.splunkPlatform.source | quote }}
  max_connections: {{ .Values.splunkPlatform.maxConnections }}
  disable_compression: {{ .Values.splunkPlatform.disableCompression }}
  timeout: {{ .Values.splunkPlatform.timeout }}
  splunk_app_name: {{ .Chart.Name }}
  splunk_app_version: {{ .Chart.Version }}
  profiling_data_enabled: false
  tls:
    insecure_skip_verify: {{ .Values.splunkPlatform.insecureSkipVerify }}
    {{- if .Values.splunkPlatform.clientCert }}
    cert_file: /otel/etc/splunk_platform_hec_client_cert
    {{- end }}
    {{- if .Values.splunkPlatform.clientKey  }}
    key_file: /otel/etc/splunk_platform_hec_client_key
    {{- end }}
    {{- if .Values.splunkPlatform.caFile }}
    ca_file: /otel/etc/splunk_platform_hec_ca_file
    {{- end }}
  retry_on_failure:
    enabled: {{ .Values.splunkPlatform.retryOnFailure.enabled }}
    initial_interval: {{ .Values.splunkPlatform.retryOnFailure.initialInterval }}
    max_interval: {{ .Values.splunkPlatform.retryOnFailure.maxInterval }}
    max_elapsed_time: {{ .Values.splunkPlatform.retryOnFailure.maxElapsedTime }}
  sending_queue:
    enabled:  {{ .Values.splunkPlatform.sendingQueue.enabled }}
    num_consumers: {{ .Values.splunkPlatform.sendingQueue.numConsumers }}
    queue_size: {{ .Values.splunkPlatform.sendingQueue.queueSize }}
{{- end }}

{{/*
Splunk Platform Metrics exporter
*/}}
{{- define "splunk-otel-collector.splunkPlatformMetricsExporter" -}}
splunk_hec/platform_metrics:
  endpoint: {{ .Values.splunkPlatform.endpoint | quote }}
  token: "${SPLUNK_PLATFORM_HEC_TOKEN}"
  index: {{ .Values.splunkPlatform.metricsIndex | quote }}
  source: {{ .Values.splunkPlatform.source | quote }}
  max_connections: {{ .Values.splunkPlatform.maxConnections }}
  disable_compression: {{ .Values.splunkPlatform.disableCompression }}
  timeout: {{ .Values.splunkPlatform.timeout }}
  splunk_app_name: {{ .Chart.Name }}
  splunk_app_version: {{ .Chart.Version }}
  tls:
    insecure_skip_verify: {{ .Values.splunkPlatform.insecureSkipVerify }}
    {{- if .Values.splunkPlatform.clientCert }}
    cert_file: /otel/etc/splunk_platform_hec_client_cert
    {{- end }}
    {{- if .Values.splunkPlatform.clientKey  }}
    key_file: /otel/etc/splunk_platform_hec_client_key
    {{- end }}
    {{- if .Values.splunkPlatform.caFile }}
    ca_file: /otel/etc/splunk_platform_hec_ca_file
    {{- end }}
  retry_on_failure:
    enabled: {{ .Values.splunkPlatform.retryOnFailure.enabled }}
    initial_interval: {{ .Values.splunkPlatform.retryOnFailure.initialInterval }}
    max_interval: {{ .Values.splunkPlatform.retryOnFailure.maxInterval }}
    max_elapsed_time: {{ .Values.splunkPlatform.retryOnFailure.maxElapsedTime }}
  sending_queue:
    enabled:  {{ .Values.splunkPlatform.sendingQueue.enabled }}
    num_consumers: {{ .Values.splunkPlatform.sendingQueue.numConsumers }}
    queue_size: {{ .Values.splunkPlatform.sendingQueue.queueSize }}
{{- end }}

{{/*
Splunk Platform Traces exporter
*/}}
{{- define "splunk-otel-collector.splunkPlatformTracesExporter" -}}
splunk_hec/platform_traces:
  endpoint: {{ .Values.splunkPlatform.endpoint | quote }}
  token: "${SPLUNK_PLATFORM_HEC_TOKEN}"
  index: {{ .Values.splunkPlatform.tracesIndex | quote }}
  source: {{ .Values.splunkPlatform.source | quote }}
  max_connections: {{ .Values.splunkPlatform.maxConnections }}
  disable_compression: {{ .Values.splunkPlatform.disableCompression }}
  timeout: {{ .Values.splunkPlatform.timeout }}
  splunk_app_name: {{ .Chart.Name }}
  splunk_app_version: {{ .Chart.Version }}
  tls:
    insecure_skip_verify: {{ .Values.splunkPlatform.insecureSkipVerify }}
    {{- if .Values.splunkPlatform.clientCert }}
    cert_file: /otel/etc/splunk_platform_hec_client_cert
    {{- end }}
    {{- if .Values.splunkPlatform.clientKey  }}
    key_file: /otel/etc/splunk_platform_hec_client_key
    {{- end }}
    {{- if .Values.splunkPlatform.caFile }}
    ca_file: /otel/etc/splunk_platform_hec_ca_file
    {{- end }}
  retry_on_failure:
    enabled: {{ .Values.splunkPlatform.retryOnFailure.enabled }}
    initial_interval: {{ .Values.splunkPlatform.retryOnFailure.initialInterval }}
    max_interval: {{ .Values.splunkPlatform.retryOnFailure.maxInterval }}
    max_elapsed_time: {{ .Values.splunkPlatform.retryOnFailure.maxElapsedTime }}
  sending_queue:
    enabled:  {{ .Values.splunkPlatform.sendingQueue.enabled }}
    num_consumers: {{ .Values.splunkPlatform.sendingQueue.numConsumers }}
    queue_size: {{ .Values.splunkPlatform.sendingQueue.queueSize }}
{{- end }}

{{/*
Add Extra Labels
*/}}
{{- define "splunk-otel-collector.addExtraLabels" -}}
{{- with .Values.extraAttributes.fromLabels }}
{{ . | toYaml}}
{{- end }}
{{- end }}

{{/*
Add Extra Annotations
*/}}
{{- define "splunk-otel-collector.addExtraAnnotations" -}}
{{- with .Values.extraAttributes.fromAnnotations }}
{{ . | toYaml}}
{{- end }}
{{- end }}
