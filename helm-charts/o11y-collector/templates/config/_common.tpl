{{/*
Common config for the otel-collector memory_limiter processor
*/}}
{{- define "o11y-collector.otelMemoryLimiterConfig" -}}
memory_limiter:
  # check_interval is the time between measurements of memory usage.
  check_interval: 5s
  # By default limit_mib is set to 80% of container memory limit
  limit_mib: {{ include "o11y-collector.getOtelMemLimitMib" . | quote }}
  # By default spike_limit_mib is set to 25% of container memory limit
  spike_limit_mib: {{ include "o11y-collector.getOtelMemSpikeLimitMib" . | quote }}
  # By default ballast_size_mib is set to 40% of container memory limit
  ballast_size_mib: {{ include "o11y-collector.getOtelMemBallastSizeMib" . | quote }}
{{- end }}

{{/*
Common config for the otel-collector sapm exporter
*/}}
{{- define "o11y-collector.otelSapmExporter" -}}
sapm:
  endpoint: {{ include "o11y-collector.ingestUrl" . }}/v2/trace
  access_token: ${SPLUNK_ACCESS_TOKEN}
{{- end }}

{{/*
Common config for the otel-collector traces receivers
*/}}
{{- define "o11y-collector.otelTraceReceivers" -}}
otlp:
  protocols:
    grpc:
    http:
sapm:
  endpoint: 0.0.0.0:7276
jaeger:
  protocols:
    thrift_http:
      endpoint: 0.0.0.0:14268
    grpc:
      endpoint: 0.0.0.0:14250
zipkin:
  endpoint: 0.0.0.0:9411
opencensus:
  endpoint: 0.0.0.0:55678
{{- end }}
