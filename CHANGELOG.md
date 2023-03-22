# Changelog

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## Unreleased

### Added

- Add experimental support for deploying OpenTelemetry Operator as a subchart [#691](https://github.com/signalfx/splunk-otel-collector-chart/pull/691)
- Improve documentation about providing tokens as Kubernetes secrets [#707](https://github.com/signalfx/splunk-otel-collector-chart/pull/691)

## [0.72.0] - 2023-03-09

### Added

- Add functional test coverage for Network Explorer metrics [#684](https://github.com/signalfx/splunk-otel-collector-chart/pull/684)
- Apply the same resources to init containers as allocated to the otel agent container [#690](https://github.com/signalfx/splunk-otel-collector-chart/pull/690)

## [0.71.0] - 2023-03-01

### Added

- Added examples for supported Kubernetes distributions and Kubernetes clusters with windows nodes ([#663](https://github.com/signalfx/splunk-otel-collector-chart/pull/663))
- Refactored the examples and rendered directories into one for better usability ([#658](https://github.com/signalfx/splunk-otel-collector-chart/pull/658))

### Changed

- Docker metadata turned off by default ([#655](https://github.com/signalfx/splunk-otel-collector-chart/pull/665))

### Fixed

- Translation of `k8s.pod.labels.app` attribute to SCK format ([#660](https://github.com/signalfx/splunk-otel-collector-chart/pull/660))

## [0.70.0] - 2023-01-31

### Added

- Support sending traces via Splunk HEC exporter ([#629](https://github.com/signalfx/splunk-otel-collector-chart/pull/629) - thanks to @mr-miles)

### Changed

- Upgrade splunk-otel-collector image to 0.70.0, skipping 0.69.0 release ([#653](https://github.com/signalfx/splunk-otel-collector-chart/pull/653))

### Fixed

- Fix invalid OpenShift SecurityContextConstraints template ([#652](https://github.com/signalfx/splunk-otel-collector-chart/pull/652))
- Limit `clusterReceiver.eventsEnabled` deprecation warning to feature users ([#648](https://github.com/signalfx/splunk-otel-collector-chart/pull/648))
- Fix noop validation for missing platform info ([#649](https://github.com/signalfx/splunk-otel-collector-chart/pull/649))

## [0.68.0] - 2023-01-25

### Added

- Allow to overwrite default SecurityContextConstraints rules with values.yaml file (#643)

### Changed

- Upgrade splunk-otel-collector image to 0.68.1 (#640)

### Fixed

- Default recombine operator for the docker container engine (#627)
- Added acl to journald log directory (#639)

## [0.67.0] - 2022-12-19

### Changed

- Upgrade splunk-otel-collector image to 0.67.0 (#612)

### Fixed

- Make sure the daemonset can start in GKE Autopiot (#608)
- Make containerd engine default in for fluentd logs and use always use it in GKE Autopiot (#609)
- Temporary disable compression in Splunk Observability logs exporter until
  0.68.0 to workaround a compression bug (#610)

## [0.66.1] - 2022-12-08

### Fixed

- Fixed network explorer image pull secrets

## [0.66.0] - 2022-12-06

### Changed

- Upgrade splunk-otel-collector image to 0.66.0 (#593)

## [0.64.0] - 2022-11-22

### Changed

- Upgrade splunk-otel-collector image to 0.64.0 (#589)

### Fixed

## [0.62.2] - 2022-11-21

- Added Network Explorer components

## [0.62.1] - 2022-11-01

### Fixed

- Make sure filelog receiver uses file_storage for checkpointing (#567)

## [0.62.0] - 2022-10-28

### Changed

- Upgrade splunk-otel-collector image to 0.62.0 (#573)

## [0.61.0] - 2022-10-07

### Changed

- Increase number of queue consumers in the gateway configuration (#554)
- Upgrade splunk-otel-collector image to 0.61.0 (#556)

## [0.59.0] - 2022-09-17

### Added

- A way to provide a custom image for init container patching host log directories (#534, #535)

### Changed

- Upgrade splunk-otel-collector image to 0.59.1 (#536)
  - [BREAKING CHANGE] Datatype of `filelog.force_flush_period` and `filelog.poll_interval` were
    changed back from map to string due to upstream changes.
    See [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0580-0590)

## [0.58.0] - 2022-08-24

### Changed

- Make Openshift SecurityContextConstraints more restrictive (#513)
- Upgrade splunk-otel-collector image to 0.58.0 (#518)
  - [BREAKING CHANGE] Datatype of `filelog.force_flush_period` and `filelog.poll_interval` were
    changed from string to map due to upstream changes.
    See [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0571-to-0580)

## [0.57.1] - 2022-08-05

### Changed

- Do not send clusterReceiver metrics through gateway (#491)

## [0.57.0] - 2022-08-05

### Changed

- Upgrade splunk-otel-collector image to 0.57.0 (#504)

## [0.56.0] - 2022-07-27

### Changed

- Removed unnecessary change of group ownership in chmod initContainer (#486)
- Upgrade splunk-otel-collector image to 0.56.0 (#501)

## [0.55.0] - 2022-07-19

### Changed

- Upgrade splunk-otel-collector image to 0.55.0 (#485)

## [0.54.2] - 2022-07-19

### Changed

- The receiver.k8sclusterreceiver.reportCpuMetricsAsDouble feature gate has been removed (#487)
  - If you are using this feature gate, then see the [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0540-to-0550)
- Remove high cardinality fields from k8s events: (#484)
  - k8s.event.start_time
  - k8s.event.name
  - k8s.event.uid

### Fixed

- Make sure that logs are enabled to send k8s events (#481)
- Make sure that "sourcetype" field is always set on k8s events (#483)

## [0.54.1] - 2022-07-01

### Fixed

- Fix failing cluster receiver with enabled profiling and disabled logs (#480)

## [0.54.0] - 2022-06-29

### Changed

- OTel Kubernetes receiver is now used for events collection instead of Signalfx events receiver (#478)
- Upgrade splunk-otel-collector image to 0.54.0 (#479)

### Fixed

- Fix recombining of oversized log records generated by CRI-O and containerd engines (#475)

## [0.53.2] - 2022-06-23

### Fixed

- Fix bug where clusterReceiver splunk_hec exporter is enabled but configured not to send o11y logs (#471)

## [0.53.1] - 2022-06-22

### Added

- A recombine operator for OTel logs collection to reconstruct multiline logs on docker engine (#467)

### Changed

- Scrape /proc/self/mountinfo in agent pods to avoid incorrect stat attempts (#467)
- Upgrade splunk-otel-collector image to 0.53.1 (#468)

## [0.53.0] - 2022-06-17

### Changed

- Upgrade splunk-otel-collector image to 0.53.0 (#466)

### Added

- Add `splunkPlatform.retryOnFailure` and `splunkPlatform.sendingQueue` config options to values.yaml (#460)

## [0.52.0] - 2022-06-07

### Changed

- Upgrade splunk-otel-collector image to 0.52.2 (#463)

## [0.51.0] - 2022-05-24

### Added

- Add troubleshooting documentation for incompatible Kubernetes and container runtime issues (#452)

### Fixed

- Fix native OTel logs collection where 0 length logs cause errors after the 0.29.0 opentelemetry-logs-library changes in 0.49.0 (#451)

### Changed

- Upgrade splunk-otel-collector image to 0.51.0 (#453)

## [0.50.0] - 2022-05-03

### Fixed

- Add gateway support for Host Logs (#437)
- Make sure that logs or profiling data is sent only when it's enabled (#444)
- Fix native OTel logs collection broken after the 0.29.0 opentelemetry-logs-library changes in 0.49.0 release (#448)

### Changed

- Upgrade splunk-otel-collector image to 0.50.0 (#449)

## [0.49.0] - 2022-04-28

### Changed

- Migrate filelog operators to follow opentelemetry-log-collection v0.29.0 changes (#436, #441)
  - [BREAKING CHANGE] Several breaking changes were made that affect the
    filelog, syslog, tcplog, and journald receivers. Any use of the
    extraFileLogs config, logsCollection.containers.extraOperators config,
    and affected receivers in a custom manner should be reviewed. See
    [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0480-to-0490)

- The receiver.k8sclusterreceiver.reportCpuMetricsAsDouble feature gate is now enabled by default (#487)
  - [BREAKING CHANGE] The Splunk Otel Collector has a feature gate to enable a
    bug fix that makes the k8sclusterreceiver emit a few Kubernetes cpu
    metrics differently to properly adhere to OpenTelemetry specifications. See
    [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0480-to-0490)

- Upgrade splunk-otel-collector image to 0.49.0 (#442)

## [0.48.0] - 2022-04-13

### Changed

- Upgrade splunk-otel-collector image to 0.48.0 (#434)

## [0.47.1] - 2022-03-31

### Fixed

- Bug where the k8sclusterreceiver emits a few Kubernetes cpu metrics improperly (#419)
  - [BREAKING CHANGE] The Splunk Otel Collector added a feature gate to enable a
    bug fix that makes the k8sclusterreceiver emit a few Kubernetes cpu
    metrics differently to properly adhere to OpenTelemetry specifications.
    See [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0470-to-0471)

## [0.47.0] - 2022-03-30

### Changed

- Upgrade splunk-otel-collector image to 0.47.1 (#422)

## [0.46.0] - 2022-03-17

### Added

- Add support for otelcol feature gates to the agent, clusterReceiver, and gateway (#410)

### Changed

- Upgrade splunk-otel-collector image to 0.46.0 (#413)

## [0.45.0] - 2022-03-10

### Changed

- Upgrade splunk-otel-collector image to 0.45.0 (#407)
- [BREAKING CHANGE] Use newer batch and autoscaling APIs in the Kubernetes
  cluster receiver (#433). The Kubernetes cluster receiver will not be able to
  collect all the metrics it previously did for Kubernetes clusters with
  versions below 1.21 or Openshift clusters with versions below 4.8.
  See [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0441-to-0450)

### Fixed

- Bug where Prometheus errors out using default configuration on EKS and GKE (#401, #405)

## [0.44.1] - 2022-03-08

### Fixed

- Add environment processor to metrics pipeline when sending metrics to Splunk Platform (#399)

## [0.44.0] - 2022-03-03

### Added

- Control plane metrics support: etcd (#384)

## [0.43.5] - 2022-03-02

### Fixed

- Add missing splunk-otel-collector secret to gateway and cluster receiver deployment (#390)

## [0.43.4] - 2022-02-25

### Changed

- [BREAKING CHANGE] Set `profilingEnabled` to default false (#388)

## [0.43.3] - 2022-02-24

### Added

- Added support to collect control plane component metrics; controller-manager, coredns, proxy, scheduler (#383)

### Fixed

- Explicitly set match_type parameter in filter processor (#385)
- Truncate eks/fargate cluster receiver StatefulSet names (#386)

## [0.43.2] - 2022-02-02

### Added

- Support of profiling data for Splunk Observability (#376)

### Changed

- [BREAKING CHANGE] OTel Collector Agent now overrides host and cloud attributes
  of logs, metrics and traces that are sent through it (#375). See [upgrade
  guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0431-to-0432)

## [0.43.1] - 2022-02-01

### Added

- `eks/fargate` distribution (#346)

## [0.43.0] - 2022-01-27

### Changed

- Upgrade splunk-otel-collector image to 0.43.0 (#370)

## [0.42.0] - 2022-01-25

### Added

- Journald logs collection (#290)
- Automatic discovery and metrics collection from the Kubernetes API server
  control plane component (#355)
- Native OTel logs collection from the Windows worker nodes (#361)
- Option to disable helm hook for custom secret validation (#350)

### Changed

- Upgrade splunk-otel-collector image to 0.42.0 (#367)

### Fixed

- Double expansion issue splunk-otel-collector (#357). See [upgrade
guideline](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0410-to-0420)
- Schema validation for `image.imagePullSecrets` configuration option (#356)
- Schema validation for `logsCollection.containers.extraOperators` configuration
  option (#356)

### Removed

- Temporary helper initContainer for OTel checkpointing log path move (#358)

## [0.41.0] - 2021-12-13

### Added

- Google Kubernetes Engine Autopilot support (#338)

### Changed

- Upgrade splunk-otel-collector image to 0.41.0 (#340)

## [0.40.0] - 2021-12-08

### Changed

- Upgrade splunk-otel-collector image to 0.40.0 (#334)

## [0.39.0] - 2021-11-30

[Upgrade
guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0380-to-0390)

### Changed

- Upgrade splunk-otel-collector image to 0.39.0 (#322)
- [BREAKING CHANGE] Logs collection is now disabled by default for Splunk
  Observability destination (#325)

## [0.38.0] - 2021-11-19

This release completes the addition of content and documentation to easily allow
users to send telemetry data including logs to both Splunk observability and
Splunk platform. This will address use cases of current users of the Splunk
Connect for Kubernetes.

[Updated
README](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/README.md)

[Migration guidelines for Splunk Connect for Kubernetes
users](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/docs/migration-from-sck.md)

[Upgrade guidelines for existing Splunk OpenTelemetry Collector for Kubernetes
users](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0371-to-0380)

### Added

- Field name compatibility for SCK (#258)
- Add initContainer for file operations for running as non root user (#263)
- Helm hook for custom secret validation (#294)
- Add include logs functionality based on pod annotations (#260)
- Support for tailing custom host files (#300)

### Changed

- Extract `container.image.tag` attribute from `container.image.name` (#285)
- Upgrade splunk-otel-collector image to 0.38.1 (#284)
- Upgrade fluentd-hec image to 1.2.8 (#281)
- Change secret names according to the GDI specification (#295)
- Make `clusterName` configuration parameter generally required (#296)
- Changed the default checkpoint path to `/var/addon/splunk/otel_pos` (#292)
- Rename "provider" and "distro" parameters to "cloudProvider" and
  "distribution" (#297)
- Changed SplunkPlatform properties to match helm best practices. (#306)
- Rename parameter groups for Splunk OTel Collector components (#301):
  - `otelAgent` -> `agent`
  - `otelCollector` -> `gateway`
  - `otelK8sClusterReceiver` -> `clusterReceiver`
- Rename `stream` log attribute to `log.iostream` (#311)
- Improve configuration for fetching attributes from annotations and labels of
  pods and namespaces (#273)
- Use `main` as default index and disable metrics by default for Splunk
  Platform (#305)

### Fixed

- Splunk Platform client certificates (#286)
- `logsCollection.containers.excludePaths` config parameter (#312)
- Splunk Platform sourcetype precedence order (#276)

### Removed

- Busybox image dependency (#275)
- `extraArgs` config parameter (#313)

## [0.37.1] - 2021-11-01

### Added

- Add initContainer for log checkpoint migration from Fluentd to Otel agent (#253)
- Add index routing for Splunk Enterprise/Cloud customers (#256)

### Fixed

- Fix metrics/logs disabling for Splunk Platform destination (#259)
- Fix kubernetes events in Observability IMM by adding `kubernetes_cluster`
  attribute (#261)

## [0.37.0] - 2021-10-26

[Upgrade
guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0362-to-0370)

### Added

- Add recommended Kubernetes labels (#217)
- Add an option to skip RBAC resources creation (#231)
- Enable container metadata. This gives all collected logs new attributes:
  `container.image.name` and `container.image.tag`. Also the native OTel logs
  collection gets `container.id` attribute that allows container level
  correlation in Splunk Observability Cloud closing a feature parity gap with
  fluentd (#238)
- Add strict values.yaml schema validation (#227, #234, #239)

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
- Upgrade splunk-otel-collector image to 0.37.1 (#237, #249)
- Simplify configuration for switching to native OTel logs collection (#246)

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
  [Upgrade guideline](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0264-to-0270)

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
- Upgraded to Splunk OpenTelemetry Collector 0.26.0
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
