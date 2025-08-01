{{/*
Config for the otel-collector agent
The values can be overridden in .Values.agent.config
*/}}
{{- define "splunk-otel-collector.agentConfig" -}}
{{ $gateway := fromYaml (include "splunk-otel-collector.gateway" .) -}}
{{ $gatewayEnabled := eq (include "splunk-otel-collector.gatewayEnabled" .) "true" }}
extensions:
  {{- if and (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq .Values.logsEngine "otel") }}
  file_storage:
    directory: {{ .Values.logsCollection.checkpointPath }}
    {{- if not (eq (toString .Values.splunkPlatform.fsyncEnabled) "<nil>") }}
    fsync: {{ .Values.splunkPlatform.fsyncEnabled }}
    {{- end }}
  {{- end }}

  {{- if .Values.splunkPlatform.sendingQueue.persistentQueue.enabled }}
  file_storage/persistent_queue:
    directory: {{ .Values.splunkPlatform.sendingQueue.persistentQueue.storagePath }}/agent
    timeout: 0
    {{- if not (eq (toString .Values.splunkPlatform.fsyncEnabled) "<nil>") }}
    fsync: {{ .Values.splunkPlatform.fsyncEnabled }}
    {{- end }}
  {{- end }}


  health_check:
    endpoint: 0.0.0.0:13133

  k8s_observer:
    auth_type: serviceAccount
    node: ${K8S_NODE_NAME}

  zpages:

  headers_setter:
    headers:
      - action: upsert
        key: X-SF-TOKEN
        from_context: X-SF-TOKEN
        default_value: "${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}"

receivers:
  {{- include "splunk-otel-collector.otelReceivers" . | nindent 2 }}
  {{- if (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
  fluentforward:
    endpoint: 0.0.0.0:8006
  {{- end }}

  {{- if eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true" }}
  # Placeholder receiver needed for discovery mode
  nop:
  {{- end }}

  # Prometheus receiver scraping metrics from the pod itself
  {{- include "splunk-otel-collector.prometheusInternalMetrics" (dict "receiver" "agent") | nindent 2}}

  {{- if (eq (include "splunk-otel-collector.metricsEnabled" .) "true") }}
  hostmetrics:
    collection_interval: 10s
    {{- if not .Values.isWindows }}
    root_path: "/hostfs"
    {{- end }}
    scrapers:
      cpu:
      disk:
      filesystem:
        # Collect metrics from the root filesystem only to avoid scraping errors since the collector
        # doesn't have access to all filesystems on the host by default. To collect metrics from
        # other devices, ensure that they are mounted to the collector container using
        # agent.extraVolumeMounts and agent.extraVolumes helm values options and override this list
        # using agent.config.hostmetrics.filesystem.include_mount_points.mount_points helm value.
        include_mount_points:
          match_type: strict
          mount_points:
            - "/"
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
      {{- if .Values.featureGates.useLightPrometheusReceiver }}
      lightprometheus:
      {{- else }}
      prometheus_simple:
      {{- end }}
        {{- if .Values.autodetect.prometheus }}
        # Enable prometheus scraping for pods with standard prometheus annotations
        rule: type == "pod" && annotations["prometheus.io/scrape"] == "true"
        {{- else }}
        # Enable prometheus scraping for istio pods only
        rule: type == "pod" && annotations["prometheus.io/scrape"] == "true" && "istio.io/rev" in labels
        {{- end }}
        config:
          {{- if .Values.featureGates.useLightPrometheusReceiver }}
          endpoint: 'http://`endpoint`:`"prometheus.io/port" in annotations ? annotations["prometheus.io/port"] : 9090``"prometheus.io/path" in annotations ? annotations["prometheus.io/path"] : "/metrics"`'
          resource_attributes:
            service.name:
              enabled: false
            service.instance.id:
              enabled: false
          {{- else }}
          metrics_path: '`"prometheus.io/path" in annotations ? annotations["prometheus.io/path"] : "/metrics"`'
          endpoint: '`endpoint`:`"prometheus.io/port" in annotations ? annotations["prometheus.io/port"] : 9090`'
          {{- end }}
      {{- end }}

      # Receivers for collecting k8s control plane metrics.
      # Distributions besides Kubernetes and Openshift are not supported.
      # Verified with Kubernetes v1.22 and Openshift v4.10.59.
      {{- if and (or (eq .Values.distribution "openshift") (eq .Values.distribution "")) (not (.Values.featureGates.useControlPlaneMetricsHistogramData)) }}
      # Below, the TLS certificate verification is often skipped because the k8s default certificate is self signed and
      # will fail the verification.
      {{- if .Values.agent.controlPlaneMetrics.coredns.enabled }}
      smartagent/coredns:
        {{- if eq .Values.distribution "openshift" }}
        rule: type == "pod" && namespace == "openshift-dns" && name contains "dns"
        {{- else }}
        rule: type == "pod" && labels["k8s-app"] == "kube-dns"
        {{- end }}
        config:
          extraDimensions:
            metric_source: k8s-coredns
          type: coredns
          {{- if eq .Values.distribution "openshift" }}
          port: 9154
          skipVerify: true
          useHTTPS: true
          useServiceAccount: true
          {{- else }}
          port: 9153
          {{- end }}
      {{- end }}
      {{- if .Values.agent.controlPlaneMetrics.etcd.enabled }}
      smartagent/etcd:
        {{- if eq .Values.distribution "openshift" }}
        rule: type == "pod" && labels["k8s-app"] == "etcd"
        {{- else }}
        rule: type == "pod" && (labels["k8s-app"] == "etcd-manager-events" || labels["k8s-app"] == "etcd-manager-main")
        {{- end }}
        config:
          clientCertPath: /otel/etc/etcd/tls.crt
          clientKeyPath: /otel/etc/etcd/tls.key
          useHTTPS: true
          type: etcd
          {{- if .Values.agent.controlPlaneMetrics.etcd.skipVerify }}
          skipVerify: true
          {{- else }}
          caCertPath: /otel/etc/etcd/cacert.pem
          skipVerify: false
          {{- end }}
          {{- if eq .Values.distribution "openshift" }}
          port: 9979
          {{- else }}
          port: 4001
          {{- end }}
      {{- end }}
      {{- if .Values.agent.controlPlaneMetrics.controllerManager.enabled }}
      smartagent/kube-controller-manager:
        {{- if eq .Values.distribution "openshift" }}
        rule: type == "pod" && labels["app"] == "kube-controller-manager" && labels["kube-controller-manager"] == "true"
        {{- else }}
        rule: type == "pod" && labels["k8s-app"] == "kube-controller-manager"
        {{- end }}
        config:
          extraDimensions:
            metric_source: kubernetes-controller-manager
          port: 10257
          skipVerify: true
          type: kube-controller-manager
          useHTTPS: true
          useServiceAccount: true
      {{- end }}
      {{- if .Values.agent.controlPlaneMetrics.apiserver.enabled }}
      smartagent/kubernetes-apiserver:
        {{- if eq .Values.distribution "openshift" }}
        rule: type == "port" && port == 6443 && pod.labels["app"] == "openshift-kube-apiserver" && pod.labels["apiserver"] == "true"
        {{- else }}
        rule: type == "port" && port == 443 && pod.labels["k8s-app"] == "kube-apiserver"
        {{- end }}
        config:
          extraDimensions:
            metric_source: kubernetes-apiserver
          skipVerify: true
          type: kubernetes-apiserver
          useHTTPS: true
          useServiceAccount: true
      {{- end }}
      {{- if .Values.agent.controlPlaneMetrics.proxy.enabled }}
      smartagent/kubernetes-proxy:
        {{- if eq .Values.distribution "openshift" }}
        rule: type == "port" && pod.labels["app"] == "sdn" && (port == 9101 || port == 29101)
        {{- else }}
        rule: type == "pod" && labels["k8s-app"] == "kube-proxy"
        {{- end }}
        config:
          extraDimensions:
            metric_source: kubernetes-proxy
          type: kubernetes-proxy
          # Connecting to kube proxy in unknown Kubernetes distributions can be troublesome and generate log noise
          # For now, set the scrape failure log level to debug when no specific distribution is selected
          {{- if eq .Values.distribution "" }}
          scrapeFailureLogLevel: debug
          {{- end }}
          {{- if eq .Values.distribution "openshift" }}
          skipVerify: true
          useHTTPS: true
          useServiceAccount: true
          {{- else }}
          port: 10249
          {{- end }}
      {{- end }}
      {{- if .Values.agent.controlPlaneMetrics.scheduler.enabled }}
      smartagent/kubernetes-scheduler:
        {{- if eq .Values.distribution "openshift" }}
        rule: type == "pod" && labels["app"] == "openshift-kube-scheduler" && labels["scheduler"] == "true"
        {{- else }}
        rule: type == "pod" && labels["k8s-app"] == "kube-scheduler"
        {{- end }}
        config:
          extraDimensions:
            metric_source: kubernetes-scheduler
          skipVerify: true
          port: 10259
          type: kubernetes-scheduler
          useHTTPS: true
          useServiceAccount: true
      {{- end }}
      {{- end }}

      {{- if and (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") .Values.featureGates.useControlPlaneMetricsHistogramData }}
      # Receivers for collecting k8s control plane metrics as native OpenTelemetry metrics, including histogram data.
      # Below, the TLS certificate verification is often skipped because the k8s default certificate is self signed and
      # will fail the verification.
      {{- if .Values.agent.controlPlaneMetrics.coredns.enabled }}
      {{- if eq .Values.distribution "gke"}}
      prometheus/kubedns:
        rule: type == "pod" && labels["k8s-app"] == "kube-dns"
        config:
          config:
            scrape_configs:
            - job_name: "kubedns"
              static_configs:
                - targets: ['`endpoint`:`"prometheus.io/port" in annotations ? annotations["prometheus.io/port"] : 9153`']
              tls_config:
                insecure_skip_verify: true
      {{- else }}
      prometheus/coredns:
        {{- if eq .Values.distribution "openshift" }}
        rule: type == "pod" && namespace == "openshift-dns" && name contains "dns"
        {{- else }}
        rule: type == "pod" && labels["k8s-app"] == "kube-dns"
        {{- end }}
        config:
          config:
            scrape_configs:
            - job_name: "coredns"
              {{- if eq .Values.distribution "openshift" }}
              static_configs:
                - targets: ["`endpoint`:9154"]
              tls_config:
                insecure_skip_verify: true
              {{- else }}
              static_configs:
                - targets: ["`endpoint`:9153"]
              {{- end }}
              metric_relabel_configs:
                - source_labels: [__name__]
                  action: keep
                  regex: "(coredns_dns_request_duration_seconds|\
                    coredns_cache_misses_total|\
                    coredns_cache_hits_total|\
                    coredns_cache_entries|\
                    coredns_dns_responses_total|\
                    coredns_dns_requests_total|\
                    rest_client_requests_total|\
                    rest_client_request_duration_seconds)(?:_sum|_count|_bucket)?"
      {{- end }}
      {{- end }}
      {{- if .Values.agent.controlPlaneMetrics.etcd.enabled }}
      prometheus/etcd:
        {{- if eq .Values.distribution "openshift" }}
        rule: type == "pod" && labels["k8s-app"] == "etcd"
        {{- else }}
        rule: type == "pod" && (labels["k8s-app"] == "etcd-manager-events" || labels["k8s-app"] == "etcd-manager-main" || labels["component"] == "etcd")
        {{- end }}
        config:
          config:
            scrape_configs:
            - job_name: "etcd"
              static_configs:
                - targets: ["`endpoint`:2381"]
              metric_relabel_configs:
                - source_labels: [__name__]
                  action: keep
                  regex: "(etcd_server_is_leader|\
                    etcd_server_leader_changes_seen_total|\
                    etcd_server_proposals_applied_total|\
                    etcd_server_proposals_committed_total|\
                    etcd_server_proposals_failed_total|\
                    etcd_server_proposals_pending|\
                    etcd_disk_wal_fsync_duration_seconds)(?:_sum|_count|_bucket)?"
      {{- end }}
      {{- if .Values.agent.controlPlaneMetrics.controllerManager.enabled }}
      prometheus/kube-controller-manager:
        {{- if eq .Values.distribution "openshift" }}
        rule: type == "pod" && labels["app"] == "kube-controller-manager" && labels["kube-controller-manager"] == "true"
        {{- else }}
        rule: type == "pod" && (labels["k8s-app"] == "kube-controller-manager" || labels["component"] == "kube-controller-manager")
        {{- end }}
        config:
          config:
            scrape_configs:
            - job_name: "kube-controller-manager"
              static_configs:
                - targets: ["`endpoint`:10257"]
              scheme: https
              authorization:
                credentials_file: "/var/run/secrets/kubernetes.io/serviceaccount/token"
                type: Bearer
              tls_config:
                ca_file: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
                insecure_skip_verify: true
              metric_relabel_configs:
                - source_labels: [__name__]
                  action: keep
                  regex: "(workqueue_longest_running_processor_seconds|\
                    workqueue_unfinished_work_seconds|\
                    workqueue_depth|\
                    workqueue_retries_total|\
                    workqueue_queue_duration_seconds)(?:_sum|_count|_bucket)?"
      {{- end }}
      {{- if .Values.agent.controlPlaneMetrics.apiserver.enabled }}
      prometheus/kubernetes-apiserver:
        {{- if eq .Values.distribution "openshift" }}
        rule: type == "port" && port == 6443 && pod.labels["app"] == "openshift-kube-apiserver" && pod.labels["apiserver"] == "true"
        {{- else }}
        rule: type == "port" && port == 443 && (pod.labels["k8s-app"] == "kube-apiserver" || pod.labels["component"] == "kube-apiserver")
        {{- end }}
        config:
          config:
            scrape_configs:
            - job_name: "kubernetes-apiserver"
              scheme: https
              authorization:
                credentials_file: "/var/run/secrets/kubernetes.io/serviceaccount/token"
                type: Bearer
              tls_config:
                ca_file: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
                insecure_skip_verify: true
              static_configs:
                - targets: ["`endpoint`"]
              metric_relabel_configs:
                - source_labels: [__name__]
                  action: keep
                  regex: "(apiserver_longrunning_requests|\
                    apiserver_request_duration_seconds|\
                    apiserver_storage_objects|\
                    apiserver_response_sizes|\
                    apiserver_request_total|\
                    rest_client_requests_total|\
                    rest_client_request_duration_seconds)(?:_sum|_count|_bucket)?"
      {{- end }}
      {{- if .Values.agent.controlPlaneMetrics.proxy.enabled }}
      prometheus/kubernetes-proxy:
        {{- if eq .Values.distribution "openshift" }}
        rule: type == "port" && pod.labels["app"] == "sdn" && (port == 9101 || port == 29101)
        {{- else }}
        rule: type == "pod" && labels["k8s-app"] == "kube-proxy"
        {{- end }}
        config:
          config:
            scrape_configs:
            - job_name: "kubernetes-proxy"
              {{- if eq .Values.distribution "openshift" }}
              scheme: https
              tls_config:
                ca_file: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
                insecure_skip_verify: true
              authorization:
                credentials_file: "/var/run/secrets/kubernetes.io/serviceaccount/token"
                type: Bearer
              static_configs:
                - targets: ["`endpoint`"]
              {{- else }}
              static_configs:
                - targets: ["`endpoint`:10249"]
              {{- end }}
              metric_relabel_configs:
                - source_labels: [__name__]
                  action: keep
                  regex: "(kubeproxy_sync_proxy_rules_iptables_restore_failures_total|\
                    kubeproxy_sync_proxy_rules_service_changes_total|\
                    kubeproxy_sync_proxy_rules_service_changes_pending|\
                    kubeproxy_sync_proxy_rules_duration_seconds|\
                    kubeproxy_network_programming_duration_seconds)(?:_sum|_count|_bucket)?"
      {{- end }}
      {{- if .Values.agent.controlPlaneMetrics.scheduler.enabled }}
      prometheus/kubernetes-scheduler:
        {{- if eq .Values.distribution "openshift" }}
        rule: type == "pod" && labels["app"] == "openshift-kube-scheduler" && labels["scheduler"] == "true"
        {{- else }}
        rule: type == "pod" && (labels["k8s-app"] == "kube-scheduler" || labels["component"] == "kube-scheduler")
        {{- end }}
        config:
          config:
            scrape_configs:
            - job_name: "kubernetes-scheduler"
              static_configs:
                - targets: ["`endpoint`:10259"]
              scheme: https
              tls_config:
                ca_file: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
                insecure_skip_verify: true
              authorization:
                credentials_file: "/var/run/secrets/kubernetes.io/serviceaccount/token"
                type: Bearer
              metric_relabel_configs:
                - source_labels: [__name__]
                  action: keep
                  regex: "(rest_client_request_duration_seconds|\
                    rest_client_requests_total|\
                    scheduler_pending_pods|\
                    scheduler_schedule_attempts_total|\
                    scheduler_queue_incoming_pods_total|\
                    scheduler_preemption_attempts_total|\
                    scheduler_scheduling_algorithm_duration_seconds|\
                    scheduler_pod_scheduling_sli_duration_seconds)(?:_sum|_count|_bucket)?"
      {{- end }}
    {{- end }}

  kubeletstats:
    collection_interval: 10s
    {{- if eq .Values.distribution "gke/autopilot" }}
    # GKE Autopilot doesn't allow using the secure kubelet endpoint,
    # use the read-only endpoint instead.
    auth_type: none
    endpoint: ${K8S_NODE_IP}:10255
    {{ else if and (eq .Values.distribution "aks") (not .Values.isWindows) }}
    ca_file: "/hostfs/etc/kubernetes/certs/kubeletserver.crt"
    endpoint: ${K8S_NODE_NAME}:10250
    auth_type: serviceAccount
    {{- else }}
    auth_type: serviceAccount
    endpoint: ${K8S_NODE_IP}:10250
    {{- end }}
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
    {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
    # Disable CPU usage metrics as they are not categorized as bundled in Splunk Observability
    metrics:
      container.cpu.usage:
        enabled: false
      k8s.pod.cpu.usage:
        enabled: false
      k8s.node.cpu.usage:
        enabled: false
    {{- end }}

  signalfx:
    endpoint: 0.0.0.0:9943
  {{- end }}

  {{- if .Values.targetAllocator.enabled  }}
  prometheus/ta:
    config:
      global:
        scrape_interval: 30s
    target_allocator:
      endpoint: http://{{ template "splunk-otel-collector.fullname" . }}-ta.{{ template "splunk-otel-collector.namespace" . }}.svc.cluster.local:80
      interval: 30s
      collector_id: ${env:K8S_POD_NAME}
  {{- end }}

  {{- if and (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq .Values.logsEngine "otel") }}
  {{- if .Values.logsCollection.containers.enabled }}
  filelog:
    {{- if not .Values.featureGates.fixMissedLogsDuringLogRotation }}
    {{- if .Values.isWindows }}
    include: ["C:\\var\\log\\pods\\*\\*\\*.log"]
    {{- else }}
    include: ["/var/log/pods/*/*/*.log"]
    {{- end }}
    {{- else }}
    {{- if .Values.isWindows }}
    include: ["C:\\var\\log\\pods\\*\\*\\*.log*"]
    {{- else }}
    include: ["/var/log/pods/*/*/*.log*"]
    {{- end }}
    {{- end }}
    # Exclude logs. The file format is
    # /var/log/pods/<namespace_name>_<pod_name>_<pod_uid>/<container_name>/<restart_count>.log
    exclude:
      {{- if .Values.featureGates.fixMissedLogsDuringLogRotation }}
      {{- if .Values.isWindows }}
      - "C:\\var\\log\\pods\\*\\*\\*.log*.gz"
      - "C:\\var\\log\\pods\\*\\*\\*.log*.tmp"
      {{- else }}
      - "/var/log/pods/*/*/*.log*.gz"
      - "/var/log/pods/*/*/*.log*.tmp"
      {{- end }}
      {{- end }}
      {{- if .Values.logsCollection.containers.excludeAgentLogs }}
      {{- if .Values.isWindows }}
      - "C:\\var\\log\\pods\\{{ template "splunk-otel-collector.namespace" . }}_{{ include "splunk-otel-collector.fullname" . }}*_*\\otel-collector\\*.log"
      {{- else }}
      - /var/log/pods/{{ template "splunk-otel-collector.namespace" . }}_{{ include "splunk-otel-collector.fullname" . }}*_*/otel-collector/*.log
      {{- end }}
      {{- end }}
      {{- range $_, $excludePath := .Values.logsCollection.containers.excludePaths }}
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
    storage: file_storage
    retry_on_failure:
      enabled: true
      {{- if .Values.featureGates.noDropLogsPipeline }}
      max_elapsed_time: 0s
      {{- end }}
    operators:
      {{- if not .Values.logsCollection.containers.containerRuntime }}
      - type: router
        id: get-format
        routes:
          - output: parser-docker
            expr: 'body matches "^\\{"'
          - output: parser-crio
            expr: 'body matches "^[^ Z]+ "'
          - output: parser-containerd
            expr: 'body matches "^[^ ]+ "'
      {{- end }}
      {{- if or (not .Values.logsCollection.containers.containerRuntime) (eq .Values.logsCollection.containers.containerRuntime "cri-o") }}
      # Parse CRI-O format
      - type: regex_parser
        id: parser-crio
        regex: '^(?P<time>[^ Z]+) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) ?(?P<log>.*)$'
        timestamp:
          parse_from: attributes.time
          layout_type: gotime
          layout: '2006-01-02T15:04:05.999999999Z07:00'
      - type: recombine
        id: crio-recombine
        output: handle_empty_log
        combine_field: attributes.log
        source_identifier: attributes["log.file.path"]
        is_last_entry: "attributes.logtag == 'F'"
        combine_with: ""
        max_log_size: {{ $.Values.logsCollection.containers.maxRecombineLogSize }}
      {{- end }}
      {{- if or (not .Values.logsCollection.containers.containerRuntime) (eq .Values.logsCollection.containers.containerRuntime "containerd") }}
      # Parse CRI-Containerd format
      - type: regex_parser
        id: parser-containerd
        regex: '^(?P<time>[^ ]+) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) ?(?P<log>.*)$'
        timestamp:
          parse_from: attributes.time
          layout_type: gotime
          layout: '2006-01-02T15:04:05.999999999Z07:00'
      - type: recombine
        id: containerd-recombine
        output: handle_empty_log
        combine_field: attributes.log
        source_identifier: attributes["log.file.path"]
        is_last_entry: "attributes.logtag == 'F'"
        combine_with: ""
        max_log_size: {{ $.Values.logsCollection.containers.maxRecombineLogSize }}
      {{- end }}
      {{- if or (not .Values.logsCollection.containers.containerRuntime) (eq .Values.logsCollection.containers.containerRuntime "docker") }}
      # Parse Docker format
      - type: json_parser
        id: parser-docker
        timestamp:
          parse_from: attributes.time
          layout: '%Y-%m-%dT%H:%M:%S.%LZ'
      - type: recombine
        id: docker-recombine
        output: handle_empty_log
        combine_field: attributes.log
        source_identifier: attributes["log.file.path"]
        is_last_entry: attributes.log endsWith "\n"
        combine_with: ""
        max_log_size: {{ $.Values.logsCollection.containers.maxRecombineLogSize }}
      {{- end }}
      - type: add
        id: handle_empty_log
        if: attributes.log == nil
        field: attributes.log
        value: ""
      # Extract metadata from file path
      - type: regex_parser
        {{- if not .Values.featureGates.fixMissedLogsDuringLogRotation }}
        {{- if .Values.isWindows }}
        regex: '^C:\\var\\log\\pods\\(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[^\/]+)\\(?P<container_name>[^\._]+)\\(?P<restart_count>\d+)\.log$'
        {{- else }}
        regex: '^\/var\/log\/pods\/(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[^\/]+)\/(?P<container_name>[^\._]+)\/(?P<restart_count>\d+)\.log$'
        {{- end }}
        {{- else }}
        {{- if .Values.isWindows }}
        regex: '^C:\\var\\log\\pods\\(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[^\/]+)\\(?P<container_name>[^\._]+)\\(?P<restart_count>\d+)\.log'
        {{- else }}
        regex: '^\/var\/log\/pods\/(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[^\/]+)\/(?P<container_name>[^\._]+)\/(?P<restart_count>\d+)\.log'
        {{- end }}
        {{- end }}
        parse_from: attributes["log.file.path"]
      # Move out attributes to Attributes
      - type: move
        from: attributes.uid
        to: resource["k8s.pod.uid"]
      - type: move
        from: attributes.restart_count
        to: resource["k8s.container.restart_count"]
      - type: move
        from: attributes.container_name
        to: resource["k8s.container.name"]
      - type: move
        from: attributes.namespace
        to: resource["k8s.namespace.name"]
      - type: move
        from: attributes.pod_name
        to: resource["k8s.pod.name"]
      - type: add
        field: resource["com.splunk.sourcetype"]
        value: EXPR("kube:container:"+resource["k8s.container.name"])
      - type: move
        from: attributes.stream
        to: attributes["log.iostream"]
      - type: move
        from: attributes["log.file.path"]
        to: resource["com.splunk.source"]
      {{- with .Values.logsCollection.containers.extraOperators }}
      {{ . | toYaml | nindent 6 }}
      {{- end }}
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
        source_identifier: resource["com.splunk.source"]
        combine_field: attributes.log
        is_first_entry: '(attributes.log) matches {{ .firstEntryRegex | quote }}'
        max_log_size: {{ $.Values.logsCollection.containers.maxRecombineLogSize }}
        {{- if hasKey . "combineWith" }}
        combine_with: {{ .combineWith | quote }}
        {{- end }}
      {{- end }}
      {{- end }}
      # Clean up log record
      - type: move
        id: clean-up-log-record
        from: attributes.log
        to: body
      - type: remove
        field: attributes.time
  {{- end }}

  {{- if .Values.logsCollection.extraFileLogs }}
  {{- $extraFileLogsList := .Values.logsCollection.extraFileLogs }}
  {{- range $extraFileLogKey, $extraFileLogValue := $extraFileLogsList }}
  {{- printf "%s:" $extraFileLogKey | nindent 2 }}
  {{- if not $extraFileLogValue.storage }}
    storage: file_storage
  {{- end }}
  {{- $extraFileLogValue | toYaml | nindent 4 }}
  {{- end }}
  {{- end }}

  # https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/journaldreceiver
  {{- if .Values.logsCollection.journald.enabled }}
  {{- range $_, $unit := .Values.logsCollection.journald.units }}
  {{- printf "journald/%s:" $unit.name | nindent 2 }}
    directory: {{ $.Values.logsCollection.journald.directory }}
    units: [{{ $unit.name }}]
    priority: {{ $unit.priority }}
    retry_on_failure:
      enabled: true
    storage: file_storage
    operators:
    - type: add
      field: resource["com.splunk.source"]
      value: {{ $.Values.logsCollection.journald.directory }}
    - type: add
      field: resource["com.splunk.sourcetype"]
      value: 'EXPR("kube:journald:"+body._SYSTEMD_UNIT)'
    - type: add
      field: resource["com.splunk.index"]
      value: {{ $.Values.logsCollection.journald.index | default $.Values.splunkPlatform.index }}
    - type: add
      field: resource["host.name"]
      value: 'EXPR(env("K8S_NODE_NAME"))'
    - type: add
      field: resource["journald.priority.number"]
      value: 'EXPR(body.PRIORITY)'
    - type: add
      field: resource["journald.unit.name"]
      value: 'EXPR(body._SYSTEMD_UNIT)'

    # extract MESSAGE field into the log body and discard rest of the fields
    - type: move
      id: set-body
      from: body.MESSAGE
      to: body
  {{- end }}
  {{- end }}
  {{- end }}

# By default k8sattributes and batch processors enabled.
processors:
  {{- include "splunk-otel-collector.k8sAttributesProcessor" . | nindent 2 }}
    # Agent specific configuration of k8sattributes:
    # If gateway deployment is enabled, the `passthrough` configuration is enabled by default.
    # It means that traces and metrics enrichment happens in the gateway, and the agent only passes information
    # about traces and metrics source, without calling k8s API.
    {{- if $gatewayEnabled }}
    passthrough: true
    {{- end }}
    filter:
      node_from_env_var: K8S_NODE_NAME


  {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
  {{- include "splunk-otel-collector.k8sAttributesSplunkPlatformMetrics" . | nindent 2 }}
    filter:
      node_from_env_var: K8S_NODE_NAME
  {{- if .Values.splunkPlatform.sourcetype }}
  {{- include "splunk-otel-collector.resourceMetricsProcessor" . | nindent 2 }}
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
     - k8s.container.name
     - k8s.namespace.name
     - k8s.pod.name
     - k8s.pod.uid
  {{- end }}

  {{- if not $gatewayEnabled }}
  {{- include "splunk-otel-collector.resourceLogsProcessor" . | nindent 2 }}
  {{- if .Values.autodetect.istio }}
  {{- include "splunk-otel-collector.transformLogsProcessor" . | nindent 2 }}
  {{- end }}
  {{- include "splunk-otel-collector.filterLogsProcessors" . | nindent 2 }}
  {{- if .Values.splunkPlatform.fieldNameConvention.renameFieldsSck }}
  transform/logs:
    log_statements:
      - context: log
        statements:
          - set(resource.attributes["container_image"], Concat([resource.attributes["container.image.name"], resource.attributes["container.image.tag"]], ":"))
  {{- end }}
  {{- end }}

  {{- include "splunk-otel-collector.otelMemoryLimiterConfig" . | nindent 2 }}

  batch:
    metadata_keys:
      - X-SF-Token

  # Resource detection processor is configured to override all host and cloud
  # attributes because OTel Collector Agent is the source of truth for all host
  # and cloud metadata, and instrumentation libraries can send wrong host
  # attributes from container environments.
  {{- include "splunk-otel-collector.resourceDetectionProcessor" . | nindent 2 }}

  # Resource detection processor that only detects the k8s.cluster.name attribute.
  {{- if eq (include "splunk-otel-collector.autoDetectClusterName" .) "true" }}
  {{- include "splunk-otel-collector.resourceDetectionProcessorKubernetesClusterName" . | nindent 2 }}
  {{- end }}

  # General resource attributes that apply to all telemetry passing through the agent.
  # It's important to put this processor after resourcedetection to make sure that
  # k8s.name.cluster attribute is always set to "{{ .Values.clusterName }}" when
  # it's declared.
  resource:
    attributes:
      - action: insert
        key: k8s.node.name
        value: "${K8S_NODE_NAME}"
      {{- if .Values.clusterName }}
      - action: upsert
        key: k8s.cluster.name
        value: {{ .Values.clusterName }}
      {{- end }}
      {{- range .Values.extraAttributes.custom }}
      - action: insert
        key: "{{ .name }}"
        value: "{{ .value }}"
      {{- end }}
      {{- if .Values.splunkPlatform.fieldNameConvention.renameFieldsSck }}
      - key: cluster_name
        from_attribute: k8s.cluster.name
        action: upsert
      {{- if not .Values.splunkPlatform.fieldNameConvention.keepOtelConvention }}
      - key: k8s.cluster.name
        action: delete
      {{- end }}
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

  # The following processor is used to add "otelcol.service.mode" attribute to the internal metrics
  resource/add_mode:
    attributes:
      - action: insert
        value: "agent"
        key: otelcol.service.mode

  {{- if .Values.isWindows }}
  metricstransform:
    transforms:
      - include: container.memory.working_set
        action: insert
        new_name: container.memory.usage
  {{- end }}

  {{- if or .Values.autodetect.prometheus .Values.autodetect.istio }}
  # This processor is used to remove excessive istio attributes to avoid running into the dimensions limit.
  # This configuration assumes single cluster istio deployment. If you run istio in multi-cluster scenarios or make use of the canonical service and revision labels,
  # you may need to adjust this configuration.
  attributes/istio:
    include:
      match_type: regexp
      metric_names:
        - istio_.*
    actions:
      - action: delete
        key: source_cluster
      - action: delete
        key: destination_cluster
      - action: delete
        key: source_canonical_service
      - action: delete
        key: destination_canonical_service
      - action: delete
        key: source_canonical_revision
      - action: delete
        key: destination_canonical_revision
  {{- end }}

# By default only SAPM exporter enabled. It will be pointed to collector deployment if enabled,
# Otherwise it's pointed directly to signalfx backend based on the values provided in signalfx setting.
# These values should not be specified manually and will be set in the templates.
exporters:

  {{- if $gatewayEnabled }}
  # If gateway is enabled, metrics, logs and traces will be sent to the gateway
  otlp:
    endpoint: {{ include "splunk-otel-collector.fullname" . }}:4317
    tls:
      insecure: true
    auth:
      authenticator: headers_setter
  {{- else }}
  # If gateway is disabled, data will be sent to directly to backends.
  {{- if (eq (include "splunk-otel-collector.o11yTracesEnabled" .) "true") }}
  {{- include "splunk-otel-collector.otlpHttpExporter" . | nindent 2 }}
  {{- end }}
  {{- if (eq (include "splunk-otel-collector.o11yLogsOrProfilingEnabled" .) "true") }}
  splunk_hec/o11y:
    endpoint: {{ include "splunk-otel-collector.o11yIngestUrl" . }}/v1/log
    token: "${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}"
    log_data_enabled: {{ .Values.splunkObservability.logsEnabled }}
    profiling_data_enabled: {{ .Values.splunkObservability.profilingEnabled }}
    # Temporary disable compression until 0.68.0 to workaround a compression bug
    disable_compression: true
  {{- end }}
  {{- $_ := set . "addPersistentStorage" .Values.splunkPlatform.sendingQueue.persistentQueue.enabled }}
  {{- if (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformLogsExporter" . | nindent 2 }}
  {{- end }}
  {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformMetricsExporter" . | nindent 2 }}
  {{- end }}
  {{- if (eq (include "splunk-otel-collector.platformTracesEnabled" .) "true") }}
  {{- include "splunk-otel-collector.splunkPlatformTracesExporter" . | nindent 2 }}
  {{- end }}
  {{- $_ := unset . "addPersistentStorage" }}
  {{- end }}

  {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
  signalfx:
    correlation:
    {{- if $gatewayEnabled }}
    ingest_url: http://{{ include "splunk-otel-collector.fullname" . }}:9943
    api_url: http://{{ include "splunk-otel-collector.fullname" . }}:6060
    {{- else }}
    ingest_url: {{ include "splunk-otel-collector.o11yIngestUrl" . }}
    api_url: {{ include "splunk-otel-collector.o11yApiUrl" . }}
    {{- end }}
    access_token: ${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}
    sync_host_metadata: true
    {{- if not .Values.isWindows }}
    root_path: /hostfs
    {{- end }}

  # To send entities (applicable only if discovery mode is enabled)
  otlphttp/entities:
    {{- if $gatewayEnabled }}
    endpoint: http://{{ include "splunk-otel-collector.fullname" . }}:4318
    {{- else }}
    logs_endpoint: {{ include "splunk-otel-collector.o11yIngestUrl" . }}/v3/event
    {{- end }}
    auth:
      authenticator: headers_setter

  {{- if .Values.featureGates.useControlPlaneMetricsHistogramData }}
  signalfx/histograms:
    ingest_url: {{ include "splunk-otel-collector.o11yIngestUrl" . }}
    api_url: {{ include "splunk-otel-collector.o11yApiUrl" . }}
    access_token: ${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}
    send_otlp_histograms: true
  {{- end }}
  {{- end }}

service:
  telemetry:
    resource:
      service.name: otel-agent
    metrics:
      readers:
        - pull:
            exporter:
              prometheus:
                host: localhost
                port: 8889
                without_scope_info: true
                without_units: true
                without_type_suffix: true
  extensions:
    {{- if and (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq .Values.logsEngine "otel") }}
    - file_storage
    {{- end }}
    {{- if .Values.splunkPlatform.sendingQueue.persistentQueue.enabled }}
    - file_storage/persistent_queue
    {{- end }}
    - health_check
    - headers_setter
    - k8s_observer
    - zpages

  # By default there are two pipelines sending metrics and traces to standalone otel-collector otlp format
  # or directly to signalfx backend depending on gateway.enabled configuration.
  # The default pipelines should to be changed. You can add any custom pipeline instead.
  # In order to disable a default pipeline just set it to `null` in agent.config overrides.
  pipelines:
    {{- if or (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq (include "splunk-otel-collector.profilingEnabled" .) "true") }}
    logs:
      receivers:
        {{- if (eq (include "splunk-otel-collector.logsEnabled" .) "true") }}
        {{- if and (eq .Values.logsEngine "otel") .Values.logsCollection.containers.enabled }}
        - filelog
        {{- end }}
        - fluentforward
        {{- end }}
        - otlp
      processors:
        - memory_limiter
        {{- if and (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq .Values.logsEngine "fluentd") }}
        - groupbyattrs/logs
        {{- end }}
        - k8sattributes
        {{- if not $gatewayEnabled }}
        - filter/logs
        {{- end }}
        {{- if not .Values.featureGates.noDropLogsPipeline }}
        - batch
        {{- end }}
        - resourcedetection
        - resource
        {{- if not $gatewayEnabled }}
        {{- if .Values.splunkPlatform.fieldNameConvention.renameFieldsSck }}
        - transform/logs
        {{- end }}
        {{- if .Values.autodetect.istio }}
        - transform/istio_service_name
        {{- end }}
        - resource/logs
        {{- end }}
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
      exporters:
        {{- if $gatewayEnabled }}
        - otlp
        {{- else }}
        {{- if (eq (include "splunk-otel-collector.o11yLogsOrProfilingEnabled" .) "true") }}
        - splunk_hec/o11y
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformLogsEnabled" .) "true") }}
        - splunk_hec/platform_logs
        {{- end }}
        {{- end }}

    {{- if and (eq .Values.logsEngine "otel") (or .Values.logsCollection.extraFileLogs .Values.logsCollection.journald.enabled) }}
    logs/host:
      receivers:
        {{- if .Values.logsCollection.extraFileLogs }}
        {{- range $key, $exporterData := .Values.logsCollection.extraFileLogs }}
        - {{ $key }}
        {{- end }}
        {{- end }}
        {{- if (.Values.logsCollection.journald.enabled)}}
        {{- range $_, $unit := .Values.logsCollection.journald.units }}
        {{- printf "- journald/%s" $unit.name | nindent 8 }}
        {{- end }}
        {{- end }}
      processors:
        - memory_limiter
        {{- if not .Values.featureGates.noDropLogsPipeline }}
        - batch
        {{- end }}
        {{- if eq (include "splunk-otel-collector.autoDetectClusterName" .) "true" }}
        - resourcedetection/k8s_cluster_name
        {{- end }}
        - resource
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
      exporters:
        {{- if $gatewayEnabled }}
        - otlp
        {{- else }}
        {{- if eq (include "splunk-otel-collector.platformLogsEnabled" .) "true" }}
        - splunk_hec/platform_logs
        {{- end }}
        {{- if eq (include "splunk-otel-collector.o11yLogsEnabled" .) "true" }}
        - splunk_hec/o11y
        {{- end }}
        {{- end }}
        {{- end }}
    {{- end }}

    {{- if (eq (include "splunk-otel-collector.tracesEnabled" .) "true") }}
    # Default traces pipeline.
    traces:
      receivers:
        - otlp
        - jaeger
        - zipkin
      processors:
        - memory_limiter
        - k8sattributes
        - batch
        - resourcedetection
        - resource
        {{- if .Values.environment }}
        - resource/add_environment
        {{- end }}
      exporters:
        {{- if $gatewayEnabled }}
        - otlp
        {{- else }}
        {{- if (eq (include "splunk-otel-collector.o11yTracesEnabled" .) "true") }}
        - otlphttp
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformTracesEnabled" .) "true") }}
        - splunk_hec/platform_traces
        {{- end }}
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.o11yMetricsEnabled" $) "true") }}
        # For trace/metric correlation.
        - signalfx
        {{- end }}
    {{- end }}

    {{- if (eq (include "splunk-otel-collector.metricsEnabled" .) "true") }}
    # Default metrics pipeline.
    metrics:
      receivers:
        - hostmetrics
        - kubeletstats
        - otlp
        {{- if not .Values.featureGates.useControlPlaneMetricsHistogramData }}
        - receiver_creator
        {{- end }}
        - signalfx
        {{- if .Values.targetAllocator.enabled  }}
        - prometheus/ta
        {{- end }}
      processors:
        - memory_limiter
        - batch
        {{- if or .Values.autodetect.prometheus .Values.autodetect.istio }}
        - attributes/istio
        {{- end }}
        - resourcedetection
        - resource
        {{/*
        The attribute `deployment.environment` is not being set on metrics sent to Splunk Observability because it's already synced as the `sf_environment` property.
        More details: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/signalfxexporter#traces-configuration-correlation-only
        */}}
        {{- if (and .Values.splunkPlatform.metricsEnabled .Values.environment) }}
        - resource/add_environment
        {{- end }}
        {{- if .Values.isWindows }}
        - metricstransform
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - k8sattributes/metrics
        {{- if .Values.splunkPlatform.sourcetype }}
        - resource/metrics
        {{- end }}
        {{- end }}
      exporters:
        {{- if $gatewayEnabled }}
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
        - resource/add_agent_k8s
        - resourcedetection
        - resource
        - resource/add_mode
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" $) "true") }}
        - k8sattributes/metrics
        {{- if .Values.splunkPlatform.sourcetype }}
        - resource/metrics
        {{- end }}
        {{- end }}
      exporters:
        {{- if (eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true") }}
        # Use signalfx instead of otlp even if collector is enabled
        # in order to sync host metadata.
        - signalfx
        {{- end }}
        {{- if (eq (include "splunk-otel-collector.platformMetricsEnabled" .) "true") }}
        {{- if $gatewayEnabled }}
        - otlp
        {{- else }}
        - splunk_hec/platform_metrics
        {{- end }}
        {{- end }}
    {{- end }}

    {{- if eq (include "splunk-otel-collector.splunkO11yEnabled" .) "true" }}
    logs/entities:
      # Receivers are added dinamically if discovery mode is enabled
      receivers: [nop]
      processors:
        - memory_limiter
        - batch
        - resourcedetection
        - resource
      exporters: [otlphttp/entities]

    {{- if .Values.featureGates.useControlPlaneMetricsHistogramData }}
    metrics/histograms:
      receivers:
       - receiver_creator
      processors:
        - memory_limiter
        - batch
        - resource/add_agent_k8s
        - resourcedetection
        - resource
      exporters:
        - signalfx/histograms
    {{- end }}
    {{- end }}
{{- end }}
{{/*
Discovery properties for the otel-collector agent
The values can be overridden in .Values.agent.discovery.properties.
*/}}
{{- define "splunk-otel-collector.agentDiscoveryProperties" -}}
extensions:
  docker_observer:
    enabled: false
  host_observer:
    enabled: false
receivers: {}
{{- end }}
