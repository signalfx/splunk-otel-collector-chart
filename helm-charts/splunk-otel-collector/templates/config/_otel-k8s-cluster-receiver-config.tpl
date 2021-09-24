{{/*
Config for the otel-collector k8s cluster receiver deployment.
The values can be overridden in .Values.otelK8sClusterReceiver.config
*/}}
{{- define "splunk-otel-collector.otelK8sClusterReceiverConfig" -}}
extensions:
  health_check:

receivers:
  # Prometheus receiver scraping metrics from the pod itself, both otel and fluentd
  prometheus/k8s_cluster_receiver:
    config:
      scrape_configs:
      - job_name: 'otel-k8s-cluster-receiver'
        scrape_interval: 10s
        static_configs:
        - targets: ["${K8S_POD_IP}:8889"]
  k8s_cluster:
    auth_type: serviceAccount
    metadata_exporters: [signalfx]
    {{- if eq .Values.distro "openshift" }}
    distribution: openshift
    {{- end }}
  {{- if .Values.otelK8sClusterReceiver.k8sEventsEnabled }}
  smartagent/kubernetes-events:
    type: kubernetes-events
    alwaysClusterReporter: true
    whitelistedEvents:
    - reason: Created
      involvedObjectKind: Pod
    - reason: Unhealthy
      involvedObjectKind: Pod
    - reason: Failed
      involvedObjectKind: Pod
    - reason: FailedCreate
      involvedObjectKind: Job
  {{- end }}

processors:
  {{- include "splunk-otel-collector.otelMemoryLimiterConfig" .Values.otelK8sClusterReceiver | nindent 2 }}

  batch:

  {{- include "splunk-otel-collector.resourceDetectionProcessor" . | nindent 2 }}

  # Resource attributes specific to the collector itself.
  resource/add_collector_k8s:
    attributes:
      - action: insert
        key: k8s.node.name
        value: "${K8S_NODE_NAME}"
      - action: insert
        key: k8s.pod.name
        value: "${K8S_POD_NAME}"
      - action: insert
        key: k8s.pod.uid
        value: "${K8S_POD_UID}"
      - action: insert
        key: k8s.namespace.name
        value: "${K8S_NAMESPACE}"

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
      receivers: [k8s_cluster]
      processors: [memory_limiter, batch, resource]
      exporters: [signalfx]

    # Pipeline for metrics collected about the collector pod itself.
    metrics/collector:
      receivers: [prometheus/k8s_cluster_receiver]
      processors:
        - memory_limiter
        - batch
        - resource
        - resource/add_collector_k8s
        - resourcedetection
      exporters: [signalfx]
    {{- if .Values.otelK8sClusterReceiver.k8sEventsEnabled }}
    logs/events:
      receivers:
        - smartagent/kubernetes-events
      processors:
        - memory_limiter
        - batch
        - resource
      exporters: [signalfx]
    {{- end }}
{{- end }}
