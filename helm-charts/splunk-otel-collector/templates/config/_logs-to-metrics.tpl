{{/*
Whether the experimental file-log Logs-to-Metrics pipeline is enabled.
*/}}
{{- define "splunk-otel-collector.logsToMetricsEnabled" -}}
{{- .Values.logsToMetrics.enabled -}}
{{- end -}}

{{/*
Whether a catalog rule is selected. An empty rule list selects the entire catalog.
Expects a dict containing the root context and the rule name.
*/}}
{{- define "splunk-otel-collector.logsToMetricsRuleEnabled" -}}
{{- or (eq (len .root.Values.logsToMetrics.rules) 0) (has .rule .root.Values.logsToMetrics.rules) -}}
{{- end -}}

{{/*
Whether any rule in a supplied list is selected.
Expects a dict containing the root context and the rule list.
*/}}
{{- define "splunk-otel-collector.logsToMetricsAnyRuleEnabled" -}}
{{- $root := .root -}}
{{- $enabled := false -}}
{{- range $rule := .rules -}}
  {{- if eq (include "splunk-otel-collector.logsToMetricsRuleEnabled" (dict "root" $root "rule" $rule)) "true" -}}
    {{- $enabled = true -}}
  {{- end -}}
{{- end -}}
{{- $enabled -}}
{{- end -}}

{{/*
Whether at least one count connector rule is selected.
*/}}
{{- define "splunk-otel-collector.logsToMetricsCountConnectorEnabled" -}}
{{- include "splunk-otel-collector.logsToMetricsAnyRuleEnabled" (dict "root" . "rules" (list "app.log.error.count" "http.server.error.count" "app.exception.unhandled.count" "app.authentication.failure.count" "app.job.failure.count" "app.retry.exhausted.count" "app.request.throttled.count" "app.log.record.count")) -}}
{{- end -}}

{{/*
Whether the sum connector rule is selected.
*/}}
{{- define "splunk-otel-collector.logsToMetricsSumConnectorEnabled" -}}
{{- include "splunk-otel-collector.logsToMetricsRuleEnabled" (dict "root" . "rule" "business.transaction.value") -}}
{{- end -}}

{{/*
Working resource attribute allowlist for log-derived metrics. Pod UID remains transiently in
workload mode so downstream Kubernetes enrichment can associate generated metrics.
*/}}
{{- define "splunk-otel-collector.logsToMetricsWorkingResourceAllowlist" -}}
{{- $attributes := list "service.name" "service.namespace" "service.version" "deployment.environment" "deployment.environment.name" "cloud.provider" "cloud.platform" "cloud.account.id" "cloud.region" "cloud.availability_zone" "k8s.cluster.name" "k8s.cluster.uid" "k8s.namespace.name" "k8s.pod.uid" "k8s.container.name" "k8s.deployment.name" "k8s.statefulset.name" "k8s.daemonset.name" "k8s.cronjob.name" "k8s.workload.name" "k8s.workload.kind" "com.splunk.index" "com.splunk.sourcetype" -}}
{{- if eq .Values.logsToMetrics.aggregation "instance" -}}
  {{- $attributes = concat $attributes (list "service.instance.id" "k8s.pod.name" "k8s.node.name" "container.id") -}}
{{- end -}}
{{- $attributes = append $attributes "splunk.logs_to_metrics" -}}
{{- $attributes | toJson | replace "\",\"" "\", \"" -}}
{{- end -}}

{{/*
Final resource attribute allowlist for log-derived metrics. Instance aggregation adds unique
pod, node, container ID, and available service instance identity. The internal marker is removed
by the last collector that handles the generated metrics.
*/}}
{{- define "splunk-otel-collector.logsToMetricsResourceAllowlist" -}}
{{- $attributes := list "service.name" "service.namespace" "service.version" "deployment.environment" "deployment.environment.name" "cloud.provider" "cloud.platform" "cloud.account.id" "cloud.region" "cloud.availability_zone" "k8s.cluster.name" "k8s.cluster.uid" "k8s.namespace.name" "k8s.container.name" "k8s.deployment.name" "k8s.statefulset.name" "k8s.daemonset.name" "k8s.cronjob.name" "k8s.workload.name" "k8s.workload.kind" "com.splunk.index" "com.splunk.sourcetype" -}}
{{- if eq .Values.logsToMetrics.aggregation "instance" -}}
  {{- $attributes = concat $attributes (list "service.instance.id" "k8s.pod.name" "k8s.pod.uid" "k8s.node.name" "container.id") -}}
{{- end -}}
{{- $attributes = append $attributes "splunk.logs_to_metrics" -}}
{{- $attributes | toJson | replace "\",\"" "\", \"" -}}
{{- end -}}

{{/*
Validate prerequisites for the experimental file-log Logs-to-Metrics pipeline.
*/}}
{{- define "splunk-otel-collector.validateLogsToMetrics" -}}
{{- if .Values.logsToMetrics.enabled -}}
  {{- $supportedRules := list "app.log.error.count" "http.server.error.count" "app.exception.unhandled.count" "app.authentication.failure.count" "app.job.failure.count" "app.retry.exhausted.count" "app.request.throttled.count" "app.log.record.count" "business.transaction.value" -}}
  {{- $supportedMappingTargets := list "severity_number" "severity_text" "error.type" "exception.type" "exception.escaped" "http.response.status_code" "http.request.method" "event.name" "event.outcome" "auth.mechanism" "job.type" "retry.exhausted" "request.throttled" "business.transaction.value" "business.transaction.type" "business.transaction.unit" -}}
  {{- if not (has .Values.logsToMetrics.aggregation (list "workload" "instance")) -}}
    {{- fail (printf "logsToMetrics.aggregation contains unsupported value %q" .Values.logsToMetrics.aggregation) -}}
  {{- end -}}
  {{- $mappingTargets := dict -}}
  {{- range $mapping := .Values.logsToMetrics.fieldMappings -}}
    {{- $target := index $mapping "to" -}}
    {{- if not (has $target $supportedMappingTargets) -}}
      {{- fail (printf "logsToMetrics.fieldMappings contains unsupported target %q" $target) -}}
    {{- end -}}
    {{- if hasKey $mappingTargets $target -}}
      {{- fail (printf "logsToMetrics.fieldMappings contains duplicate target %q" $target) -}}
    {{- end -}}
    {{- $_ := set $mappingTargets $target true -}}
  {{- end -}}
  {{- range $rule := .Values.logsToMetrics.rules -}}
    {{- if not (has $rule $supportedRules) -}}
      {{- fail (printf "logsToMetrics.rules contains unsupported rule %q" $rule) -}}
    {{- end -}}
  {{- end -}}
  {{- if not .Values.agent.enabled -}}
    {{- fail "logsToMetrics.enabled requires agent.enabled=true" -}}
  {{- end -}}
  {{- if eq .Values.distribution "eks/fargate" -}}
    {{- fail "logsToMetrics.enabled is not supported with distribution=eks/fargate because the agent DaemonSet is unavailable" -}}
  {{- end -}}
  {{- if not .Values.logsCollection.containers.enabled -}}
    {{- fail "logsToMetrics.enabled requires logsCollection.containers.enabled=true" -}}
  {{- end -}}
  {{- if ne (include "splunk-otel-collector.logsEnabled" .) "true" -}}
    {{- fail "logsToMetrics.enabled requires a configured logs destination so source logs remain available" -}}
  {{- end -}}
  {{- if ne (include "splunk-otel-collector.metricsEnabled" .) "true" -}}
    {{- fail "logsToMetrics.enabled requires a configured metrics destination" -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{/*
Agent processor definitions for Logs-to-Metrics.
*/}}
{{- define "splunk-otel-collector.logsToMetricsAgentProcessors" -}}
{{- if eq (include "splunk-otel-collector.logsToMetricsEnabled" .) "true" }}
{{- $severityNormalizationEnabled := eq (include "splunk-otel-collector.logsToMetricsAnyRuleEnabled" (dict "root" . "rules" (list "app.log.error.count" "app.log.record.count"))) "true" -}}
{{- $httpNormalizationEnabled := eq (include "splunk-otel-collector.logsToMetricsAnyRuleEnabled" (dict "root" . "rules" (list "http.server.error.count" "app.request.throttled.count"))) "true" -}}
{{- $eventNameNormalizationEnabled := eq (include "splunk-otel-collector.logsToMetricsAnyRuleEnabled" (dict "root" . "rules" (list "app.authentication.failure.count" "app.job.failure.count"))) "true" -}}
{{- $eventOutcomeNormalizationEnabled := eq (include "splunk-otel-collector.logsToMetricsAnyRuleEnabled" (dict "root" . "rules" (list "app.authentication.failure.count" "app.job.failure.count" "business.transaction.value"))) "true" -}}
{{- $businessTransactionNormalizationEnabled := eq (include "splunk-otel-collector.logsToMetricsSumConnectorEnabled" .) "true" -}}
# Enrich the metricization branch with stable service and workload identity even when a gateway is enabled.
k8s_attributes/logs_to_metrics:
  pod_association:
    - sources:
      - from: resource_attribute
        name: k8s.pod.uid
    - sources:
      - from: resource_attribute
        name: k8s.pod.ip
    - sources:
      - from: resource_attribute
        name: ip
  extract:
    metadata:
      - service.name
      - service.namespace
      - service.version
      - k8s.namespace.name
      - k8s.container.name
      - k8s.deployment.name
      - k8s.statefulset.name
      - k8s.daemonset.name
      - k8s.cronjob.name
      {{- if eq .Values.logsToMetrics.aggregation "instance" }}
      - k8s.pod.name
      - k8s.pod.uid
      - k8s.node.name
      - container.id
      {{- end }}
      {{- if gt (len .Values.logsToMetrics.scope.workloads) 0 }}
      - k8s.job.name
      - k8s.replicaset.name
      {{- end }}
    annotations:
      - key: {{ include "splunk-otel-collector.filterAttr" . }}
        tag_name: {{ include "splunk-otel-collector.filterAttr" . }}
        from: namespace
      - key: {{ include "splunk-otel-collector.filterAttr" . }}
        tag_name: {{ include "splunk-otel-collector.filterAttr" . }}
        from: pod
    {{- if gt (len .Values.logsToMetrics.scope.podLabels) 0 }}
    labels:
      {{- range $key, $_ := .Values.logsToMetrics.scope.podLabels }}
      - key: {{ $key | quote }}
        tag_name: {{ printf "splunk.logs_to_metrics.scope.pod_label.%s" $key | quote }}
        from: pod
      {{- end }}
    {{- end }}
  filter:
    node_from_env_var: K8S_NODE_NAME

# Honor the chart's annotation filter, then drop logs outside any configured canary scope.
filter/logs_to_metrics:
  error_mode: ignore
  log_conditions:
  {{- if .Values.logsCollection.containers.useSplunkIncludeAnnotation }}
    - 'resource.attributes["splunk.com/include"] != "true"'
  {{- else }}
    - 'resource.attributes["splunk.com/exclude"] == "true"'
  {{- end }}
  {{- if gt (len .Values.logsToMetrics.scope.namespaces) 0 }}
    {{- $namespaceMatches := list -}}
    {{- range $namespace := .Values.logsToMetrics.scope.namespaces -}}
      {{- $namespaceMatches = append $namespaceMatches (printf `resource.attributes["k8s.namespace.name"] == %s` (toJson $namespace)) -}}
    {{- end }}
    - {{ printf "not (%s)" (join " or " $namespaceMatches) | quote }}
  {{- end }}
  {{- if gt (len .Values.logsToMetrics.scope.workloads) 0 }}
    {{- $workloadMatches := list -}}
    {{- $workloadAttributes := list "k8s.deployment.name" "k8s.statefulset.name" "k8s.daemonset.name" "k8s.cronjob.name" "k8s.job.name" "k8s.replicaset.name" -}}
    {{- range $workload := .Values.logsToMetrics.scope.workloads -}}
      {{- range $attribute := $workloadAttributes -}}
        {{- $workloadMatches = append $workloadMatches (printf `resource.attributes[%s] == %s` (toJson $attribute) (toJson $workload)) -}}
      {{- end -}}
    {{- end }}
    - {{ printf "not (%s)" (join " or " $workloadMatches) | quote }}
  {{- end }}
  {{ range $key, $value := .Values.logsToMetrics.scope.podLabels }}
    - {{ printf `resource.attributes[%s] != %s` (toJson (printf "splunk.logs_to_metrics.scope.pod_label.%s" $key)) (toJson $value) | quote }}
  {{ end }}

# Normalize structured container-log fields for the dedicated Logs-to-Metrics branch.
# This processor mutates only the file_log fan-out used by logs/log_to_metrics.
transform/logs_to_metrics:
  error_mode: silent
  log_statements:
    - merge_maps(log.attributes, log.body, "insert") where IsMap(log.body)
    - merge_maps(log.attributes, ParseJSON(log.body), "insert") where IsString(log.body) and IsMatch(log.body, "^\\s*\\{")
    {{ range $mapping := .Values.logsToMetrics.fieldMappings }}
    - {{ printf `set(log.attributes[%s], log.attributes[%s]) where log.attributes[%s] == nil and log.attributes[%s] != nil` (toJson (index $mapping "to")) (toJson (index $mapping "from")) (toJson (index $mapping "to")) (toJson (index $mapping "from")) | quote }}
    {{ end }}
    {{- if $severityNormalizationEnabled }}
    - set(log.attributes["severity_number"], Int(log.attributes["severity_number"])) where IsDouble(log.attributes["severity_number"]) and log.attributes["severity_number"] == Int(log.attributes["severity_number"])
    - set(log.attributes["splunk.logs_to_metrics.severity.text"], ToUpperCase(log.attributes["severity_text"])) where IsString(log.attributes["severity_text"])
    - set(log.attributes["splunk.logs_to_metrics.severity.text"], ToUpperCase(log.attributes["severity"])) where log.attributes["splunk.logs_to_metrics.severity.text"] == nil and IsString(log.attributes["severity"])
    - set(log.attributes["splunk.logs_to_metrics.severity.text"], ToUpperCase(log.attributes["level"])) where log.attributes["splunk.logs_to_metrics.severity.text"] == nil and IsString(log.attributes["level"])
    - set(log.severity_number, SEVERITY_NUMBER_TRACE) where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED and IsInt(log.attributes["severity_number"]) and log.attributes["severity_number"] >= 1 and log.attributes["severity_number"] <= 4
    - set(log.severity_number, SEVERITY_NUMBER_DEBUG) where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED and IsInt(log.attributes["severity_number"]) and log.attributes["severity_number"] >= 5 and log.attributes["severity_number"] <= 8
    - set(log.severity_number, SEVERITY_NUMBER_INFO) where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED and IsInt(log.attributes["severity_number"]) and log.attributes["severity_number"] >= 9 and log.attributes["severity_number"] <= 12
    - set(log.severity_number, SEVERITY_NUMBER_WARN) where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED and IsInt(log.attributes["severity_number"]) and log.attributes["severity_number"] >= 13 and log.attributes["severity_number"] <= 16
    - set(log.severity_number, SEVERITY_NUMBER_ERROR) where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED and IsInt(log.attributes["severity_number"]) and log.attributes["severity_number"] >= 17 and log.attributes["severity_number"] <= 20
    - set(log.severity_number, SEVERITY_NUMBER_FATAL) where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED and IsInt(log.attributes["severity_number"]) and log.attributes["severity_number"] >= 21 and log.attributes["severity_number"] <= 24
    - set(log.severity_number, SEVERITY_NUMBER_TRACE) where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED and IsString(log.attributes["splunk.logs_to_metrics.severity.text"]) and IsMatch(log.attributes["splunk.logs_to_metrics.severity.text"], "^TRACE")
    - set(log.severity_number, SEVERITY_NUMBER_DEBUG) where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED and IsString(log.attributes["splunk.logs_to_metrics.severity.text"]) and IsMatch(log.attributes["splunk.logs_to_metrics.severity.text"], "^DEBUG")
    - set(log.severity_number, SEVERITY_NUMBER_INFO) where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED and IsString(log.attributes["splunk.logs_to_metrics.severity.text"]) and IsMatch(log.attributes["splunk.logs_to_metrics.severity.text"], "^INFO")
    - set(log.severity_number, SEVERITY_NUMBER_WARN) where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED and IsString(log.attributes["splunk.logs_to_metrics.severity.text"]) and IsMatch(log.attributes["splunk.logs_to_metrics.severity.text"], "^(WARN|WARNING)")
    - set(log.severity_number, SEVERITY_NUMBER_ERROR) where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED and IsString(log.attributes["splunk.logs_to_metrics.severity.text"]) and IsMatch(log.attributes["splunk.logs_to_metrics.severity.text"], "^(ERROR|ERR)")
    - set(log.severity_number, SEVERITY_NUMBER_FATAL) where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED and IsString(log.attributes["splunk.logs_to_metrics.severity.text"]) and IsMatch(log.attributes["splunk.logs_to_metrics.severity.text"], "^(FATAL|CRITICAL)")
    - set(log.attributes["log.severity"], "TRACE") where log.severity_number >= SEVERITY_NUMBER_TRACE and log.severity_number < SEVERITY_NUMBER_DEBUG
    - set(log.attributes["log.severity"], "DEBUG") where log.severity_number >= SEVERITY_NUMBER_DEBUG and log.severity_number < SEVERITY_NUMBER_INFO
    - set(log.attributes["log.severity"], "INFO") where log.severity_number >= SEVERITY_NUMBER_INFO and log.severity_number < SEVERITY_NUMBER_WARN
    - set(log.attributes["log.severity"], "WARN") where log.severity_number >= SEVERITY_NUMBER_WARN and log.severity_number < SEVERITY_NUMBER_ERROR
    - set(log.attributes["log.severity"], "ERROR") where log.severity_number >= SEVERITY_NUMBER_ERROR and log.severity_number < SEVERITY_NUMBER_FATAL
    - set(log.attributes["log.severity"], "FATAL") where log.severity_number >= SEVERITY_NUMBER_FATAL
    - set(log.attributes["log.severity"], "UNSPECIFIED") where log.severity_number == SEVERITY_NUMBER_UNSPECIFIED
    {{- end }}
    {{- if $httpNormalizationEnabled }}
    - set(log.attributes["http.response.status_code"], Int(log.attributes["http.response.status_code"])) where IsDouble(log.attributes["http.response.status_code"]) and log.attributes["http.response.status_code"] == Int(log.attributes["http.response.status_code"])
    - set(log.attributes["http.request.method"], ToUpperCase(log.attributes["http.request.method"])) where IsString(log.attributes["http.request.method"])
    - set(log.attributes["http.request.method"], "_OTHER") where IsString(log.attributes["http.request.method"]) and not IsMatch(log.attributes["http.request.method"], "^(CONNECT|DELETE|GET|HEAD|OPTIONS|PATCH|POST|PUT|TRACE)$")
    {{- end }}
    {{- if $eventNameNormalizationEnabled }}
    - set(log.attributes["splunk.logs_to_metrics.event.name"], ToLowerCase(log.attributes["event.name"])) where IsString(log.attributes["event.name"])
    {{- end }}
    {{- if $eventOutcomeNormalizationEnabled }}
    - set(log.attributes["splunk.logs_to_metrics.event.outcome"], ToLowerCase(log.attributes["event.outcome"])) where IsString(log.attributes["event.outcome"])
    {{- end }}
    {{- if $businessTransactionNormalizationEnabled }}
    - set(log.attributes["business.transaction.unit"], log.attributes["business.transaction.currency"]) where log.attributes["business.transaction.unit"] == nil and IsString(log.attributes["business.transaction.currency"])
    # The 0.156 sum connector adds a value once per grouping attribute. Use one composite key,
    # then restore the two public attributes in transform/logs_to_metrics_metrics.
    - set(log.attributes["splunk.logs_to_metrics.business.transaction.type_unit"], Concat([log.attributes["business.transaction.type"], log.attributes["business.transaction.unit"]], "|")) where IsString(log.attributes["business.transaction.type"]) and log.attributes["business.transaction.type"] != "" and not IsMatch(log.attributes["business.transaction.type"], "\\|") and IsString(log.attributes["business.transaction.unit"]) and log.attributes["business.transaction.unit"] != "" and not IsMatch(log.attributes["business.transaction.unit"], "\\|")
    {{- end }}
    - set(resource.attributes["splunk.logs_to_metrics"], true)
    # Keep the selected resource identity plus transient association/routing keys. The metrics-side
    # transform applies the final aggregation allowlist and removes the internal marker before export.
    - {{ printf "keep_keys(resource.attributes, %s)" (include "splunk-otel-collector.logsToMetricsWorkingResourceAllowlist" .) | quote }}

# Normalize generated metric dimensions and enforce their resource allowlist.
transform/logs_to_metrics_metrics:
  error_mode: silent
  metric_statements:
    {{- if $businessTransactionNormalizationEnabled }}
    # Restore public transaction dimensions after the sum connector's single-key grouping workaround.
    - set(datapoint.attributes["business.transaction.type"], Split(datapoint.attributes["splunk.logs_to_metrics.business.transaction.type_unit"], "|")[0]) where metric.name == "business.transaction.value" and IsString(datapoint.attributes["splunk.logs_to_metrics.business.transaction.type_unit"])
    - set(datapoint.attributes["business.transaction.unit"], Split(datapoint.attributes["splunk.logs_to_metrics.business.transaction.type_unit"], "|")[1]) where metric.name == "business.transaction.value" and IsString(datapoint.attributes["splunk.logs_to_metrics.business.transaction.type_unit"])
    - delete_key(datapoint.attributes, "splunk.logs_to_metrics.business.transaction.type_unit") where metric.name == "business.transaction.value"
    {{- end }}
    - {{ printf `keep_keys(resource.attributes, %s) where resource.attributes["splunk.logs_to_metrics"] == true` (include "splunk-otel-collector.logsToMetricsResourceAllowlist" .) | quote }}
    {{- if not .Values.gateway.enabled }}
    - delete_key(resource.attributes, "splunk.logs_to_metrics") where resource.attributes["splunk.logs_to_metrics"] == true
    {{- end }}
{{- end }}
{{- end -}}

{{/*
Agent connector definitions for Logs-to-Metrics.
*/}}
{{- define "splunk-otel-collector.logsToMetricsAgentConnectorDefinitions" -}}
{{- if eq (include "splunk-otel-collector.logsToMetricsEnabled" .) "true" }}
{{- if eq (include "splunk-otel-collector.logsToMetricsCountConnectorEnabled" .) "true" }}
count/logs_to_metrics:
  logs:
    {{- if eq (include "splunk-otel-collector.logsToMetricsRuleEnabled" (dict "root" . "rule" "app.log.error.count")) "true" }}
    app.log.error.count:
      description: Number of application log records with ERROR or higher severity.
      conditions:
        - 'log.severity_number >= SEVERITY_NUMBER_ERROR'
      attributes:
        - key: error.type
          default_value: unknown
        - key: log.severity
          default_value: ERROR
    {{- end }}
    {{- if eq (include "splunk-otel-collector.logsToMetricsRuleEnabled" (dict "root" . "rule" "http.server.error.count")) "true" }}
    http.server.error.count:
      description: Number of HTTP server responses with a 5xx status code.
      conditions:
        - 'IsInt(log.attributes["http.response.status_code"]) and log.attributes["http.response.status_code"] >= 500 and log.attributes["http.response.status_code"] <= 599'
      attributes:
        - key: http.request.method
          default_value: unknown
        - key: http.response.status_code
        - key: error.type
          default_value: unknown
    {{- end }}
    {{- if eq (include "splunk-otel-collector.logsToMetricsRuleEnabled" (dict "root" . "rule" "app.exception.unhandled.count")) "true" }}
    app.exception.unhandled.count:
      description: Number of escaped or otherwise unhandled application exceptions.
      conditions:
        - 'IsString(log.attributes["exception.type"]) and log.attributes["exception.type"] != "" and IsBool(log.attributes["exception.escaped"]) and log.attributes["exception.escaped"] == true'
      attributes:
        - key: exception.type
    {{- end }}
    {{- if eq (include "splunk-otel-collector.logsToMetricsRuleEnabled" (dict "root" . "rule" "app.authentication.failure.count")) "true" }}
    app.authentication.failure.count:
      description: Number of structured authentication failure events.
      conditions:
        - 'log.attributes["splunk.logs_to_metrics.event.name"] == "authentication" and log.attributes["splunk.logs_to_metrics.event.outcome"] == "failure"'
      attributes:
        - key: auth.mechanism
          default_value: unknown
    {{- end }}
    {{- if eq (include "splunk-otel-collector.logsToMetricsRuleEnabled" (dict "root" . "rule" "app.job.failure.count")) "true" }}
    app.job.failure.count:
      description: Number of terminal structured background-job failure events.
      conditions:
        - 'log.attributes["splunk.logs_to_metrics.event.name"] == "job" and log.attributes["splunk.logs_to_metrics.event.outcome"] == "failure" and IsString(log.attributes["job.type"]) and log.attributes["job.type"] != ""'
      attributes:
        - key: job.type
        - key: error.type
          default_value: unknown
    {{- end }}
    {{- if eq (include "splunk-otel-collector.logsToMetricsRuleEnabled" (dict "root" . "rule" "app.retry.exhausted.count")) "true" }}
    app.retry.exhausted.count:
      description: Number of operations that exhausted all configured retries.
      conditions:
        - 'IsBool(log.attributes["retry.exhausted"]) and log.attributes["retry.exhausted"] == true'
      attributes:
        - key: error.type
          default_value: unknown
    {{- end }}
    {{- if eq (include "splunk-otel-collector.logsToMetricsRuleEnabled" (dict "root" . "rule" "app.request.throttled.count")) "true" }}
    app.request.throttled.count:
      description: Number of HTTP 429 responses or explicitly throttled requests.
      conditions:
        - '(IsInt(log.attributes["http.response.status_code"]) and log.attributes["http.response.status_code"] == 429) or (IsBool(log.attributes["request.throttled"]) and log.attributes["request.throttled"] == true)'
      attributes:
        - key: http.request.method
          default_value: unknown
        - key: http.response.status_code
    {{- end }}
    {{- if eq (include "splunk-otel-collector.logsToMetricsRuleEnabled" (dict "root" . "rule" "app.log.record.count")) "true" }}
    app.log.record.count:
      description: Number of Kubernetes container log records processed by file_log.
      attributes:
        - key: log.severity
          default_value: UNSPECIFIED
    {{- end }}
{{- end }}
{{- if eq (include "splunk-otel-collector.logsToMetricsSumConnectorEnabled" .) "true" }}
sum/logs_to_metrics:
  logs:
    business.transaction.value:
      description: Sum of successful structured business transaction values.
      source_attribute: business.transaction.value
      conditions:
        - '(IsInt(log.attributes["business.transaction.value"]) or IsDouble(log.attributes["business.transaction.value"])) and log.attributes["splunk.logs_to_metrics.event.outcome"] == "success" and IsString(log.attributes["splunk.logs_to_metrics.business.transaction.type_unit"])'
      attributes:
        - key: splunk.logs_to_metrics.business.transaction.type_unit
{{- end }}
{{- end }}
{{- end -}}

{{/*
The complete agent connectors section is kept here because the top-level YAML key is shared
with Secure App routing.
*/}}
{{- define "splunk-otel-collector.logsToMetricsAgentConnectors" -}}
{{- if or .Values.splunkObservability.secureAppEnabled (eq (include "splunk-otel-collector.logsToMetricsEnabled" .) "true") }}
connectors:
  {{- if .Values.splunkObservability.secureAppEnabled }}
  routing/logs:
    {{- if or (eq (include "splunk-otel-collector.logsEnabled" .) "true") (eq (include "splunk-otel-collector.profilingEnabled" .) "true") }}
    default_pipelines: [logs]
    {{- else }}
    default_pipelines: []
    {{- end }}
    table:
      - context: log
        condition: instrumentation_scope.name == "secureapp"
        pipelines: [logs/secureapp]
  {{- end }}
  {{- include "splunk-otel-collector.logsToMetricsAgentConnectorDefinitions" . | nindent 2 }}
{{- end }}
{{- end -}}

{{/*
Agent file-log branch for Logs-to-Metrics.
*/}}
{{- define "splunk-otel-collector.logsToMetricsAgentPipeline" -}}
{{- if eq (include "splunk-otel-collector.logsToMetricsEnabled" .) "true" }}
# Opt-in Logs-to-Metrics branch sharing the normal container file_log receiver.
logs/log_to_metrics:
  receivers:
    - file_log
  processors:
    - memory_limiter
    - k8s_attributes/logs_to_metrics
    - filter/logs_to_metrics
    - resource_detection
    - resource
    {{- if .Values.environment }}
    - resource/add_environment
    {{- end }}
    - transform/logs_to_metrics
  exporters:
    {{- if eq (include "splunk-otel-collector.logsToMetricsCountConnectorEnabled" .) "true" }}
    - count/logs_to_metrics
    {{- end }}
    {{- if eq (include "splunk-otel-collector.logsToMetricsSumConnectorEnabled" .) "true" }}
    - sum/logs_to_metrics
    {{- end }}
{{- end }}
{{- end -}}

{{/*
Logs-to-Metrics connector receivers for the agent metrics pipeline.
*/}}
{{- define "splunk-otel-collector.logsToMetricsAgentMetricReceivers" -}}
{{- if eq (include "splunk-otel-collector.logsToMetricsEnabled" .) "true" }}
{{- if eq (include "splunk-otel-collector.logsToMetricsCountConnectorEnabled" .) "true" }}
- count/logs_to_metrics
{{- end }}
{{- if eq (include "splunk-otel-collector.logsToMetricsSumConnectorEnabled" .) "true" }}
- sum/logs_to_metrics
{{- end }}
{{- end }}
{{- end -}}

{{/*
Final Logs-to-Metrics processor for the agent metrics pipeline.
*/}}
{{- define "splunk-otel-collector.logsToMetricsAgentMetricProcessors" -}}
{{- if eq (include "splunk-otel-collector.logsToMetricsEnabled" .) "true" }}
# Keep this last so resource detection and platform enrichment cannot reintroduce unsafe dimensions.
- transform/logs_to_metrics_metrics
{{- end }}
{{- end -}}

{{/*
Gateway processor definitions for Logs-to-Metrics.
*/}}
{{- define "splunk-otel-collector.logsToMetricsGatewayProcessors" -}}
{{- if eq (include "splunk-otel-collector.logsToMetricsEnabled" .) "true" }}
# Apply the final cardinality allowlist after gateway-side metric enrichment.
transform/logs_to_metrics_metrics:
  error_mode: silent
  metric_statements:
    - {{ printf `keep_keys(resource.attributes, %s) where resource.attributes["splunk.logs_to_metrics"] == true` (include "splunk-otel-collector.logsToMetricsResourceAllowlist" .) | quote }}
    - delete_key(resource.attributes, "splunk.logs_to_metrics") where resource.attributes["splunk.logs_to_metrics"] == true
{{- end }}
{{- end -}}

{{/*
Final Logs-to-Metrics processor for the gateway metrics pipeline.
*/}}
{{- define "splunk-otel-collector.logsToMetricsGatewayMetricProcessors" -}}
{{- if eq (include "splunk-otel-collector.logsToMetricsEnabled" .) "true" }}
# Keep this last so gateway enrichment cannot reintroduce unsafe dimensions.
- transform/logs_to_metrics_metrics
{{- end }}
{{- end -}}
