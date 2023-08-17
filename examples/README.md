# Examples of Helm Chart value configurations and resulting rendered Kubernetes manifests

## Structure

Each example has a directory where each of the following is included.
- README.md: A short description about the example.
- A Helm values configuration file to demonstrate the example, the file name always ends in values.yaml
  or values.norender.yaml.
  - Files ending with `-values.yaml` will be rendered by the script, resulting in a populated `rendered_manifests`
    directory.
  - Files ending with `-values.norender.yaml` are validated, but their rendered output is not persisted
    in the repository. This is because these files typically represent derivative configurations. While
    it's essential to ensure they are valid, we avoid persisting their output to keep the repository clean
    and focused. If such a file is present in an example directory, the now `rendered_manifests` directory
    and related content will exist.
- A rendered_manifests directory that contains the rendered Kubernetes manifests for the example.
  - Search for "CHANGEME" to find the values that must be changed in order to use the rendered manifests directly.

## Using Install Examples

Usage example:
```
helm install my-splunk-otel-collector --values path-to-values-file.yaml splunk-otel-collector-chart/splunk-otel-collector
```

## Common Configurations

The Splunk OpenTelemetry Collector Chart can be configured to export data to
to the following targets:
- [Splunk Enterprise](https://www.splunk.com/en_us/software/splunk-enterprise.html)
- [Splunk Cloud Platform](https://www.splunk.com/en_us/software/splunk-cloud-platform.html)
- [Splunk Observability Cloud](https://www.observability.splunk.com/)

All the provided examples must include one of these two configuration sets to
know which target to export data to.

Use these configurations for exporting data to Splunk Enterprise or Splunk Cloud Platform.
```yaml
# Splunk Platform required parameters
clusterName: CHANGEME
splunkPlatform:
  token: CHANGEME
  endpoint: http://localhost:8088/services/collector
```

Use these configurations for exporting data to Splunk Observability Cloud.
```yaml
# Splunk Observability required parameters
clusterName: CHANGEME
splunkObservability:
  realm: CHANGEME
  accessToken: CHANGEME
```

## For Developers

For developer-specific details regarding how to add examples, the involved rendering process, and other related notes, please refer to [DEVELOPER_GUIDE.md](./DEVELOPER_GUIDE.md).
