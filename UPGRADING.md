# Upgrade guidelines

## 0.37.1 to 0.38.0

[297](https://github.com/signalfx/splunk-otel-collector-chart/pull/297),
[301](https://github.com/signalfx/splunk-otel-collector-chart/pull/301) Several
parameters in values.yaml configuration were renamed according to [Splunk GDI
Specification](https://github.com/signalfx/gdi-specification/blob/main/specification/configuration.md#kubernetes-package-management-solutions)

If you use the following parameters in your custom values.yaml, please rename
them accordingly:
- `provider` -> `cloudProvider`
- `distro` -> `distribution`
- `otelAgent` -> `agent`
- `otelCollector` -> `gateway`
- `otelK8sClusterReceiver` -> `clusterReceiver`

[#295 Secret names are changed according to the GDI
specification](https://github.com/signalfx/splunk-otel-collector-chart/pull/295)

If you provide access token for Splunk Observability using a custom Kubernetes
secret (secter.create=false), please update the secret name from
`splunk_o11y_access_token` to `splunk_observability_access_token`

[#273 Changed configuration to fetch attributes from labels and annotations of pods and namespaces](https://github.com/signalfx/splunk-otel-collector-chart/pull/273)

The following parameter `podLabels` under the `extraAttributes` group is changed to `fromLabels`
This parameter needs to be updated in your custom values.yaml

## 0.36.2 to 0.37.0

[#232 Access to underlying node's filesystem was reduced to the minimum scope
required for default functionality: host metrics and logs
collection](https://github.com/signalfx/splunk-otel-collector-chart/pull/232)

If you have any extra receivers that require access to node's files or
directories that are not [mounted by
default](https://github.com/signalfx/splunk-otel-collector-chart/blob/83fefe2a01effaab1e9eaba34a2557863981a2cd/helm-charts/splunk-otel-collector/templates/daemonset.yaml#L330-L347),
you need to setup additional volume mounts.

For example, if you have the following `smartagent/docker-container-stats`
receiver added to your configuration:

```yaml
agent:
  config:
    receivers:
      smartagent/docker-container-stats:
        type: docker-container-stats
        dockerURL: unix:///hostfs/var/run/docker.sock
```

You need to mount the docker socket to your container as follows:

```yaml
  extraVolumeMounts:
    - mountPath: /hostfs/var/run/docker.sock
      name: host-var-run-docker
      readOnly: true
  extraVolumes:
    - name: host-var-run-docker
      hostPath:
        path: /var/run/docker.sock
```

[#246 Simplify configuration for switching to native OTel logs
collection](https://github.com/signalfx/splunk-otel-collector-chart/pull/246)

The config to enable native OTel logs collection was changed from

```yaml
fluentd:
  enabled: false
logsCollection:
  enabled: true
```

to

```yaml
logsEngine: otel
```

Enabling both engines is not supported anymore. If you need that, you can
install fluentd separately.

## 0.35.3 to 0.36.0

[#209 Configuration interface changed to support both Splunk Enterprise/Cloud and Splunk Observability destinations](https://github.com/signalfx/splunk-otel-collector-chart/pull/209)

The following parameters are now deprecated and moved under
`splunkObservability` group. They need to be updated in your custom values.yaml
files before backward compatibility is discontinued.

Required parameters:

- `splunkRealm` changed to `splunkObservability.realm`
- `splunkAccessToken` changed to `splunkObservability.accessToken`

Optional parameters:

- `ingestUrl` changed to `splunkObservability.ingestUrl`
- `apiUrl` changed to `splunkObservability.apiUrl`
- `metricsEnabled` changed to `splunkObservability.metricsEnabled`
- `tracesEnabled` changed to `splunkObservability.tracesEnabled`
- `logsEnabled` changed to `splunkObservability.logsEnabled`

## 0.26.4 to 0.27.0

[#163 Auto-detection of prometheus metrics is disabled by default](https://github.com/signalfx/splunk-otel-collector-chart/pull/163):
If you rely on automatic prometheus endpoints detection to scrape prometheus
metrics from pods in your k8s cluster, make sure to add this configuration to
your values.yaml:

```yaml
autodetect:
  prometheus: true
```
