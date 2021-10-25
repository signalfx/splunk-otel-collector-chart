{{/*
Config for the otel-collector agent
The values can be overridden in .Values.otelAgent.config
*/}}
{{- define "splunk-otel-collector.otelAgentConfig" -}}
extensions:
  {{- if and (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq .Values.logsEngine "otel") }}
  file_storage:
    directory: {{ .Values.logsCollection.checkpointPath }}
  {{- end }}

  memory_ballast:
    size_mib: ${SPLUNK_BALLAST_SIZE_MIB}

  health_check:

  k8s_observer:
    auth_type: serviceAccount
    node: ${K8S_NODE_NAME}

  zpages:

receivers:
  {{- include "splunk-otel-collector.otelReceivers" . | nindent 2 }}
  {{- if (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
  fluentforward:
    endpoint: 0.0.0.0:8006
  {{- end }}

  # Prometheus receiver scraping metrics from the pod itself
  prometheus/agent:
    config:
      scrape_configs:
      - job_name: 'otel-agent'
        scrape_interval: 10s
        static_configs:
        - targets:
          - "${K8S_POD_IP}:8889"
          # Fluend metrics collection disabled by default
          # - "${K8S_POD_IP}:24231"

  {{- if (eq (include "splunk-otel-collector.metricsEnabled" .) "true") }}
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
      {{- if or .Values.autodetect.prometheus .Values.autodetect.istio }}
      prometheus_simple:
        {{- if .Values.autodetect.prometheus }}
        # Enable prometheus scraping for pods with standard prometheus annotations
        rule: type == "pod" && annotations["prometheus.io/scrape"] == "true"
        {{- else }}
        # Enable prometheus scraping for istio pods only
        rule: type == "pod" && annotations["prometheus.io/scrape"] == "true" && "istio.io/rev" in labels
        {{- end }}
        config:
          metrics_path: '`"prometheus.io/path" in annotations ? annotations["prometheus.io/path"] : "/metrics"`'
          endpoint: '`endpoint`:`"prometheus.io/port" in annotations ? annotations["prometheus.io/port"] : 9090`'
      {{- end }}

  kubeletstats:
    collection_interval: 10s
    auth_type: serviceAccount
    endpoint: ${K8S_NODE_IP}:10250
    metric_groups:
      - container
      - pod
      - node
      # Volume metrics are not collected by default
      # - volume
    # To collect metadata from underlying storage resources, set k8s_api_config and list k8s.volume.type
    # under extra_metadata_labels
    # k8s_api_config:
    #  auth_type: serviceAccount
    extra_metadata_labels:
      - container.id
      # - k8s.volume.type

  signalfx:
    endpoint: 0.0.0.0:9943
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.o11yTracesEnabled" .) "true") }}
  smartagent/signalfx-forwarder:
    type: signalfx-forwarder
    listenAddress: 0.0.0.0:9080
  {{- end }}

  {{- if and (eq .Values.logsEngine "otel") .Values.logsCollection.containers.enabled }}
  filelog:
    include: ["/var/log/pods/*/*/*.log"]
    # Exclude logs. The file format is
    # /var/log/pods/<namespace_name>_<pod_name>_<pod_uid>/<container_name>/<restart_count>.log
    exclude:
      {{- if .Values.logsCollection.containers.excludeAgentLogs }}
      - /var/log/pods/{{ .Release.Namespace }}_{{ include "splunk-otel-collector.fullname" . }}*_*/otel-collector/*.log
      {{- end }}
      {{- range $_, $excludePath := .Values.logsCollection.containers.exclude_paths }}
      - {{ $excludePath }}
      {{- end }}
    start_at: beginning
    include_file_path: true
    include_file_name: false
    poll_interval: 200ms
    max_concurrent_files: 1024
    encoding: utf-8
    fingerprint_size: 1kb
    max_log_size: 1MiB
    # Disable force flush until this issue is fixed:
    # https://github.com/open-telemetry/opentelemetry-log-collection/issues/292
    force_flush_period: "0"
    operators:
      {{- if not .Values.logsCollection.containers.containerRuntime }}
      - type: router
        id: get-format
        routes:
          - output: parser-docker
            expr: '$$$$body matches "^\\{"'
          - output: parser-crio
            expr: '$$$$body matches "^[^ Z]+ "'
          - output: parser-containerd
            expr: '$$$$body matches "^[^ Z]+Z"'
      {{- end }}
      {{- if or (not .Values.logsCollection.containers.containerRuntime) (eq .Values.logsCollection.containers.containerRuntime "cri-o") }}
      # Parse CRI-O format
      - type: regex_parser
        id: parser-crio
        regex: '^(?P<time>[^ Z]+) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) (?P<log>.*)$'
        timestamp:
          parse_from: time
          layout_type: gotime
          layout: '2006-01-02T15:04:05.000000000-07:00'
      - type: recombine
        id: crio-recombine
        combine_field: log
        is_last_entry: "($$.logtag) == 'F'"
      - type: restructure
        id: crio-handle_empty_log
        output: filename
        if: $$.log == nil
        ops:
          - add:
              field: log
              value: ""
      {{- end }}
      {{- if or (not .Values.logsCollection.containers.containerRuntime) (eq .Values.logsCollection.containers.containerRuntime "containerd") }}
      # Parse CRI-Containerd format
      - type: regex_parser
        id: parser-containerd
        regex: '^(?P<time>[^ ^Z]+Z) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) (?P<log>.*)$'
        timestamp:
          parse_from: time
          layout: '%Y-%m-%dT%H:%M:%S.%LZ'
      - type: recombine
        id: containerd-recombine
        combine_field: log
        is_last_entry: "($$.logtag) == 'F'"
      - type: restructure
        id: containerd-handle_empty_log
        output: filename
        if: $$.log == nil
        ops:
          - add:
              field: log
              value: ""
      {{- end }}
      {{- if or (not .Values.logsCollection.containers.containerRuntime) (eq .Values.logsCollection.containers.containerRuntime "docker") }}
      # Parse Docker format
      - type: json_parser
        id: parser-docker
        timestamp:
          parse_from: time
          layout: '%Y-%m-%dT%H:%M:%S.%LZ'
      {{- end }}
      - type: metadata
        id: filename
        resource:
          com.splunk.source: EXPR($$$$attributes["file.path"])
      # Extract metadata from file path
      - type: regex_parser
        id: extract_metadata_from_filepath
        regex: '^\/var\/log\/pods\/(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[^\/]+)\/(?P<container_name>[^\._]+)\/(?P<restart_count>\d+)\.log$'
        parse_from: $$$$attributes["file.path"]
      # Move out attributes to Attributes
      - type: metadata
        resource:
          k8s.pod.uid: 'EXPR($$.uid)'
          k8s.container.restart_count: 'EXPR($$.restart_count)'
          k8s.container.name: 'EXPR($$.container_name)'
          k8s.namespace.name: 'EXPR($$.namespace)'
          k8s.pod.name: 'EXPR($$.pod_name)'
          com.splunk.sourcetype: 'EXPR("kube:container:"+$$.container_name)'
        attributes:
          stream: 'EXPR($$.stream)'
      {{- if .Values.logsCollection.containers.multilineConfigs }}
      - type: router
        routes:
        {{- range $.Values.logsCollection.containers.multilineConfigs }}
          - output: {{ include "splunk-otel-collector.newlineKey" . | quote }}
            expr: {{ include "splunk-otel-collector.newlineExpr" . | quote }}
        {{- end }}
        default: clean-up-log-record
      {{- range $.Values.logsCollection.containers.multilineConfigs }}
      - type: recombine
        id: {{ include "splunk-otel-collector.newlineKey" . | quote}}
        output: clean-up-log-record
        combine_field: log
        is_first_entry: '($$.log) matches {{ .firstEntryRegex | quote }}'
      {{- end }}
      {{- end }}
      {{- with .Values.logsCollection.containers.extraOperators }}
      {{ . | toYaml | nindent 6 }}
      {{- end }}
      # Clean up log record
      - type: restructure
        id: clean-up-log-record
        ops:
          - move:
              from: log
              to: $$
  {{- end }}

# By default k8sattributes and batch processors enabled.
processors:
  # k8sattributes enriches traces and metrics with k8s metadata
  k8sattributes:
    # If standalone collector deployment is enabled, the `passthrough` configuration is enabled by default.
    # It means that traces and metrics enrichment happens in collector, and the agent only passes information
    # about traces and metrics source, without calling k8s API.
    {{- if .Values.otelCollector.enabled }}
    passthrough: true
    {{- end }}
    filter:
      node_from_env_var: K8S_NODE_NAME
    pod_association:
      - from: resource_attribute
        name: k8s.pod.uid
      - from: resource_attribute
        name: k8s.pod.ip
      - from: resource_attribute
        name: ip
      - from: connection
      - from: resource_attribute
        name: host.name
    extract:
      metadata:
        - k8s.namespace.name
        - k8s.node.name
        - k8s.pod.name
        - k8s.pod.uid
        - container.id
        - container.image.name
        - container.image.tag
      annotations:
        - key: splunk.com/sourcetype
          from: pod
        - key: splunk.com/exclude
          tag_name: splunk.com/exclude
          from: namespace
        - key: splunk.com/exclude
          tag_name: splunk.com/exclude
          from: pod
      {{- with .Values.extraAttributes.podLabels }}
      labels:
        {{- range . }}
        - key: {{ . }}
        {{- end }}
      {{- end }}

  {{- if eq .Values.logsEngine "fluentd" }}
  # Move flat fluentd logs attributes to resource attributes
  groupbyattrs/logs:
    keys:
     - com.splunk.source
     - com.splunk.sourcetype
     - container.id
     - fluent.tag
     - istio_service_name
     - k8s.container.name
     - k8s.namespace.name
     - k8s.pod.name
     - k8s.pod.uid
  {{- end }}

  {{- if not .Values.otelCollector.enabled }}
  {{- include "splunk-otel-collector.resourceLogsProcessor" . | nindent 2 }}
  {{- include "splunk-otel-collector.filterLogsProcessors" . | nindent 2 }}
  {{- end }}

  {{- include "splunk-otel-collector.otelMemoryLimiterConfig" .Values.otelAgent | nindent 2 }}

  batch:

  {{- include "splunk-otel-collector.resourceDetectionProcessor" . | nindent 2 }}

  resource:
    # General resource attributes that apply to all telemetry passing through the agent.
    attributes:
      - action: insert
        key: k8s.node.name
        value: "${K8S_NODE_NAME}"
      {{- with .Values.clusterName }}
      - action: insert
        key: k8s.cluster.name
        value: "{{ . }}"
      {{- end }}
      {{- range .Values.extraAttributes.custom }}
      - action: insert
        key: "{{ .name }}"
        value: "{{ .value }}"
      {{- end }}

  # Resource attributes specific to the agent itself.
  resource/add_agent_k8s:
    attributes:
      - action: insert
        key: k8s.pod.name
        value: "${K8S_POD_NAME}"
      - action: insert
        key: k8s.pod.uid
        value: "${K8S_POD_UID}"
      - action: insert
        key: k8s.namespace.name
        value: "${K8S_NAMESPACE}"

  {{- if .Values.environment }}
  resource/add_environment:
    attributes:
      - action: insert
        key: deployment.environment
        value: "{{ .Values.environment }}"
  {{- end }}

  {{- if .Values.isWindows }}
  metricstransform:
    transforms:
      - include: container.memory.working_set
        action: insert
        new_name: container.memory.usage
  {{- end }}

# By default only SAPM exporter enabled. It will be pointed to collector deployment if enabled,
# Otherwise it's pointed directly to signalfx backend based on the values provided in signalfx setting.
# These values should not be specified manually and will be set in the templates.
exporters:

  {{- if .Values.otelCollector.enabled }}
  # If collector is enabled, metrics, logs and traces will be sent to collector
  otlp:
    endpoint: {{ include "splunk-otel-collector.fullname" . }}:4317
    tls:
      insecure: true
  {{- else }}
  # If collector is disabled, data will be sent to directly to backends.
  {{- if (eq (include "splunk-otel-collector.o11yTracesEnabled" .) "true") }}
  {{- include "splunk-otel-collector.otelSapmExporter" . | nindent 2 }}
  {{- end }}
  {{- if (eq (include "splunk-otel-collector.o11yLogsEnabled" .) "true") }}
  splunk_hec/o11y:
    endpoint: {{ include "splunk-otel-collector.o11yIngestUrl" . }}/v1/log
    token: "${SPLUNK_O11Y_ACCESS_TOKEN}"
  {{- end }}
  {{- if (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformLogsExporter" . | nindent 2 }}
  {{- end }}
  {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformMetricsExporter" . | nindent 2 }}
  {{- end }}
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
  signalfx:
    correlation:
    {{- if .Values.otelCollector.enabled }}
    ingest_url: http://{{ include "splunk-otel-collector.fullname" . }}:9943
    api_url: http://{{ include "splunk-otel-collector.fullname" . }}:6060
    {{- else }}
    ingest_url: {{ include "splunk-otel-collector.o11yIngestUrl" . }}
    api_url: {{ include "splunk-otel-collector.o11yApiUrl" . }}
    {{- end }}
    access_token: ${SPLUNK_O11Y_ACCESS_TOKEN}
    sync_host_metadata: true
  {{- end }}

service:
  extensions:
    {{- if and (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq .Values.logsEngine "otel") }}
    - file_storage
    {{- end }}
    - health_check
    - k8s_observer
    - memory_ballast
    - zpages

  # By default there are two pipelines sending metrics and traces to standalone otel-collector otlp format
  # or directly to signalfx backend depending on otelCollector.enabled configuration.
  # The default pipelines should to be changed. You can add any custom pipeline instead.
  # In order to disable a default pipeline just set it to `null` in otelAgent.config overrides.
  pipelines:
    {{- if (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
    logs:
      receivers:
        {{- if and (eq .Values.logsEngine "otel") .Values.logsCollection.containers.enabled }}
        - filelog
        {{- end }}
        - fluentforward
        - otlp
      processors:
        - memory_limiter
        {{- if eq .Values.logsEngine "fluentd" }}
        - groupbyattrs/logs
        {{- end }}
        - k8sattributes
        - batch
        {{- if not .Values.otelCollector.enabled }}
        - filter/logs
        - resource/logs
        {{- end }}
        - resource
        - resourcedetection
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
      exporters:
        {{- if .Values.otelCollector.enabled }}
        - otlp
        {{- else }}
        {{- if (eq (include "splunk-otel-collector.o11yLogsEnabled" .) "true") }}
        - splunk_hec/o11y
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") }}
        - splunk_hec/platform_logs
        {{- end }}
        {{- end }}
    {{- end }}

    {{- if (eq (include "splunk-otel-collector.tracesEnabled" .) "true") }}
    # Default traces pipeline.
    traces:
      receivers: [otlp, jaeger, smartagent/signalfx-forwarder, zipkin]
      processors:
        - memory_limiter
        - k8sattributes
        - batch
        - resource
        - resourcedetection
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
      exporters:
        {{- if .Values.otelCollector.enabled }}
        - otlp
        {{- else }}
        - sapm
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" $) "true") }}
        # For trace/metric correlation.
        - signalfx
        {{- end }}
    {{- end }}

    {{- if (eq (include "splunk-otel-collector.metricsEnabled" .) "true") }}
    # Default metrics pipeline.
    metrics:
      receivers: [hostmetrics, kubeletstats, otlp, receiver_creator, signalfx]
      processors:
        - memory_limiter
        - batch
        - resource
        - resourcedetection
        {{- if .Values.isWindows }}
        - metricstransform
        {{- end }}
      exporters:
        {{- if .Values.otelCollector.enabled }}
        - otlp
        {{- else }}
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" .) "true") }}
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true") }}
        - splunk_hec/platform_metrics
        {{- end }}
        {{- end }}
    {{- end }}

    {{- if or (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true") }}
    # Pipeline for metrics collected about the agent pod itself.
    metrics/agent:
      receivers: [prometheus/agent]
      processors:
        - memory_limiter
        - batch
        - resource
        - resource/add_agent_k8s
        - resourcedetection
      exporters:
        {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
        # Use signalfx instead of otlp even if collector is enabled
        # in order to sync host metadata.
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true") }}
        {{- if .Values.otelCollector.enabled }}
        - otlp
        {{- else }}
        - splunk_hec/platform_metrics
        {{- end }}
        {{- end }}
    {{- end }}
{{- end }}
