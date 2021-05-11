{{/*
Config for the otel-collector k8s cluster receiver deployment.
The values can be overridden in .Values.otelK8sClusterReceiver.config
*/}}
{{- define "splunk-otel-collector.otelK8sClusterReceiverConfig" -}}
extensions:
  health_check:

receivers:
  # Prometheus receiver scraping metrics from the pod itself, both otel and fluentd
  prometheus:
    config:
      scrape_configs:
      - job_name: 'otel-k8s-cluster-receiver'
        scrape_interval: 10s
        static_configs:
        - targets: ["${K8S_POD_IP}:8888"]
  k8s_cluster:
    auth_type: serviceAccount
    metadata_exporters: [signalfx]

processors:
  {{- include "splunk-otel-collector.otelMemoryLimiterConfig" .Values.otelK8sClusterReceiver | nindent 2 }}

  batch:

  # k8s_tagger to enrich its own metrics
  k8s_tagger:
    filter:
      node_from_env_var: K8S_NODE_NAME
      labels:
        key: component
        op: equals
        value: otel-k8s-cluster-receiver

  resource:
    attributes:
      # TODO: Remove once available in mapping service.
      - action: insert
        key: metric_source
        value: kubernetes
      # XXX: Added so that Smart Agent metrics and OTel metrics don't map to the same MTS identity
      # (same metric and dimension names and values) after mappings are applied. This would be
      # the case if somebody uses the same cluster name from Smart Agent and OTel in the same org.
      - action: insert
        key: receiver
        value: k8scluster
      - action: upsert
        key: k8s.cluster.name
        value: {{ .Values.clusterName }}
      {{- range .Values.extraAttributes.custom }}
      - action: upsert
        key: {{ .name }}
        value: {{ .value }}
      {{- end }}

exporters:
  signalfx:
    {{- if .Values.otelCollector.enabled }}
    ingest_url: http://{{ include "splunk-otel-collector.fullname" . }}:9943
    api_url: http://{{ include "splunk-otel-collector.fullname" . }}:6060
    {{- else }}
    ingest_url: {{ include "splunk-otel-collector.ingestUrl" . }}
    api_url: {{ include "splunk-otel-collector.apiUrl" . }}
    {{- end }}
    access_token: ${SPLUNK_ACCESS_TOKEN}
    timeout: 10s

service:
  extensions: [health_check]
  pipelines:
    # k8s metrics pipeline
    metrics:
      receivers: [prometheus, k8s_cluster]
      processors: [memory_limiter, batch, resource]
      exporters: [signalfx]
{{- end }}
