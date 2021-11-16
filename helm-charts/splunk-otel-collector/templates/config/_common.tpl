{{/*
Common config for the otel-collector memory_limiter processor
*/}}
{{- define "splunk-otel-collector.otelMemoryLimiterConfig" -}}
memory_limiter:
  # check_interval is the time between measurements of memory usage.
  check_interval: 2s
  # By default limit_mib is set to 80% of container memory limit
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
Common config for resourcedetection processor
*/}}
{{- define "splunk-otel-collector.resourceDetectionProcessor" -}}
# Resource detection processor picks attributes from host environment.
# https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor
resourcedetection:
  detectors:
    # Note: Kubernetes distro detectors need to come first so they set the proper cloud.platform
    # before it gets set later by the cloud provider detector.
    - env
    {{- if eq .Values.distro "gke" }}
    - gke
    {{- else if eq .Values.distro "eks" }}
    - eks
    {{- else if eq .Values.distro "aks" }}
    - aks
    {{- end }}
    {{- if eq .Values.provider "gcp" }}
    - gce
    {{- else if eq .Values.provider "aws" }}
    - ec2
    {{- else if eq .Values.provider "azure" }}
    - azure
    {{- end }}
    # The `system` detector goes last so it can't preclude cloud detectors from setting host/os info.
    # https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor#ordering
    - system
  # Don't override existing resource attributes to maintain identification of data sources
  override: false
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
    - key: splunk.com/exclude
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
    {{- range $_, $label := .Values.extraAttributes.podLabels }}
    - key: {{ printf "k8s.pod.labels.%s" $label }}
      action: delete
    {{- end }}
    {{- end }}
    {{- end }}
{{- end }}

{{/*
Filter logs processor
*/}}
{{- define "splunk-otel-collector.filterLogsProcessors" -}}
# Drop logs coming from pods and namespaces with splunk.com/exclude annotation.
filter/logs:
  logs:
    exclude:
      resource_attributes:
        - key: splunk.com/exclude
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
  max_connections: {{ .Values.splunkPlatform.max_connections }}
  disable_compression: {{ .Values.splunkPlatform.disable_compression }}
  timeout: {{ .Values.splunkPlatform.timeout }}
  splunk_app_name: {{ .Chart.Name }}
  splunk_app_version: {{ .Chart.Version }}
  tls:
    insecure_skip_verify: {{ .Values.splunkPlatform.insecure_skip_verify }}
    {{- if .Values.splunkPlatform.clientCert }}
    cert_file: /otel/etc/splunk_platform_hec_client_cert
    {{- end }}
    {{- if .Values.splunkPlatform.clientKey  }}
    key_file: /otel/etc/splunk_platform_hec_client_key
    {{- end }}
    {{- if .Values.splunkPlatform.caFile }}
    ca_file: /otel/etc/splunk_platform_hec_ca_file
    {{- end }}
{{- end }}

{{/*
Splunk Platform Logs exporter
*/}}
{{- define "splunk-otel-collector.splunkPlatformMetricsExporter" -}}
splunk_hec/platform_metrics:
  endpoint: {{ .Values.splunkPlatform.endpoint | quote }}
  token: "${SPLUNK_PLATFORM_HEC_TOKEN}"
  index: {{ .Values.splunkPlatform.metrics_index | quote }}
  source: {{ .Values.splunkPlatform.source | quote }}
  max_connections: {{ .Values.splunkPlatform.max_connections }}
  disable_compression: {{ .Values.splunkPlatform.disable_compression }}
  timeout: {{ .Values.splunkPlatform.timeout }}
  splunk_app_name: {{ .Chart.Name }}
  splunk_app_version: {{ .Chart.Version }}
  tls:
    insecure_skip_verify: {{ .Values.splunkPlatform.insecure_skip_verify }}
    {{- if .Values.splunkPlatform.clientCert }}
    cert_file: /otel/etc/splunk_platform_hec_client_cert
    {{- end }}
    {{- if .Values.splunkPlatform.clientKey  }}
    key_file: /otel/etc/splunk_platform_hec_client_key
    {{- end }}
    {{- if .Values.splunkPlatform.caFile }}
    ca_file: /otel/etc/splunk_platform_hec_ca_file
    {{- end }}
{{- end }}
