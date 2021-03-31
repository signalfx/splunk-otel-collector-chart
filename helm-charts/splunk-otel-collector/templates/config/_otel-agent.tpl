{{/*
Config for the otel-collector agent
The values can be overridden in .Values.otelAgent.config
*/}}
{{- define "splunk-otel-collector.otelAgentConfig" -}}
extensions:
  health_check:

  k8s_observer:
    auth_type: serviceAccount
    node: ${K8S_NODE_NAME}

  zpages:

receivers:
  {{- include "splunk-otel-collector.otelTraceReceivers" . | nindent 2 }}
  # Prometheus receiver scraping metrics from the pod itself
  prometheus:
    config:
      scrape_configs:
      - job_name: 'otel-agent'
        scrape_interval: 10s
        static_configs:
        - targets: ["${K8S_POD_IP}:8888", "${K8S_POD_IP}:24231"]

  hostmetrics:
    collection_interval: 10s
    scrapers:
      cpu:
      disk:
      filesystem:
      memory:
      network:
      # System load average metrics https://en.wikipedia.org/wiki/Load_(computing)
      load:
      # Paging/Swap space utilization and I/O metrics
      paging:
      # Aggregated system process count metrics
      processes:
      # System processes metrics, disabled by default
      # process:

  receiver_creator:
    watch_observers: [k8s_observer]
    receivers:
      prometheus_simple:
        # Enable prometheus scraping for pods with standard prometheus annotations
        rule: type == "pod" && annotations["prometheus.io/scrape"] == "true"
        config:
          metrics_path: '`"prometheus.io/path" in annotations ? annotations["prometheus.io/path"] : "/metrics"`'
          endpoint: '`endpoint`:`"prometheus.io/port" in annotations ? annotations["prometheus.io/port"] : 9090`'

  kubeletstats:
    collection_interval: 10s
    auth_type: serviceAccount
    endpoint: ${K8S_NODE_IP}:10250
    extra_metadata_labels:
      - container.id

# By default k8s_tagger and batch processors enabled.
processors:
  # k8s_tagger enriches traces and metrics with k8s metadata
  k8s_tagger:
    # If standalone collector deployment is enabled, the `passthrough` configuration is enabled by default.
    # It means that traces and metrics enrichment happens in collector, and the agent only passes information
    # about traces and metrics source, without calling k8s API.
    {{- if .Values.otelCollector.enabled }}
    passthrough: true
    {{- end }}
    filter:
      node_from_env_var: K8S_NODE_NAME
    extract:
      metadata:
        - namespace
        - node
        - podName
        - podUID
      {{- with .Values.extraAttributes.podLabels }}
      labels:
        {{- range . }}
        - key: {{ . }}
        {{- end }}
      {{- end }}

  # Resource detection processor picks attributes from host environment.
  # https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor
  resourcedetection:
    detectors:
      - env
      {{- if eq .Values.platform "gcp" }}
      - gke
      - gce
      {{- else if eq .Values.platform "aws" }}
      - eks
      - ec2
      {{- end }}
    timeout: 10s

  {{- include "splunk-otel-collector.otelMemoryLimiterConfig" .Values.otelAgent | nindent 2 }}

  batch:
    timeout: 200ms
    send_batch_size: 128

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

  {{- if .Values.environment }}
  resource/add_environment:
    attributes:
      - action: insert
        value: {{ .Values.environment }}
        key: deployment.environment
  {{- end }}

# By default only SAPM exporter enabled. It will be pointed to collector deployment if enabled,
# Otherwise it's pointed directly to signalfx backend based on the values provided in signalfx setting.
# These values should not be specified manually and will be set in the templates.
exporters:

  {{- if .Values.otelCollector.enabled }}
  # If collector is enabled, metrics and traces will be sent to collector
  otlp:
    endpoint: {{ include "splunk-otel-collector.fullname" . }}:55680
    insecure: true
  {{- else }}
  # If collector is disabled, metrics and traces will be set to to SignalFx backend
  {{- include "splunk-otel-collector.otelSapmExporter" . | nindent 2 }}
  {{- end }}
  signalfx:
    correlation:
    ingest_url: {{ include "splunk-otel-collector.ingestUrl" . }}
    api_url: {{ include "splunk-otel-collector.apiUrl" . }}
    access_token: ${SPLUNK_ACCESS_TOKEN}
    sync_host_metadata: true

service:
  extensions: [health_check, k8s_observer, zpages]

  # By default there are two pipelines sending metrics and traces to standalone otel-collector otlp format
  # or directly to signalfx backend depending on otelCollector.enabled configuration.
  # The default pipelines should to be changed. You can add any custom pipeline instead.
  # In order to disable a default pipeline just set it to `null` in otelAgent.config overrides.
  pipelines:

    # default traces pipeline
    traces:
      receivers: [otlp, jaeger, zipkin, opencensus]
      processors:
        - memory_limiter
        - resourcedetection
        - k8s_tagger
        - resource/add_cluster_name
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
        - batch
      exporters:
        {{- if .Values.otelCollector.enabled }}
        - otlp
        {{- else }}
        - sapm
        {{- end }}
        - signalfx

    # default metrics pipeline
    metrics:
      receivers: [hostmetrics, prometheus, kubeletstats, receiver_creator]
      processors: [memory_limiter, resource/add_cluster_name, resourcedetection]
      exporters:
        {{- if .Values.otelCollector.enabled }}
        - otlp
        {{- else }}
        - signalfx
        {{- end }}
{{- end }}
