# Examples of chart configuration

The Splunk OpenTelemetry Collector Chart can be configured in many ways to
support different use-cases. Here is a collection of example values.yaml files
that can be used with the chart installation or upgrade commands to change the
default behavior.

Usage example:
```
helm install my-splunk-otel-collector --values my-values.yaml splunk-otel-collector-chart/splunk-otel-collector
```

All of the provided examples must also include the required parameters:

```yaml
# Splunk Platform required parameters
splunkPlatform:
  token: xxxxxx
  endpoint: http://localhost:8088/services/collector
```

or

```yaml
# Splunk Observability required parameters
clusterName: my-cluster
splunkObservability:
  realm: us0
  accessToken: my-access-token
```

## Enable traces sampling

This example shows how to change default OTel Collector configuration to add
[Probabilistic Sampling Processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/probabilisticsamplerprocessor).
This approach can be used for any other OTel Collector re-configuration as well.
Final OTel config will be created by merging the custom config provided in
`agent.config` into [default configuration of agent-mode
collector](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/helm-charts/splunk-otel-collector/templates/config/_otel-agent.tpl).

```yaml
agent:
  config:
    processors:
      probabilistic_sampler:
        hash_seed: 22
        sampling_percentage: 15.3
    service:
      pipelines:
        traces:
          processors:
            - memory_limiter
            - probabilistic_sampler
            - k8sattributes
            - batch
            - resource
            - resourcedetection
```

In the example above, first we define a new processor, then add it to the
default traces pipeline. The pipeline has to be fully redefined, because
lists cannot merge - they have to be overridden.

## Add Receiver Creator

This example shows how to add a receiver creator to the OTel Collector configuration
[Receiver Creator](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/receivercreator).
In this example we will configure it to observe kubernetes pods and in case there is a pod
using port 5432 the collector will dynamically create a smartagent/postgresql receiver to monitor it.

```yaml
agent:
  config:
    receivers:
      receiver_creator:
        watch_observers: [k8s_observer]
        receivers:
          smartagent/postgresql:
            rule: type == "port" && port == 5432
            config:
              type: postgresql
              connectionString: 'sslmode=disable user={{.username}} password={{.password}}'
              params:
                username: postgres
                password: password
              port: 5432
```

By default, the receiver_creator receiver is part of the metrics pipeline, for example:
```yaml
pipelines:
    metrics:
      receivers:
      - hostmetrics
      - kubeletstats
      - otlp
      - receiver_creator
      - signalfx
```

## Enable OTel Collector in the gateway mode

This configuration installs collector as a gateway deployment along with
regular components. All the telemetry will be routed through this collector.
By default, the gateway-mode collector deployed with 3 replicas with 4 CPU
cores and 8Gb of memory each, but this can be easily changed as in this example.
`resources` can be adjusted for other components as well: `agent`,
`clusterReceiver`, `fluentd`.

```yaml
gateway:
  enabled: true
  replicaCount: 1
  resources:
    limits:
      cpu: 2
      memory: 4Gb
```

## Deploy gateway-mode OTel Collector only

This configuration will install collector as a gateway deployment only.
No metrics or logs will be collected from the gateway instance(s), the gateway
can be used to forward telemetry data through it for aggregation, enrichment
purposes.

```yaml
gateway:
  enabled: true
agent:
  enabled: false
clusterReceiver:
  enabled: false
```

## Route telemetry data through a gateway deployed separately

The following configuration can be used to forward telemetry through an OTel
collector gateway deployed separately.

```yaml
agent:
  config:
    exporters:
      otlp:
        endpoint: <custom-gateway-url>:4317
        tls:
          insecure: true
      signalfx:
        ingest_url: http://<custom-gateway-url>:9943
        api_url: http://<custom-gateway-url>:6060
    service:
      pipelines:
        traces:
          exporters: [otlp, signalfx]
        metrics:
          exporters: [otlp]
        logs:
          exporters: [otlp]

clusterReceiver:
  config:
    exporters:
      signalfx:
        ingest_url: http://<custom-gateway-url>:9943
        api_url: http://<custom-gateway-url>:6060
```

OTLP format is used between agent and gateway wherever possible for performance
reasons. OTLP is almost the same as internal data representation in OTel
Collector, so using it between agent and gateway reduce CPU cycles spent on
data format transformations.

## Route telemetry data through a proxy server

This configuration shows how to add extra environment variables to OTel
Collector containers to send the traffic through a proxy server from
both components that are enabled by default.

```yaml
agent:
  extraEnvs:
    - name: HTTPS_PROXY
      value: "192.168.0.10"
clusterReceiver:
  extraEnvs:
    - name: HTTPS_PROXY
      value: "192.168.0.10"
```

## Enable multiline logs parsing of Java stack traces

This configuration shows how to enable parsing of Java stack trace from all
pods in the cluster starting with "java-app" name.

```yaml
fluentd:
  config:
    logs:
      java-app:
        from:
          pod: "java-app"
        multiline:
          firstline: /\d{4}-\d{1,2}-\d{1,2}/
```

## Filter out specific containers

This example shows how you can filter out specific containers from metrics
pipelines. This could be adapted for other metadata or pipelines.
Filters should be added to both the agent and the cluster receiver.

```yaml
agent:
  config:
    processors:
      filter/exclude_containers:
        metrics:
          exclude:
            match_type: regexp
            resource_attributes:
              - Key: k8s.container.name
                Value: '^(containerX|containerY)$'
    service:
      pipelines:
        metrics:
          processors:
            - memory_limiter
            - batch
            - resourcedetection
            - resource
            - filter/exclude_containers
clusterReceiver:
  config:
    processors:
      filter/exclude_containers:
        metrics:
          exclude:
            match_type: regexp
            resource_attributes:
              - Key: k8s.container.name
                Value: '^(containerX|containerY)$'
    service:
      pipelines:
        metrics:
          processors:
            - memory_limiter
            - batch
            - resource
            - resource/k8s_cluster
            - filter/exclude_containers
```

# Logs collection configuration for CRI-O container runtime

Default logs collection is configured for Docker container runtime.
The following configuration should be set for CRI-O or containerd runtimes,
e.g. OpenShift.

```yaml
fluentd:
  config:
    containers:
      logFormatType: cri
      criTimeFormat: "%Y-%m-%dT%H:%M:%S.%N%:z"
```

`criTimeFormat` can be used to configure logs collection for different log
formats, e.g. `criTimeFormat: "%Y-%m-%dT%H:%M:%S.%NZ"` for IBM IKS.

# Route log records to specific Splunk Enterprise indexes

Configure log collection to set the index to target with logs to the name
of the kubernetes namespace they originate from.

```yaml
logsCollection:
  containers:
    extraOperators:
      - type: copy
        from: resource["k8s.namespace.name"]
        to: resource["com.splunk.index"]
```
