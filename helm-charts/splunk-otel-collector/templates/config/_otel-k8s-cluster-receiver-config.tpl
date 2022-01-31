{{/*
Config for the otel-collector k8s cluster receiver deployment.
The values can be overridden in .Values.clusterReceiver.config
*/}}
{{- define "splunk-otel-collector.clusterReceiverConfig" -}}
{{ $gateway := fromYaml (include "splunk-otel-collector.gateway" .) -}}
{{ $clusterReceiver := fromYaml (include "splunk-otel-collector.clusterReceiver" .) -}}
extensions:
  health_check:

  memory_ballast:
    size_mib: ${SPLUNK_BALLAST_SIZE_MIB}

  {{- if eq (include "splunk-otel-collector.distribution" .) "eks/fargate" }}
  # k8s_observer w/ pod and node detection for eks/fargate deployment
  k8s_observer:
    auth_type: serviceAccount
    observe_pods: true
    observe_nodes: true
  {{- end }}

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
    {{- if eq (include "splunk-otel-collector.o11yMetricsEnabled" $) "true" }}
    metadata_exporters: [signalfx]
    {{- end }}
    {{- if eq (include "splunk-otel-collector.distribution" .) "openshift" }}
    distribution: openshift
    {{- end }}
  {{- if $clusterReceiver.k8sEventsEnabled }}
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
  {{- if eq (include "splunk-otel-collector.distribution" .) "eks/fargate" }}
  # dynamically created kubeletstats receiver to report all Fargate "node" kubelet stats
  # with exception of collector "node's" own since Fargate forbids connection.
  receiver_creator:
    receivers:
      kubeletstats:
        rule: type == "k8s.node" && name contains "fargate"
        config:
          auth_type: serviceAccount
          collection_interval: 10s
          endpoint: "`endpoint`:`kubelet_endpoint_port`"
          extra_metadata_labels:
            - container.id
          metric_groups:
            - container
            - pod
            - node
    watch_observers:
      - k8s_observer
  {{- end }}

processors:
  {{- include "splunk-otel-collector.otelMemoryLimiterConfig" . | nindent 2 }}

  batch:

  {{- include "splunk-otel-collector.resourceDetectionProcessor" . | nindent 2 }}

  {{- if and $clusterReceiver.k8sEventsEnabled (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
  resource/add_event_k8s:
    attributes:
      - action: insert
        key: kubernetes_cluster
        value: {{ .Values.clusterName }}
  {{- end }}

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
      - action: upsert
        key: k8s.cluster.name
        value: {{ .Values.clusterName }}
      {{- range .Values.extraAttributes.custom }}
      - action: upsert
        key: {{ .name }}
        value: {{ .value }}
      {{- end }}

  resource/k8s_cluster:
    attributes:
      # XXX: Added so that Smart Agent metrics and OTel metrics don't map to the same MTS identity
      # (same metric and dimension names and values) after mappings are applied. This would be
      # the case if somebody uses the same cluster name from Smart Agent and OTel in the same org.
      - action: insert
        key: receiver
        value: k8scluster

exporters:
  {{- if eq (include "splunk-otel-collector.o11yMetricsEnabled" $) "true" }}
  signalfx:
    {{- if $gateway.enabled }}
    ingest_url: http://{{ include "splunk-otel-collector.fullname" . }}:9943
    api_url: http://{{ include "splunk-otel-collector.fullname" . }}:6060
    {{- else }}
    ingest_url: {{ include "splunk-otel-collector.o11yIngestUrl" . }}
    api_url: {{ include "splunk-otel-collector.o11yApiUrl" . }}
    {{- end }}
    access_token: ${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}
    timeout: 10s
  {{- end }}

  {{- if and (eq (include "splunk-otel-collector.logsEnabled" $) "true") $clusterReceiver.k8sEventsEnabled }}
  splunk_hec/o11y:
    endpoint: {{ include "splunk-otel-collector.o11yIngestUrl" . }}/v1/log
    token: "${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}"
    sourcetype: kube:events
    source: kubelet
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformMetricsExporter" . | nindent 2 }}
  {{- end }}

service:
  telemetry:
    metrics:
      address: 0.0.0.0:8889
  {{- if eq (include "splunk-otel-collector.distribution" .) "eks/fargate" }}
  extensions: [health_check, memory_ballast, k8s_observer]
  {{- else }}
  extensions: [health_check, memory_ballast]
  {{- end }}
  pipelines:
    # k8s metrics pipeline
    metrics:
      receivers: [k8s_cluster]
      processors: [memory_limiter, batch, resource, resource/k8s_cluster]
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}

    {{- if eq (include "splunk-otel-collector.distribution" .) "eks/fargate" }}
    metrics/eks:
      receivers: [receiver_creator]
      processors: [memory_limiter, batch, resource]
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}
    {{- end }}

    {{- if or (eq (include "splunk-otel-collector.splunkO11yEnabled" $) "true") (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
    # Pipeline for metrics collected about the collector pod itself.
    metrics/collector:
      receivers: [prometheus/k8s_cluster_receiver]
      processors:
        - memory_limiter
        - batch
        - resource
        - resource/add_collector_k8s
        - resourcedetection
      exporters:
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}
    {{- end }}

    {{- if and $clusterReceiver.k8sEventsEnabled (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
    logs/events:
      receivers:
        - smartagent/kubernetes-events
      processors:
        - memory_limiter
        - batch
        - resource
        - resource/add_event_k8s
      exporters:
        - signalfx
        {{- if (eq (include "splunk-otel-collector.o11yLogsEnabled" .) "true") }}
        - splunk_hec/o11y
        {{- end }}
    {{- end }}
{{- end }}

{{/*
Pod anti-affinity to prevent eks/fargate replicas from being on same node
*/}}
{{- define "splunk-otel-collector.clusterReceiverPodAntiAffinity" -}}
podAntiAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchExpressions:
          - key: component
            operator: In
            values:
              - otel-k8s-cluster-receiver
      topologyKey: "kubernetes.io/hostname"
{{- end }}
