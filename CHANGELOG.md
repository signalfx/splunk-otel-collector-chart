# Changelog

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## Unreleased

[Upgrade
guidelines](https://github.com/signalfx/splunk-otel-collector-chart#0362-to-0370)

### Added

- Add recommended Kubernetes labels (#217)
- Add an option to skip RBAC resources creation (#231)
- Enable container metadata. This gives all collected logs new attributes:
  `container.image.name` and `container.image.tag`. Also the native OTel logs
  collection gets `container.id` attribute that allows container level
  correlation in Splunk Observability Cloud closing a feature parity gap with
  fluentd (#238)

### Changed

- BREAKING CHANGE: Reorder resource detectors, moving the `system` detector
  to the end of the list. Applying this change in an EC2 or Azure environment
  may change the `host.name` dimension and the resource ID dimension
  on some MTSes, possibly causing detectors to fire.
- BREAKING CHANGE: Reduce scope of host mounted volumes on linux systems (#232)
- Change `run_id` log resource attribute to `k8s.container.restart_count` (#226)
- Use only `splunkPlatform.endpoint` and `splunkObservability.realm` parameters
  to identify which destination is enabled, remove default value for
  `splunkObservability.realm` (#230, #233)
- Upgrade splunk-otel-collector image to 0.37.0 (#237)
- Simplify configuration for switching to native OTel logs collection (#245)

### Fixed

- Fix setting of SPLUNK_MEMORY_TOTAL_MIB env var in otelAgent daemonset (#240)
- Enable OTLP HTTP ports (4318 and 55681) in otelAgent daemonset (#243)

## [0.36.2] - 2021-10-08

### Fixed

- Exclude redundant `groupbyattrs/logs` processor from native logs collection
  pipeline (#219)
- Fix deprecation messages for old `<telemetry>Enabled` parameters (#220)

## [0.36.1] - 2021-10-07

### Fixed

- Fix backward compatibility for `splunkRealm` parameter (#218)

## [0.36.0] - 2021-10-06

### Added

- Support k8s clusters with Windows nodes (#190)

### Changed

- Change configuration interface to be able to send data to Splunk
  Enterprise/Cloud and to Splunk Observability (#209)
- Improve multiline logs configuration for native logs collection (#208)

## [0.35.3] - 2021-09-29

### Added

- Add an option to provide additional custom RBAC rules (#206)

## [0.35.2] - 2021-09-28

### Added

- Send k8s events additionally to Splunk HEC endpoint (#202)

## [0.35.1] - 2021-09-23

### Added

- Add support for OpenShift distribution (#196)
- Add native OTel logs collection as an option (#197)

### Removed

- Remove PodSecurityPolicy installation option (#195)

## [0.35.0] - 2021-09-17

### Added

- Add an option to collect k8s events with smartagent/kubernetes-events receiver (#187)

### Changed

- Move k8s metadata enrichment from fluentd to otel-collector (#192)

## [0.31.0] - 2021-08-10

### Changed

- Upgrade splunk-otel-collector image to 0.31.0 (#183)
- Set more frequent checks for memory_limiter (#178)
- Make Fluentd init container image variable (#182)

### Fixed

- All missing attributes are added to prometheus metrics reported
  by gateway and k8s-cluster-receiver collector deployments (#170)
- Fix pod affinity setting (#181)

## [0.29.1] - 2021-07-09

### Fixed

- Fix generation of service.name log attribute in istio environment (#176)

## [0.29.0] - 2021-07-08

### Changed

- Change internal metrics port from 8888 to 8889 (#172)
- Upgrade splunk-otel-collector image version to 0.29.0 (#174)

## [0.28.2] - 2021-07-07

### Added

- Add Istio specific configurations (#171)
- Enable OTLP receiver in logs pipeline (#167)

### Removed

- BREAKING: Remove SAPM receiver from default config (#168)

## [0.28.1] - 2021-06-18

### Changed

- Upgrade splunk-otel-collector image to 0.28.1 (#166)

## [0.28.0] - 2021-06-16

### Changed

- Upgrade splunk-otel-collector image to 0.28.0 (#164)

## [0.27.0] - 2021-06-15

### Changed

- BREAKING CHANGE: Auto-detection of prometheus metrics is disabled by default (#163). See
  [Upgrade guideline](https://github.com/signalfx/splunk-otel-collector-chart#0264-to-0270)

## [0.26.4] - 2021-06-09

### Fixed

- Fix container runtime detection when metrics pipeline disabled (#161)

## [0.26.3] - 2021-06-08

- Add an option to add extra labels to pods (#158)
- Add an option to add extra annotations to deployments, daemonset, pods and service account (#158)
- Add an option to mount extra volumes to gateway-mode and k8s cluster receiver collectors (#157)

## [0.26.2] - 2021-05-28

### Changed

- Automatically detect container runtime using initContainers and apply
  relevant parsing config instead of asking user to specify criTimeFormat.
  This is an important change to enable smooth transition from deprecated docker
  to containerd runtime (#154)

## [0.26.1] - 2021-05-25

### Added

- Add an option to mount extra volumes using `otelAgent.extraVolumes` and `otelAgent.extraVolumeMounts` (#151)

## [0.26.0] - 2021-05-21

### Added

- Add signalfx metrics receiver to the agent (#136)

### Changed

- fluentd logs are now sent through the collector instead of being sent directly to the backend (#109)
- Logs are sent through the OpenTelemetry Agent on the local node by default. `otelAgent.enabled` value must be set to `true` when using logs (#127)
- `otelAgent.ports` and `otelCollector.ports` are selectively enabled depending on what telemetry types are enabled with `metricsEnabled`, `tracesEnabled`, and `logsEnabled`
- Removed setting `host.name` through the `resource` processor as it is already set by the `resourcedetection/system` processor
- Upgraded to Splunk OpenTelemetry Connector 0.26.0
- Kubernetes cluster metrics now have a dimension `receiver:k8scluster` to ensure that
  MTS do not conflict with Kubernetes metrics sent by Smart Agent for the same cluster. (#134)

### Removed

- Removed `ingestHost`, `ingestPort`, `ingestProtocol`, use `ingestUrl` instead (#123)
- Removed `logsBackend`, configure `splunk_hec` exporter directly (#123)
- Removed `splunk.com/index` annotation for logs (#123)
- Removed `fluentd.config.indexFields` as all fields sent are indexed (#123)
- Removed `fluentforward` receiver from gateway (#127)
- Removed `service.ports`, sourced from `otelCollector.ports` instead (#140)

## [0.25.0] - 2021-05-07

### Changed

- Upgrade splunk-otel-collector docker image to 0.25.0 (#131)

### Added

- Pre-rendered manifests can be found in [rendered](rendered) directory

## [0.24.13] - 2021-05-04

### Changed

- Remove internal fluentd metrics sent as logs with monitor_agent. Prometheus
  metrics exposed on 0.0.0.0:24231 should be used instead (#122)

## [0.24.12] - 2021-05-03

### Fixed

- Fix logs collection configuration for CRI-O / containerd runtimes (#120)

## [0.24.11] - 2021-04-29

### Changed

- Change the way to configure "concat" filter for container logs (#117)

## [0.24.10] - 2021-04-21

### Changed

- Disable fluentd metrics collection by default (#108)

## [0.24.9] - 2021-04-18

### Changed

- Change OTLP port from deprecated 55680 to default 4317 (#103)

### Fixed

- Open port for signalfx-forwarder on the agent (#106)

## [0.24.8] - 2021-04-16

### Fixed

- Fix traces enrichment with k8s metadata (#102)

## [0.24.7] - 2021-04-15

### Changed

- Switch to stable Splunk OTel Collector image 0.24.3 (#100)

## [0.24.6] - 2021-04-15

### Added

- Enable smartagent/signalfx-forwarder in the default agent trace pipeline (#98)

## [0.24.5] - 2021-04-13

### Added

- Enable batch processor in the default metrics pipelines (#90)

### Changed

- Ensure all metrics and traces are routed through the gateway deployment if
  it's enabled (#96)

## [0.24.4] - 2021-04-12

### Added

- Add an option to set extra environment variables (#91)

## [0.24.3] - 2021-04-12

### Fixed

- Fix resource attribute in the default traces pipeline (#88)
- Add metric_source:kubernetes for all k8s cluster metrics (#89)
- Fix host.name attribute in logs (#87)

## [0.24.2] - 2021-04-07

### Added

- Add host.name attribute to logs (#86)

## [0.24.1] - 2021-04-07

### Changed

- Remove deprecated opencensus receiver (#85)

## [0.24.0] - 2021-04-07

### Changed

- Upgrade image to 0.24.0 version (#84)
- Add system detector to default metrics and traces pipelines (#84)
