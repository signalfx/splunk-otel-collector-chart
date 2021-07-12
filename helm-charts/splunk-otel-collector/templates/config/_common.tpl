{{/*
Common config for the otel-collector memory_limiter processor
*/}}
{{- define "splunk-otel-collector.otelMemoryLimiterConfig" -}}
memory_limiter:
  # check_interval is the time between measurements of memory usage.
  check_interval: 5s
  # By default limit_mib is set to 80% of container memory limit
  limit_mib: ${SPLUNK_MEMORY_LIMIT_MIB}
  # Agent will set this value.
  ballast_size_mib: ${SPLUNK_BALLAST_SIZE_MIB}
{{- end }}

{{/*
Common config for the otel-collector sapm exporter
*/}}
{{- define "splunk-otel-collector.otelSapmExporter" -}}
{{- if .Values.tracesEnabled }}
sapm:
  endpoint: {{ include "splunk-otel-collector.ingestUrl" . }}/v2/trace
  access_token: ${SPLUNK_ACCESS_TOKEN}
{{- end }}
{{- end }}

{{/*
Common config for the otel-collector traces receivers
*/}}
{{- define "splunk-otel-collector.otelTraceReceivers" -}}
otlp:
  protocols:
    grpc:
      endpoint: 0.0.0.0:4317
    http:
      endpoint: 0.0.0.0:55681

{{- if .Values.tracesEnabled }}
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
    - system
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
  # Don't override existing resource attributes to maintain identification of data sources
  override: false
  timeout: 10s
{{- end }}
