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
clusterName: my-cluster
splunkRealm: us0
splunkAccessToken: my-access-token
```

## Enable traces sampling

This example shows how to change default OTel Collector configuration to add
[Probabilistic Sampling Processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/probabilisticsamplerprocessor).
This approach can be used for any other OTel Collector re-configuration as well.
Final OTel config will be created by merging the custom config provided in
`otelAgent.config` into
[default configuration of agent-mode collector](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/helm-charts/splunk-otel-collector/templates/config/_otel-agent.tpl).

```yaml
otelAgent:
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
            - k8s_tagger
            - batch
            - resource
            - resourcedetection
```

In the example above, first we define a new processor, then add it to the
default traces pipeline. The pipeline has to be fully redefined, because
lists cannot merge - they have to be overridden.

## Enable OTel Collector in the gateway mode

This configuration installs collector as a gateway deployment along with
regular components. All the telemetry will be routed through this collector.
By default, the gateway-mode collector deployed with 3 replicas with 4 CPU
cores and 8Gb of memory each, but this can be easily changed as in this example.
`resources` can be adjusted for other components as well: `otelAgent`,
`otelK8sClusterReceiver`, `fluentd`.

```yaml
otelCollector:
  enabled: true
  replicaCount: 1
  resources:
    limits:
      cpu: 2
      memory: 4Gb
```

## Deploy gateway-mode OTel Collector only

This configuration will install collector as a gateway deployment only.
No metrics or logs will be collector, the gateway can be used to forward
telemetry data through it for aggregation, enrichment purposes.

```yaml
otelCollector:
  enabled: true
otelAgent:
  enabled: false
logsEnabled: false
```

## Route telemetry data through a gateway deployed separately

The following configuration can be used to forward telemetry through an OTel
collector gateway deployed separately.

```yaml
otelAgent:
  config:
    exporters:
      otlp:
        endpoint: <custom-gateway-url>:4317
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

otelK8sClusterReceiver:
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
otelAgent:
  extraEnvs:
    - name: HTTPS_PROXY
      value: "192.168.0.10"
otelK8sClusterReceiver:
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
