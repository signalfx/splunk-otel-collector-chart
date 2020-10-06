{{/*
Config for the otel-collector k8s cluster receiver deployment.
The values can be overridden in .Values.otelK8sClusterReceiver.config
*/}}
{{- define "o11y-collector.otelK8sClusterReceiverConfig" -}}
extensions:
  health_check: {}

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
  queued_retry: {}

  {{- include "o11y-collector.otelMemoryLimiterConfig" .Values.otelK8sClusterReceiver | nindent 2 }}

  # k8s_tagger to enrich its own metrics
  k8s_tagger:
    filter:
      node_from_env_var: K8S_NODE_NAME
      labels:
        key: component
        op: equals
        value: otel-k8s-cluster-receiver

  resource/add_cluster_name:
    attributes:
      - action: upsert
        value: {{ .Values.clusterName }}
        key: k8s.cluster.name
      {{- range .Values.extraAttributes.custom }}
      - action: upsert
        value: {{ .value }}
        key: {{ .name }}
      {{- end }}

exporters:
  signalfx:
    ingest_url: {{ include "o11y-collector.ingestUrl" . }}/v2/datapoint
    api_url: {{ include "o11y-collector.apiUrl" . }}
    access_token: ${SPLUNK_ACCESS_TOKEN}
    send_compatible_metrics: true
    timeout: 10s

service:
  extensions: [health_check]
  pipelines:
    # k8s metrics pipeline
    metrics:
      receivers: [prometheus, k8s_cluster]
      processors: [memory_limiter, k8s_tagger, resource/add_cluster_name, queued_retry]
      exporters: [signalfx]
{{- end }}
