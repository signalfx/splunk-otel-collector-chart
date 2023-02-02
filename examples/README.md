# Examples of Helm Chart value configurations and resulting rendered Kubernetes manifests

## Structure

Each example has a directory where each of the following is included.
- README.md: A short description about the example.
- A Helm values configuration file to demonstrate the example.
- A rendered_manifests directory that contains the rendered Kubernetes manifests for the example.
  - Search for "CHANGEME" to find the values that must be changed in order to use the rendered manifests directly.

## Using Install Examples

Usage example:
```
helm install my-splunk-otel-collector --values path-to-values-file.yaml splunk-otel-collector-chart/splunk-otel-collector
```

## Common Configurations

The Splunk OpenTelemetry Collector Chart can be configured to send data to
various backends, each example needs at least one of the following configuration
to know which backend to send data to.

All the provided examples must include one of these required parameter sets.

```yaml
# Splunk Platform required parameters
clusterName: CHANGEME
splunkPlatform:
  token: CHANGEME
  endpoint: http://localhost:8088/services/collector
```

or

```yaml
# Splunk Observability required parameters
clusterName: CHANGEME
splunkObservability:
  realm: CHANGEME
  accessToken: CHANGEME
```
