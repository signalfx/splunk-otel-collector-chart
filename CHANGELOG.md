<!-- This file is autogenerate, see CONTRIBUTING.md for instructions to add content. -->
# Changelog

<!-- For unreleased changes, see entries in .chloggen -->
<!-- next version -->

## [0.105.5] - 2024-08-28

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.105.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.105.0).

### 💡 Enhancements 💡

- `all`: Offer an experimental feature gate to mount service tokens in specific containers. ([#1421](https://github.com/signalfx/splunk-otel-collector-chart/pull/1421))
  Kubernetes API access tokens are currently granted via mounting them on all containers of the cluster receiver,
  gateway and daemonset. They are also enabled for the target allocator deployment.
  This experimental change defines how to mount the service account token on specific containers.

## [0.105.4] - 2024-08-26

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.105.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.105.0).

### 🛑 Breaking changes 🛑

- `operator`: Bump java from v1.32.3 to v2.7.0 in helm-charts/splunk-otel-collector/values.yaml ([#1349](https://github.com/signalfx/splunk-otel-collector-chart/pull/1349))
  This is a major upgrade. If you use Java auto-instrumentation please review the [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#01053-01070)

### 🧰 Bug fixes 🧰

- `agent`: Retry indefinitely on filelog receiver if noDropLogsPipeline feature gate is enabled. ([#1410](https://github.com/signalfx/splunk-otel-collector-chart/pull/1410))

## [0.105.3] - 2024-08-21

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.105.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.105.0).

## [0.105.2] - 2024-08-21

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.105.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.105.0).

### 💡 Enhancements 💡

- `operator`: Bump nodejs to v2.11.0 in helm-charts/splunk-otel-collector/values.yaml ([#1352](https://github.com/signalfx/splunk-otel-collector-chart/pull/1352))

## [0.105.1] - 2024-08-20

### 🛑 Breaking changes 🛑

- `agent`: Remove the deprecated OTLP HTTP port 55681 ([#1359](https://github.com/signalfx/splunk-otel-collector-chart/pull/1359))

### 🚀 New components 🚀

- `targetAllocator`: Add support for Target Allocator as part of the Helm chart. ([#689](https://github.com/signalfx/splunk-otel-collector-chart/pull/689))
  Target Allocator is a component of the OpenTelemetry Operator.
  With this addition, the target allocator is deployed to work in coordination with the daemonset of collectors.
  It applies a default configuration applying scrape targets per node.
  By default, the Target Allocator looks for all ServiceMonitor and PodMonitor CRDs across all namespaces.
  This can be tuned by overriding the service account associated with the Target Allocator.


### 💡 Enhancements 💡

- `agent`: Add an experimental feature gate to use exporter batching instead of the batch processor ([#1387](https://github.com/signalfx/splunk-otel-collector-chart/pull/1387))
- `all`: Set automountServiceAccountToken to true explicitly for the chart's defined service accounts. ([#1390](https://github.com/signalfx/splunk-otel-collector-chart/pull/1390))
- `operator`: Bump java to v1.32.3 in helm-charts/splunk-otel-collector/values.yaml ([#1355](https://github.com/signalfx/splunk-otel-collector-chart/pull/1355))

### 🧰 Bug fixes 🧰

- `agent`: Remove apparmor pod annotation by enabled default ([#1378](https://github.com/signalfx/splunk-otel-collector-chart/pull/1378))

## [0.105.0] - 2024-07-30

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.105.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.105.0).

## [0.104.0] - 2024-07-11

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.104.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.104.0).

### 💡 Enhancements 💡

- `operator`: Bump nodejs to v2.9.0 in helm-charts/splunk-otel-collector/values.yaml ([#1337](https://github.com/signalfx/splunk-otel-collector-chart/pull/1337))

## [0.103.0] - 2024-06-27

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.103.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.103.0).

### 💡 Enhancements 💡

- `operator`: Bump dotnet to v1.6.0 in helm-charts/splunk-otel-collector/values.yaml ([#1327](https://github.com/signalfx/splunk-otel-collector-chart/pull/1327))
- `operator`: Bump java to v1.32.2 in helm-charts/splunk-otel-collector/values.yaml ([#1328](https://github.com/signalfx/splunk-otel-collector-chart/pull/1328))

### 🧰 Bug fixes 🧰

- `chart`: Updated Security Context Constraints for OpenShift support to fix formatting issues and add support for the operator service account ([#1325](https://github.com/signalfx/splunk-otel-collector-chart/pull/1325))

## [0.102.0] - 2024-06-06

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.102.1](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.102.1).

### 💡 Enhancements 💡

- `agent`: Add a pod annotation that designates the otel-collector as unconfined for appArmor-protected environments ([#1313](https://github.com/signalfx/splunk-otel-collector-chart/pull/1313))

## [0.101.0] - 2024-05-29

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.101.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.101.0).

### 💡 Enhancements 💡

- `operator`: Bump java to v1.32.1 in helm-charts/splunk-otel-collector/values.yaml ([#1300](https://github.com/signalfx/splunk-otel-collector-chart/pull/1300))

### 🧰 Bug fixes 🧰

- `operator`: Fix issue where SPLUNK_OTEL_AGENT env var was set before custom operator.instrumentation.spec.env env vars ([#1292](https://github.com/signalfx/splunk-otel-collector-chart/pull/1292))

## [0.100.0] - 2024-05-09

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.100.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.100.0).

## [0.99.1] - 2024-05-09

### 💡 Enhancements 💡

- `agent`: Add an option `skipInitContainers` to skip init container setting file ACLs when the `runAsUser` and
  `runAsGroup` are provided. This is useful when the user wants to manage the file ACLs themselves.
  ([#1286](https://github.com/signalfx/splunk-otel-collector-chart/pull/1286))
- `operator`: Bump dotnet to v1.5.0 in helm-charts/splunk-otel-collector/values.yaml ([#1282](https://github.com/signalfx/splunk-otel-collector-chart/pull/1282))

## [0.99.0] - 2024-04-26

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.99.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.99.0).

### 💡 Enhancements 💡

- `operator`: Bump java to v1.32.0 in helm-charts/splunk-otel-collector/values.yaml ([#1231](https://github.com/signalfx/splunk-otel-collector-chart/pull/1231),[#1265](https://github.com/signalfx/splunk-otel-collector-chart/pull/#1265))
- `operator`: Bump nodejs to v2.8.0 in helm-charts/splunk-otel-collector/values.yaml ([#1269](https://github.com/signalfx/splunk-otel-collector-chart/pull/1269))

## [0.98.0] - 2024-04-16

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.98.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.98.0).

### 💡 Enhancements 💡

- `agent`: Enable `retry_on_failure` for journald receiver ([#764](https://github.com/signalfx/splunk-otel-collector-chart/pull/764))
  In case of temporary errors the journald receiver should slow down and retry the log delivery instead of dropping it.

### 🧰 Bug fixes 🧰

- `clusterReceiver`: Added clusterRole for events.k8s.io, without it k8sobjectsreceiver throws an error on startup ([#1238](https://github.com/signalfx/splunk-otel-collector-chart/pull/1238))

## [0.97.0] - 2024-03-28

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.97.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.97.0).

### 💡 Enhancements 💡

- `chart`: Removed memory_ballast property from all the configs ([#1240](https://github.com/signalfx/splunk-otel-collector-chart/pull/1240))
- `operator`: Bump operator to 0.49.1 in helm-charts/splunk-otel-collector/Chart.yaml ([#1221](https://github.com/signalfx/splunk-otel-collector-chart/pull/1221))

## [0.96.0] - 2024-03-12

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.96.1](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.96.1).

### 💡 Enhancements 💡

- `operator`: Bump certmanager to v1.14.4 in helm-charts/splunk-otel-collector/Chart.yaml ([#1205](https://github.com/signalfx/splunk-otel-collector-chart/pull/1205))
- `operator`: Bump java to v1.31.0 in helm-charts/splunk-otel-collector/values.yaml ([#1199](https://github.com/signalfx/splunk-otel-collector-chart/pull/1199))

## [0.95.0] - 2024-03-06

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.95.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.95.0).

### 💡 Enhancements 💡

- `chart`: The request is to add the namespace to the generated templates, for commands like "helm template --namespace mynamespace (...)" ([#1011](https://github.com/signalfx/splunk-otel-collector-chart/pull/1011))

## [0.94.0] - 2024-03-01

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.94.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.94.0).

### 💡 Enhancements 💡

- `operator`: Bump certmanager to v1.14.3 in helm-charts/splunk-otel-collector/Chart.yaml ([#1182](https://github.com/signalfx/splunk-otel-collector-chart/pull/1182))
- `operator`: Bump java to v1.30.3 in helm-charts/splunk-otel-collector/values.yaml ([#1188](https://github.com/signalfx/splunk-otel-collector-chart/pull/1188))
- `operator`: Bump nodejs to v2.7.1 in helm-charts/splunk-otel-collector/values.yaml ([#1180](https://github.com/signalfx/splunk-otel-collector-chart/pull/1180))

### 🧰 Bug fixes 🧰

- `clusterReceiver`: Bring back the default translations for kubelet metrics in EKS Fargate ([#1174](https://github.com/signalfx/splunk-otel-collector-chart/pull/1174))
- `agent`: Remove a post-delete hook which targeted one a single node for reverting file ACLs. ([#1175](https://github.com/signalfx/splunk-otel-collector-chart/pull/1175))
  The removed hook was intended to undo the ACLs set on log directores when
  runAsUser and runAsGroup are provided. An initContainer run as root-user updates
  the permissions of log directories to allow read access to the provided uid/gid.
  But there is no graceful way to revert these ACLs on each node as part of the
  chart uninstallation process.


## [0.93.3] - 2024-02-21

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.93.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.93.0).

### 🛑 Breaking changes 🛑

- `networkExplorer`: The `networkExplorer` option was deprecated in 0.88.0. It is now entirely removed. ([#1156](https://github.com/signalfx/splunk-otel-collector-chart/pull/1156))

### 🧰 Bug fixes 🧰

- `agent`: Fix GKE Autopilot deployment ([#1171](https://github.com/signalfx/splunk-otel-collector-chart/pull/1171))

## [0.93.2] - 2024-02-15

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.93.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.93.0).

### 💡 Enhancements 💡

- `operator`:  Improve Auto-instrumentation Configurations ([#1166](https://github.com/signalfx/splunk-otel-collector-chart/pull/1166))

## [0.93.1] - 2024-02-14

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.93.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.93.0).

### 💡 Enhancements 💡

- `agent`: Change the default sending queue size from 5000 to 1000. ([#1157](https://github.com/signalfx/splunk-otel-collector-chart/pull/1157))
- `operator`: Bump dotnet to v1.4.0 in helm-charts/splunk-otel-collector/values.yaml ([#1158](https://github.com/signalfx/splunk-otel-collector-chart/pull/1158))
- `operator`: Bump certmanager to v1.14.2 in helm-charts/splunk-otel-collector/Chart.yaml ([#1152](https://github.com/signalfx/splunk-otel-collector-chart/pull/1152))
  - ⚠️ Known Issue fixed in certmanager v1.14.2
    - In cert-manager [v1.14.1](https://github.com/cert-manager/cert-manager/releases/tag/v1.14.1), the CA and SelfSigned issuers issue certificates with SANs set to non-critical even when the subject is empty. It incorrectly copies the critical field from the CSR.
    - To avoid this issue, please upgrade directly to version 0.93.1 of this chart when utilizing `certmanager.enabled=true`, thereby bypassing affected versions v0.92.1 and v0.93.0.

### 🧰 Bug fixes 🧰

- `chart`: Remove networkExplorer deprecation note that can cause the chart installation to fail ([#1162](https://github.com/signalfx/splunk-otel-collector-chart/pull/1162))

## [0.93.0] - 2024-02-08

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.93.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.93.0).

### 🛑 Breaking changes 🛑

- `internal metrics`: Stop reporting metrics sent by the batch processor ([#1147](https://github.com/signalfx/splunk-otel-collector-chart/pull/1147))

### 💡 Enhancements 💡

- `operator`: Bump java to v1.30.1 in helm-charts/splunk-otel-collector/values.yaml ([#1139](https://github.com/signalfx/splunk-otel-collector-chart/pull/1139))
- `operator`: Bump nodejs to v2.7.0 in helm-charts/splunk-otel-collector/values.yaml ([#1143](https://github.com/signalfx/splunk-otel-collector-chart/pull/1143))

## [0.92.1] - 2024-02-06

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.92.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.92.0).

### 💡 Enhancements 💡

- `operator`: Bump certmanager to v1.14.1 in helm-charts/splunk-otel-collector/Chart.yaml ([#1140](https://github.com/signalfx/splunk-otel-collector-chart/pull/1140))

### 🧰 Bug fixes 🧰

- `chart`: Fix Helm chart incorrectly handling Kubernetes versions containing a "+" character, causing deployment errors for PodDisruptionBudget in certain environments ([#1144](https://github.com/signalfx/splunk-otel-collector-chart/pull/1144))
- `collector`: Fix template function to be able to convert non-integer memory limit values ([#1128](https://github.com/signalfx/splunk-otel-collector-chart/pull/1128))
- `operator`: Fix issue where the collector agent exporter endpoint used in operator .NET and Python auto-instrumentation was missing the proper IP address ([#1129](https://github.com/signalfx/splunk-otel-collector-chart/pull/1129))

## [0.92.0] - 2024-01-23

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.92.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.92.0).

### 💡 Enhancements 💡

- `operator`: Use the Splunk Distribution of OpenTelemetry .NET docker image by default when auto-instrumenting with the operator ([#1098](https://github.com/signalfx/splunk-otel-collector-chart/pull/1098))
- `other`: Added support for rerouting metrics index with pod/namespace annotations ([#1053](https://github.com/signalfx/splunk-otel-collector-chart/pull/1053))
- `chart`: Allows to set the hostNetwork parameter in chart ([#1014](https://github.com/signalfx/splunk-otel-collector-chart/pull/1014))
- `operator`: Bump dotnet to v1.3.0 in helm-charts/splunk-otel-collector/values.yaml ([#1121](https://github.com/signalfx/splunk-otel-collector-chart/pull/1121))
- `operator`: Bump operator to 0.46.0 in helm-charts/splunk-otel-collector/Chart.yaml ([#1116](https://github.com/signalfx/splunk-otel-collector-chart/pull/1116),[#1124](https://github.com/signalfx/splunk-otel-collector-chart/pull/#1124))

## [0.91.1] - 2024-01-11

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.91.3](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.91.3).

### 💡 Enhancements 💡

- `agent`: linux: Adopt discovery mode in agent and provide agent.discovery.properties value mapping ([#1108](https://github.com/signalfx/splunk-otel-collector-chart/pull/1108))
- `chart`: Make clusterName optional in EKS and GKE ([#1056](https://github.com/signalfx/splunk-otel-collector-chart/pull/1056),[#1067](https://github.com/signalfx/splunk-otel-collector-chart/pull/#1067))
- `operator`: Bump certmanager to v1.13.3 in helm-charts/splunk-otel-collector/Chart.yaml ([#1085](https://github.com/signalfx/splunk-otel-collector-chart/pull/1085))
- `operator`: Bump nodejs to v2.6.1 in helm-charts/splunk-otel-collector/values.yaml ([#1094](https://github.com/signalfx/splunk-otel-collector-chart/pull/1094))
- `operator`: Bump operator to 0.44.2 in helm-charts/splunk-otel-collector/Chart.yaml ([#1084](https://github.com/signalfx/splunk-otel-collector-chart/pull/1084))

### 🧰 Bug fixes 🧰

- `agent`: Change the default directory of the journald receiver ([#1110](https://github.com/signalfx/splunk-otel-collector-chart/pull/1110))

## [0.91.0] - 2023-12-12

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.91.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.91.0).

### 🛑 Breaking changes 🛑

- `networkExplorer`: Remove networkExplorer from helm chart ([#1076](https://github.com/signalfx/splunk-otel-collector-chart/pull/1076))
  Network explorer is no longer part of this helm chart and should be installed separately.
  See https://github.com/signalfx/splunk-otel-collector-chart/blob/main/docs/advanced-configuration.md#using-opentelemetry-ebpf-helm-chart-with-splunk-opentelemetry-collector-for-kubernetes
  for more details.

### 💡 Enhancements 💡

- `operator`: Bump nodejs to v2.6.0 in helm-charts/splunk-otel-collector/values.yaml ([#1080](https://github.com/signalfx/splunk-otel-collector-chart/pull/1080))

## [0.90.1] - 2023-12-08

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.90.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.90.0).

### 🧰 Bug fixes 🧰

- `agent`: Fix GKE Autopilot deployment ([#1071](https://github.com/signalfx/splunk-otel-collector-chart/pull/1071))

## [0.90.0] - 2023-12-07

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.90.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.90.0).

### 💡 Enhancements 💡

- `operator`: Bump java to v1.30.0 in helm-charts/splunk-otel-collector/values.yaml ([#1064](https://github.com/signalfx/splunk-otel-collector-chart/pull/1064))
- `operator`: Bump version of the operator subchart to 0.43.0 ([#1049](https://github.com/signalfx/splunk-otel-collector-chart/pull/1049))

## [0.88.0] - 2023-11-22

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.88.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.88.0).

### 🚩 Deprecations 🚩

- `networkExplorer`: Deprecate networkExplorer in favor of the upstream OpenTelemetry eBPF helm chart. ([#1026](https://github.com/signalfx/splunk-otel-collector-chart/pull/1026))

### 💡 Enhancements 💡

- `agent`: Remove the use of the `max_connections` configuration key, use `max_idle_conns_per_host` instead. ([#1034](https://github.com/signalfx/splunk-otel-collector-chart/pull/1034))
- `operator`: Bump java to v1.29.1 in helm-charts/splunk-otel-collector/values.yaml ([#1042](https://github.com/signalfx/splunk-otel-collector-chart/pull/1042))
- `operator`: Bump nodejs to v2.5.1 in helm-charts/splunk-otel-collector/values.yaml ([#1040](https://github.com/signalfx/splunk-otel-collector-chart/pull/1040))
- `operator`: Bump operator to 0.42.3 in helm-charts/splunk-otel-collector/Chart.yaml ([#1025](https://github.com/signalfx/splunk-otel-collector-chart/pull/1025))

## [0.87.0] - 2023-11-15

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.87.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.87.0).

### 💡 Enhancements 💡

- `agent, fluentd`: Allow users to override `enable_stat_watcher` and `refresh_interval` for tail plugin from values.yaml ([#982](https://github.com/signalfx/splunk-otel-collector-chart/pull/982))
- `agent`: Add combineWith field to multiline configuration ([#756](https://github.com/signalfx/splunk-otel-collector-chart/pull/756))
- `operator`: Bump certmanager to v1.13.2 in helm-charts/splunk-otel-collector/Chart.yaml ([#1007](https://github.com/signalfx/splunk-otel-collector-chart/pull/1007))
- `operator`: Bump operator to 0.41.0 in helm-charts/splunk-otel-collector/Chart.yaml ([#985](https://github.com/signalfx/splunk-otel-collector-chart/pull/985))

### 🧰 Bug fixes 🧰

- `chart`: Remove by default empty allowedFlexVolumes ([#981](https://github.com/signalfx/splunk-otel-collector-chart/pull/981))
  Removed the allowedFlexVolumes empty field since it does not provide any default additional benefits for the users.

## [0.86.1] - 2023-10-13

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.86.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.86.0).

### 💡 Enhancements 💡

- `operator`: use splunk-otel-js as Docker image for node.js auto-instrumentation ([#967](https://github.com/signalfx/splunk-otel-collector-chart/pull/967))

## [0.86.0] - 2023-10-11

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.86.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.86.0).

### 💡 Enhancements 💡

- `agent`: Drop time attribute used during parsing the time from log record, so it is not reported as an extra field. ([#912](https://github.com/signalfx/splunk-otel-collector-chart/pull/912))
- `agent`: Change the default logs collection engine (`logsEngine`) to the native OpenTelemetry logs collection (`otel`) ([#934](https://github.com/signalfx/splunk-otel-collector-chart/pull/934))
  If you want to keep using Fluentd sidecar for the logs collection, set `logsEngine' to 'fluentd` in your values.yaml
- `operator`: Bump certmanager to v1.13.1 in helm-charts/splunk-otel-collector/Chart.yaml ([#941](https://github.com/signalfx/splunk-otel-collector-chart/pull/941))
- `operator`: Bump operator to 0.39.1 in helm-charts/splunk-otel-collector/Chart.yaml ([#940](https://github.com/signalfx/splunk-otel-collector-chart/pull/940))
- `chart`: Add support for OpenTelemetry CHANGELOG.md generator tool, see [chloggen](https://github.com/open-telemetry/opentelemetry-operator/tree/main/.chloggen) ([#923](https://github.com/signalfx/splunk-otel-collector-chart/pull/923))

### 🧰 Bug fixes 🧰

- `networkExplorer`: delete deprecated Pod Security Policy from networkExplorer templates ([#896](https://github.com/signalfx/splunk-otel-collector-chart/pull/896))
- `operator`: Fix Operator helm template issues ([#938](https://github.com/signalfx/splunk-otel-collector-chart/pull/938))
  Resolves smaller issues in the Helm template related to the of non-default Operator auto-instrumentation values, which could trigger deployment failures

## [0.85.0] - 2023-09-19

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.85.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.85.0).

### 🚀 New components 🚀

- Add `component` label to daemonset pods and optional k8s service for agent daemonset which can be enabled with `agent.service.enabled` config. [#740](https://github.com/signalfx/splunk-otel-collector-chart/pull/740)

### 💡 Enhancements 💡

- Update Splunk Fluend HEC docker image to v1.3.3 [#924](https://github.com/signalfx/splunk-otel-collector-chart/pull/924)
- Add ability to update and track operator auto-instrumentation images [#917](https://github.com/signalfx/splunk-otel-collector-chart/pull/917)
  - [BREAKING CHANGE] Refactored auto-instrumentation image definition from operator.instrumentation.spec.{library}.image
    to operator.instrumentation.spec.{library}.repository and operator.instrumentation.spec.{library}.tag.
    See [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0840-0850)
- Upgrade the Splunk OpenTelemetry Collector for Kubernetes dependencies [#32](https://github.com/signalfx/splunk-otel-collector-chart/pull/932),[#929](https://github.com/signalfx/splunk-otel-collector-chart/pull/929)
  - opentelemetry-operator upgraded to [v0.37.1](https://github.com/open-telemetry/opentelemetry-helm-charts/releases/tag/opentelemetry-operator-0.37.1)
  - Java auto-instrumentation upgraded from 1.28.0 to [1.28.0](https://github.com/signalfx/splunk-otel-java/releases/tag/v1.28.0)

## [0.84.0] - 2023-09-11

### 💡 Enhancements 💡

- Disable `opencensus.resourcetype` resource attribute in k8s_cluster receiver [#914](https://github.com/signalfx/splunk-otel-collector-chart/pull/914)
  - This change does not affect Splunk Observability users since it has already been disabled in the default translation rules of the Signalfx exporter
- Disable signalfx exporter default translations in clusterReceiver deployment [#915](https://github.com/signalfx/splunk-otel-collector-chart/pull/915)
  - This change improves performance of clusterReceiver, but can be breaking if the deprecated signalfx exporter `translation_rules` option is being used
- Upgrade the Splunk OpenTelemetry Collector for Kubernetes dependencies [#919](https://github.com/signalfx/splunk-otel-collector-chart/pull/919),[#909](https://github.com/signalfx/splunk-otel-collector-chart/pull/909)
- opentelemetry-operator upgraded to [v0.37.0](https://github.com/open-telemetry/opentelemetry-helm-charts/releases/tag/opentelemetry-operator-0.37.0)
- cert-manager upgraded to [v1.12.4](https://github.com/cert-manager/cert-manager/releases/tag/v1.12.4)

### 🧰 Bug fixes 🧰

- Enable native OTel logs collection (logsEngine=otel) in GKE Autopilot [#809](https://github.com/signalfx/splunk-otel-collector-chart/pull/809)

### 🚀 New components 🚀

- Configuration of persistent buffering for agent [861](https://github.com/signalfx/splunk-otel-collector-chart/pull/861)
- Add option to disable Openshift SecurityContextConstraint resource [#843](https://github.com/signalfx/splunk-otel-collector-chart/pull/843)

## [0.83.0] - 2023-08-18

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.83.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.83.0).

### 💡 Enhancements 💡

- Upgrade the Splunk OpenTelemetry Collector for Kubernetes dependencies [#885](https://github.com/signalfx/splunk-otel-collector-chart/pull/885),[#876](https://github.com/signalfx/splunk-otel-collector-chart/pull/876)
  - opentelemetry-operator upgraded to [v0.35.0](https://github.com/open-telemetry/opentelemetry-helm-charts/releases/tag/opentelemetry-operator-0.35.0)
  - cert-manager upgraded to [v1.12.3](https://github.com/cert-manager/cert-manager/releases/tag/v1.12.3)

### 🧰 Bug fixes 🧰

- Fix for secret name which now respects the same overrides as other resources in the chart [#873](https://github.com/signalfx/splunk-otel-collector-chart/pull/873)
- Update the secret validation hook pod to use imagePullSecrets instead of possible non-existing serviceAccountName [#888](https://github.com/signalfx/splunk-otel-collector-chart/pull/888)

## [0.82.0] - 2023-08-02

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.82.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.82.0).

### 🧰 Bug fixes 🧰

- Use "ContainerAdministrator" user for windows nodes by default [#809](https://github.com/signalfx/splunk-otel-collector-chart/pull/809)

## [0.81.0] - 2023-07-21

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.81.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.81.0).

### 💡 Enhancements 💡

- Upgrade the Splunk OpenTelemetry Collector for Kubernetes dependencies [#856](https://github.com/signalfx/splunk-otel-collector-chart/pull/856),[#858](https://github.com/signalfx/splunk-otel-collector-chart/pull/858)
  - opentelemetry-operator upgraded from 0.32.0 to [0.34.0](https://github.com/open-telemetry/opentelemetry-helm-charts/releases/tag/opentelemetry-operator-0.34.0)
  - Java auto-instrumentation upgraded from 1.24.0 to [1.26.0](https://github.com/signalfx/splunk-otel-java/releases/tag/v1.26.0)

### 🧰 Bug fixes 🧰

- Set cluster_name for host logs too if renameFieldsSck is enabled [#837](https://github.com/signalfx/splunk-otel-collector-chart/pull/837)
- Fix `smartagent` collectd based monitors on read-only filesystems [#839](https://github.com/signalfx/splunk-otel-collector-chart/pull/839)

### 🚀 New components 🚀

- Update PodDisruptionBudgets API version to allow both `policy/v1beta1` and `policy/v1` [#835](https://github.com/signalfx/splunk-otel-collector-chart/pull/835)
- Update clusterrole to allow collector to check for the `aws-auth` configmap in EKS clusters [#840](https://github.com/signalfx/splunk-otel-collector-chart/pull/840)
- Add support to create default Instrumentation for operator based auto-instrumentation [#836](https://github.com/signalfx/splunk-otel-collector-chart/pull/836)

## [0.80.0] - 2023-06-27

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.80.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.80.0).

### 💡 Enhancements 💡

- Add `service.name` resource attribute to logs if `autodetect.istio` is enabled using transform processor. This change
  removes the limitation of `service.name` attribute being available only with logsEngine=fluentd.
  [#823](https://github.com/signalfx/splunk-otel-collector-chart/pull/823)
- Upgrade the Splunk OpenTelemetry Collector for Kubernetes dependencies [#828](https://github.com/signalfx/splunk-otel-collector-chart/pull/828)
  - cert-manager upgraded from 1.11.1 to [1.12.2](https://github.com/cert-manager/cert-manager/releases/tag/v1.12.2)
  - opentelemetry-operator upgraded from 0.28.0 to [0.32.0](https://github.com/open-telemetry/opentelemetry-helm-charts/releases/tag/opentelemetry-operator-0.32.0)
- Update the log level for metric scrape failures of the smartagent/kubernetes-proxy receiver from error to debug when distribution='' [#832](https://github.com/signalfx/splunk-otel-collector-chart/pull/832)

## [0.79.1] - 2023-06-22

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.79.1](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.79.1).

### 🧰 Bug fixes 🧰

- Fix cri-o log format time layout [#817](https://github.com/signalfx/splunk-otel-collector-chart/pull/817)
- Align the set of default resource attributes added by k8s attributes processor if the gateway is enabled [#820](https://github.com/signalfx/splunk-otel-collector-chart/pull/820)

## [0.79.0] - 2023-06-16

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.79.0](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.79.0).

### 🚀 New components 🚀

- Adopt new control-plane node toleration [#814](https://github.com/signalfx/splunk-otel-collector-chart/pull/814)

### 💡 Enhancements 💡

- Update the Kubernetes Proxy monitor for OpenShift clusters to start using secure ports 9101 or 29101 with authentication [#810](https://github.com/signalfx/splunk-otel-collector-chart/pull/810)

## [0.78.0] - 2023-06-07

This Splunk OpenTelemetry Collector for Kubernetes release adopts the [Splunk OpenTelemetry Collector v0.78.1](https://github.com/signalfx/splunk-otel-collector/releases/tag/v0.78.1).

### 🚀 New components 🚀

- Adopt Openshift distribution in Network Explorer [#804](https://github.com/signalfx/splunk-otel-collector-chart/pull/804)

## [0.77.0] - 2023-06-05

### 🚀 New components 🚀

- Add `serviceAccountName` to secret validation hook pod [#781](https://github.com/signalfx/splunk-otel-collector-chart/pull/781)
- Allow enabling profiling only for Observability [#788](https://github.com/signalfx/splunk-otel-collector-chart/pull/788)
- Add storage to filelog receiver for extraFileLog [#755](https://github.com/signalfx/splunk-otel-collector-chart/pull/755)
- Add Openshift support for Network-Explorer [804](https://github.com/signalfx/splunk-otel-collector-chart/pull/804)

### 💡 Enhancements 💡

- Avoid `runAsUser` in SecurityContext for Windows [#797](https://github.com/signalfx/splunk-otel-collector-chart/pull/797)

## [0.76.0] - 2023-05-04

### 🚀 New components 🚀

- Option to use lightprometheus receiver through a feature gate for metrics collection from discovered Prometheus endpoints [757](https://github.com/signalfx/splunk-otel-collector-chart/pull/757)

### 💡 Enhancements 💡

- Add `logsCollection.containers.maxRecombineLogSize` config option with default 1Mb value which is applied
  to `max_log_size` option of the multiline recombine operators
- Move `extraOperators` above `multilineConfig` in _otel_agent.tpl, as `extraOperators` gets
  skipped when we use both `multilineConfig` AND `extraOperators` in values.yaml
- Enable retry mechanism in filelog receiver to avoid dropping logs on backpressure from the downstream
  pipeline components [#764](https://github.com/signalfx/splunk-otel-collector-chart/pull/764)
- Drop excessive istio attributes to avoid running into the dimensions limit when scraping istio metrics is enabled [765](https://github.com/signalfx/splunk-otel-collector-chart/pull/765)

### 🧰 Bug fixes 🧰

- Fix k8s.cluster.name resource attribute for logs in GCP [#771](https://github.com/signalfx/splunk-otel-collector-chart/pull/771)

## [0.75.0] - 2023-04-17

### 💡 Enhancements 💡

- Update the Kubernetes scheduler monitor to stop using insecure port 10251 and start using secure port 10259 with authentication [#711](https://github.com/signalfx/splunk-otel-collector-chart/pull/711)
- Upgrade splunk-otel-collector image to 0.75.0

### 🧰 Bug fixes 🧰

- Sending request timeouts in k8s cluster receiver deployment on big k8s clusters [#717](https://github.com/signalfx/splunk-otel-collector-chart/pull/717)
- Properly handle enableMetrics in Network Explorer Reducer template [#724](https://github.com/signalfx/splunk-otel-collector-chart/pull/724)

### 🚀 New components 🚀

- Add support for Operator based Java auto-instrumentation [#701](https://github.com/signalfx/splunk-otel-collector-chart/pull/701)
- Add experimental support for deploying OpenTelemetry Operator as a subchart [#691](https://github.com/signalfx/splunk-otel-collector-chart/pull/691)
- Improve documentation about providing tokens as Kubernetes secrets [#707](https://github.com/signalfx/splunk-otel-collector-chart/pull/691)
- Expose `idle_conn_timeout` on the splunk HEC exporters [#728](https://github.com/signalfx/splunk-otel-collector-chart/pull/728)
- Add `logsCollection.containers.maxRecombineLogSize` config option with default 1Mb value which is applied
  to `max_log_size` option of the default recombine operators [#713](https://github.com/signalfx/splunk-otel-collector-chart/pull/713)

## [0.72.0] - 2023-03-09

### 🚀 New components 🚀

- Add functional test coverage for Network Explorer metrics [#684](https://github.com/signalfx/splunk-otel-collector-chart/pull/684)
- Apply the same resources to init containers as allocated to the otel agent container [#690](https://github.com/signalfx/splunk-otel-collector-chart/pull/690)

## [0.71.0] - 2023-03-01

### 🚀 New components 🚀

- Added examples for supported Kubernetes distributions and Kubernetes clusters with windows nodes ([#663](https://github.com/signalfx/splunk-otel-collector-chart/pull/663))
- Refactored the examples and rendered directories into one for better usability ([#658](https://github.com/signalfx/splunk-otel-collector-chart/pull/658))

### 💡 Enhancements 💡

- Docker metadata turned off by default ([#655](https://github.com/signalfx/splunk-otel-collector-chart/pull/665))

### 🧰 Bug fixes 🧰

- Translation of `k8s.pod.labels.app` attribute to SCK format ([#660](https://github.com/signalfx/splunk-otel-collector-chart/pull/660))

## [0.70.0] - 2023-01-31

### 🚀 New components 🚀

- Support sending traces via Splunk HEC exporter ([#629](https://github.com/signalfx/splunk-otel-collector-chart/pull/629) - thanks to @mr-miles)

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.70.0, skipping 0.69.0 release ([#653](https://github.com/signalfx/splunk-otel-collector-chart/pull/653))

### 🧰 Bug fixes 🧰

- Fix invalid OpenShift SecurityContextConstraints template ([#652](https://github.com/signalfx/splunk-otel-collector-chart/pull/652))
- Limit `clusterReceiver.eventsEnabled` deprecation warning to feature users ([#648](https://github.com/signalfx/splunk-otel-collector-chart/pull/648))
- Fix noop validation for missing platform info ([#649](https://github.com/signalfx/splunk-otel-collector-chart/pull/649))

## [0.68.0] - 2023-01-25

### 🚀 New components 🚀

- Allow to overwrite default SecurityContextConstraints rules with values.yaml file ([#643](https://github.com/signalfx/splunk-otel-collector-chart/pull/643))

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.68.1 ([#640](https://github.com/signalfx/splunk-otel-collector-chart/pull/640))

### 🧰 Bug fixes 🧰

- Default recombine operator for the docker container engine ([#627](https://github.com/signalfx/splunk-otel-collector-chart/pull/627))
- Added acl to journald log directory ([#639](https://github.com/signalfx/splunk-otel-collector-chart/pull/639))

## [0.67.0] - 2022-12-19

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.67.0 ([#612](https://github.com/signalfx/splunk-otel-collector-chart/pull/612))

### 🧰 Bug fixes 🧰

- Make sure the daemonset can start in GKE Autopiot ([#608](https://github.com/signalfx/splunk-otel-collector-chart/pull/608))
- Make containerd engine default in for fluentd logs and use always use it in GKE Autopiot ([#609](https://github.com/signalfx/splunk-otel-collector-chart/pull/609))
- Temporary disable compression in Splunk Observability logs exporter until
  0.68.0 to workaround a compression bug ([#610](https://github.com/signalfx/splunk-otel-collector-chart/pull/610))

## [0.66.1] - 2022-12-08

### 🧰 Bug fixes 🧰

- Fixed network explorer image pull secrets

## [0.66.0] - 2022-12-06

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.66.0 ([#593](https://github.com/signalfx/splunk-otel-collector-chart/pull/593))

## [0.64.0] - 2022-11-22

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.64.0 ([#589](https://github.com/signalfx/splunk-otel-collector-chart/pull/589))

### 🧰 Bug fixes 🧰

## [0.62.2] - 2022-11-21

- Added Network Explorer components

## [0.62.1] - 2022-11-01

### 🧰 Bug fixes 🧰

- Make sure filelog receiver uses file_storage for checkpointing ([#567](https://github.com/signalfx/splunk-otel-collector-chart/pull/567))

## [0.62.0] - 2022-10-28

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.62.0 ([#573](https://github.com/signalfx/splunk-otel-collector-chart/pull/573))

## [0.61.0] - 2022-10-07

### 💡 Enhancements 💡

- Increase number of queue consumers in the gateway configuration ([#554](https://github.com/signalfx/splunk-otel-collector-chart/pull/554))
- Upgrade splunk-otel-collector image to 0.61.0 ([#556](https://github.com/signalfx/splunk-otel-collector-chart/pull/556))

## [0.59.0] - 2022-09-17

### 🚀 New components 🚀

- A way to provide a custom image for init container patching host log directories ([#534](https://github.com/signalfx/splunk-otel-collector-chart/pull/534),[#535](https://github.com/signalfx/splunk-otel-collector-chart/pull/#535))

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.59.1 ([#536](https://github.com/signalfx/splunk-otel-collector-chart/pull/536))
  - [BREAKING CHANGE] Datatype of `filelog.force_flush_period` and `filelog.poll_interval` were
    changed back from map to string due to upstream changes.
    See [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0580-0590)

## [0.58.0] - 2022-08-24

### 💡 Enhancements 💡

- Make Openshift SecurityContextConstraints more restrictive ([#513](https://github.com/signalfx/splunk-otel-collector-chart/pull/513))
- Upgrade splunk-otel-collector image to 0.58.0 ([#518](https://github.com/signalfx/splunk-otel-collector-chart/pull/518))
  - [BREAKING CHANGE] Datatype of `filelog.force_flush_period` and `filelog.poll_interval` were
    changed from string to map due to upstream changes.
    See [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0571-to-0580)

## [0.57.1] - 2022-08-05

### 💡 Enhancements 💡

- Do not send clusterReceiver metrics through gateway ([#491](https://github.com/signalfx/splunk-otel-collector-chart/pull/491))

## [0.57.0] - 2022-08-05

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.57.0 ([#504](https://github.com/signalfx/splunk-otel-collector-chart/pull/504))

## [0.56.0] - 2022-07-27

### 💡 Enhancements 💡

- Removed unnecessary change of group ownership in chmod initContainer ([#486](https://github.com/signalfx/splunk-otel-collector-chart/pull/486))
- Upgrade splunk-otel-collector image to 0.56.0 ([#501](https://github.com/signalfx/splunk-otel-collector-chart/pull/501))

## [0.55.0] - 2022-07-19

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.55.0 ([#485](https://github.com/signalfx/splunk-otel-collector-chart/pull/485))

## [0.54.2] - 2022-07-19

### 💡 Enhancements 💡

- The receiver.k8sclusterreceiver.reportCpuMetricsAsDouble feature gate has been removed ([#487](https://github.com/signalfx/splunk-otel-collector-chart/pull/487))
  - If you are using this feature gate, then see the [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0540-to-0550)
- Remove high cardinality fields from k8s events: ([#484](https://github.com/signalfx/splunk-otel-collector-chart/pull/484))
  - k8s.event.start_time
  - k8s.event.name
  - k8s.event.uid

### 🧰 Bug fixes 🧰

- Make sure that logs are enabled to send k8s events ([#481](https://github.com/signalfx/splunk-otel-collector-chart/pull/481))
- Make sure that "sourcetype" field is always set on k8s events ([#483](https://github.com/signalfx/splunk-otel-collector-chart/pull/483))

## [0.54.1] - 2022-07-01

### 🧰 Bug fixes 🧰

- Fix failing cluster receiver with enabled profiling and disabled logs ([#480](https://github.com/signalfx/splunk-otel-collector-chart/pull/480))

## [0.54.0] - 2022-06-29

### 💡 Enhancements 💡

- OTel Kubernetes receiver is now used for events collection instead of Signalfx events receiver ([#478](https://github.com/signalfx/splunk-otel-collector-chart/pull/478))
- Upgrade splunk-otel-collector image to 0.54.0 ([#479](https://github.com/signalfx/splunk-otel-collector-chart/pull/479))

### 🧰 Bug fixes 🧰

- Fix recombining of oversized log records generated by CRI-O and containerd engines ([#475](https://github.com/signalfx/splunk-otel-collector-chart/pull/475))

## [0.53.2] - 2022-06-23

### 🧰 Bug fixes 🧰

- Fix bug where clusterReceiver splunk_hec exporter is enabled but configured not to send o11y logs ([#471](https://github.com/signalfx/splunk-otel-collector-chart/pull/471))

## [0.53.1] - 2022-06-22

### 🚀 New components 🚀

- A recombine operator for OTel logs collection to reconstruct multiline logs on docker engine ([#467](https://github.com/signalfx/splunk-otel-collector-chart/pull/467))

### 💡 Enhancements 💡

- Scrape /proc/self/mountinfo in agent pods to avoid incorrect stat attempts ([#467](https://github.com/signalfx/splunk-otel-collector-chart/pull/467))
- Upgrade splunk-otel-collector image to 0.53.1 ([#468](https://github.com/signalfx/splunk-otel-collector-chart/pull/468))

## [0.53.0] - 2022-06-17

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.53.0 ([#466](https://github.com/signalfx/splunk-otel-collector-chart/pull/466))

### 🚀 New components 🚀

- Add `splunkPlatform.retryOnFailure` and `splunkPlatform.sendingQueue` config options to values.yaml ([#460](https://github.com/signalfx/splunk-otel-collector-chart/pull/460))

## [0.52.0] - 2022-06-07

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.52.2 ([#463](https://github.com/signalfx/splunk-otel-collector-chart/pull/463))

## [0.51.0] - 2022-05-24

### 🚀 New components 🚀

- Add troubleshooting documentation for incompatible Kubernetes and container runtime issues ([#452](https://github.com/signalfx/splunk-otel-collector-chart/pull/452))

### 🧰 Bug fixes 🧰

- Fix native OTel logs collection where 0 length logs cause errors after the 0.29.0 opentelemetry-logs-library changes in 0.49.0 ([#451](https://github.com/signalfx/splunk-otel-collector-chart/pull/451))

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.51.0 ([#453](https://github.com/signalfx/splunk-otel-collector-chart/pull/453))

## [0.50.0] - 2022-05-03

### 🧰 Bug fixes 🧰

- Add gateway support for Host Logs ([#437](https://github.com/signalfx/splunk-otel-collector-chart/pull/437))
- Make sure that logs or profiling data is sent only when it's enabled ([#444](https://github.com/signalfx/splunk-otel-collector-chart/pull/444))
- Fix native OTel logs collection broken after the 0.29.0 opentelemetry-logs-library changes in 0.49.0 release ([#448](https://github.com/signalfx/splunk-otel-collector-chart/pull/448))

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.50.0 ([#449](https://github.com/signalfx/splunk-otel-collector-chart/pull/449))

## [0.49.0] - 2022-04-28

### 💡 Enhancements 💡

- Migrate filelog operators to follow opentelemetry-log-collection v0.29.0 changes ([#436](https://github.com/signalfx/splunk-otel-collector-chart/pull/436),[#441](https://github.com/signalfx/splunk-otel-collector-chart/pull/#441))
  - [BREAKING CHANGE] Several breaking changes were made that affect the
    filelog, syslog, tcplog, and journald receivers. Any use of the
    extraFileLogs config, logsCollection.containers.extraOperators config,
    and affected receivers in a custom manner should be reviewed. See
    [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0480-to-0490)

- The receiver.k8sclusterreceiver.reportCpuMetricsAsDouble feature gate is now enabled by default ([#487](https://github.com/signalfx/splunk-otel-collector-chart/pull/487))
  - [BREAKING CHANGE] The Splunk Otel Collector has a feature gate to enable a
    bug fix that makes the k8sclusterreceiver emit a few Kubernetes cpu
    metrics differently to properly adhere to OpenTelemetry specifications. See
    [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0480-to-0490)

- Upgrade splunk-otel-collector image to 0.49.0 ([#442](https://github.com/signalfx/splunk-otel-collector-chart/pull/442))

## [0.48.0] - 2022-04-13

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.48.0 ([#434](https://github.com/signalfx/splunk-otel-collector-chart/pull/434))

## [0.47.1] - 2022-03-31

### 🧰 Bug fixes 🧰

- Bug where the k8sclusterreceiver emits a few Kubernetes cpu metrics improperly ([#419](https://github.com/signalfx/splunk-otel-collector-chart/pull/419))
  - [BREAKING CHANGE] The Splunk Otel Collector added a feature gate to enable a
    bug fix that makes the k8sclusterreceiver emit a few Kubernetes cpu
    metrics differently to properly adhere to OpenTelemetry specifications.
    See [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0470-to-0471)

## [0.47.0] - 2022-03-30

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.47.1 ([#422](https://github.com/signalfx/splunk-otel-collector-chart/pull/422))

## [0.46.0] - 2022-03-17

### 🚀 New components 🚀

- Add support for otelcol feature gates to the agent, clusterReceiver, and gateway ([#410](https://github.com/signalfx/splunk-otel-collector-chart/pull/410))

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.46.0 ([#413](https://github.com/signalfx/splunk-otel-collector-chart/pull/413))

## [0.45.0] - 2022-03-10

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.45.0 ([#407](https://github.com/signalfx/splunk-otel-collector-chart/pull/407))
- [BREAKING CHANGE] Use newer batch and autoscaling APIs in the Kubernetes
  cluster receiver ([#433](https://github.com/signalfx/splunk-otel-collector-chart/pull/433)). The Kubernetes cluster receiver will not be able to
  collect all the metrics it previously did for Kubernetes clusters with
  versions below 1.21 or Openshift clusters with versions below 4.8.
  See [upgrade guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0441-to-0450)

### 🧰 Bug fixes 🧰

- Bug where Prometheus errors out using default configuration on EKS and GKE ([#401](https://github.com/signalfx/splunk-otel-collector-chart/pull/401),[#405](https://github.com/signalfx/splunk-otel-collector-chart/pull/#405))

## [0.44.1] - 2022-03-08

### 🧰 Bug fixes 🧰

- Add environment processor to metrics pipeline when sending metrics to Splunk Platform ([#399](https://github.com/signalfx/splunk-otel-collector-chart/pull/399))

## [0.44.0] - 2022-03-03

### 🚀 New components 🚀

- Control plane metrics support: etcd ([#384](https://github.com/signalfx/splunk-otel-collector-chart/pull/384))

## [0.43.5] - 2022-03-02

### 🧰 Bug fixes 🧰

- Add missing splunk-otel-collector secret to gateway and cluster receiver deployment ([#390](https://github.com/signalfx/splunk-otel-collector-chart/pull/390))

## [0.43.4] - 2022-02-25

### 💡 Enhancements 💡

- [BREAKING CHANGE] Set `profilingEnabled` to default false ([#388](https://github.com/signalfx/splunk-otel-collector-chart/pull/388))

## [0.43.3] - 2022-02-24

### 🚀 New components 🚀

- Added support to collect control plane component metrics; controller-manager, coredns, proxy, scheduler ([#383](https://github.com/signalfx/splunk-otel-collector-chart/pull/383))

### 🧰 Bug fixes 🧰

- Explicitly set match_type parameter in filter processor ([#385](https://github.com/signalfx/splunk-otel-collector-chart/pull/385))
- Truncate eks/fargate cluster receiver StatefulSet names ([#386](https://github.com/signalfx/splunk-otel-collector-chart/pull/386))

## [0.43.2] - 2022-02-02

### 🚀 New components 🚀

- Support of profiling data for Splunk Observability ([#376](https://github.com/signalfx/splunk-otel-collector-chart/pull/376))

### 💡 Enhancements 💡

- [BREAKING CHANGE] OTel Collector Agent now overrides host and cloud attributes
  of logs, metrics and traces that are sent through it ([#375](https://github.com/signalfx/splunk-otel-collector-chart/pull/375)). See [upgrade
  guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0431-to-0432)

## [0.43.1] - 2022-02-01

### 🚀 New components 🚀

- `eks/fargate` distribution ([#346](https://github.com/signalfx/splunk-otel-collector-chart/pull/346))

## [0.43.0] - 2022-01-27

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.43.0 ([#370](https://github.com/signalfx/splunk-otel-collector-chart/pull/370))

## [0.42.0] - 2022-01-25

### 🚀 New components 🚀

- Journald logs collection ([#290](https://github.com/signalfx/splunk-otel-collector-chart/pull/290))
- Automatic discovery and metrics collection from the Kubernetes API server
  control plane component ([#355](https://github.com/signalfx/splunk-otel-collector-chart/pull/355))
- Native OTel logs collection from the Windows worker nodes ([#361](https://github.com/signalfx/splunk-otel-collector-chart/pull/361))
- Option to disable helm hook for custom secret validation ([#350](https://github.com/signalfx/splunk-otel-collector-chart/pull/350))

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.42.0 ([#367](https://github.com/signalfx/splunk-otel-collector-chart/pull/367))

### 🧰 Bug fixes 🧰

- Double expansion issue splunk-otel-collector ([#357](https://github.com/signalfx/splunk-otel-collector-chart/pull/357)). See [upgrade
  guideline](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0410-to-0420)
- Schema validation for `image.imagePullSecrets` configuration option ([#356](https://github.com/signalfx/splunk-otel-collector-chart/pull/356))
- Schema validation for `logsCollection.containers.extraOperators` configuration
  option ([#356](https://github.com/signalfx/splunk-otel-collector-chart/pull/356))

### Removed

- Temporary helper initContainer for OTel checkpointing log path move ([#358](https://github.com/signalfx/splunk-otel-collector-chart/pull/358))

## [0.41.0] - 2021-12-13

### 🚀 New components 🚀

- Google Kubernetes Engine Autopilot support ([#338](https://github.com/signalfx/splunk-otel-collector-chart/pull/338))

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.41.0 ([#340](https://github.com/signalfx/splunk-otel-collector-chart/pull/340))

## [0.40.0] - 2021-12-08

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.40.0 ([#334](https://github.com/signalfx/splunk-otel-collector-chart/pull/334))

## [0.39.0] - 2021-11-30

[Upgrade
guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0380-to-0390)

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.39.0 ([#322](https://github.com/signalfx/splunk-otel-collector-chart/pull/322))
- [BREAKING CHANGE] Logs collection is now disabled by default for Splunk
  Observability destination ([#325](https://github.com/signalfx/splunk-otel-collector-chart/pull/325))

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

### 🚀 New components 🚀

- Field name compatibility for SCK ([#258](https://github.com/signalfx/splunk-otel-collector-chart/pull/258))
- Add initContainer for file operations for running as non root user ([#263](https://github.com/signalfx/splunk-otel-collector-chart/pull/263))
- Helm hook for custom secret validation ([#294](https://github.com/signalfx/splunk-otel-collector-chart/pull/294))
- Add include logs functionality based on pod annotations ([#260](https://github.com/signalfx/splunk-otel-collector-chart/pull/260))
- Support for tailing custom host files ([#300](https://github.com/signalfx/splunk-otel-collector-chart/pull/300))

### 💡 Enhancements 💡

- Extract `container.image.tag` attribute from `container.image.name` ([#285](https://github.com/signalfx/splunk-otel-collector-chart/pull/285))
- Upgrade splunk-otel-collector image to 0.38.1 ([#284](https://github.com/signalfx/splunk-otel-collector-chart/pull/284))
- Upgrade fluentd-hec image to 1.2.8 ([#281](https://github.com/signalfx/splunk-otel-collector-chart/pull/281))
- Change secret names according to the GDI specification ([#295](https://github.com/signalfx/splunk-otel-collector-chart/pull/295))
- Make `clusterName` configuration parameter generally required ([#296](https://github.com/signalfx/splunk-otel-collector-chart/pull/296))
- Changed the default checkpoint path to `/var/addon/splunk/otel_pos` ([#292](https://github.com/signalfx/splunk-otel-collector-chart/pull/292))
- Rename "provider" and "distro" parameters to "cloudProvider" and
  "distribution" ([#297](https://github.com/signalfx/splunk-otel-collector-chart/pull/297))
- Changed SplunkPlatform properties to match helm best practices. ([#306](https://github.com/signalfx/splunk-otel-collector-chart/pull/306))
- Rename parameter groups for Splunk OTel Collector components ([#301](https://github.com/signalfx/splunk-otel-collector-chart/pull/301)):
  - `otelAgent` -> `agent`
  - `otelCollector` -> `gateway`
  - `otelK8sClusterReceiver` -> `clusterReceiver`
- Rename `stream` log attribute to `log.iostream` ([#311](https://github.com/signalfx/splunk-otel-collector-chart/pull/311))
- Improve configuration for fetching attributes from annotations and labels of
  pods and namespaces ([#273](https://github.com/signalfx/splunk-otel-collector-chart/pull/273))
- Use `main` as default index and disable metrics by default for Splunk
  Platform ([#305](https://github.com/signalfx/splunk-otel-collector-chart/pull/305))

### 🧰 Bug fixes 🧰

- Splunk Platform client certificates ([#286](https://github.com/signalfx/splunk-otel-collector-chart/pull/286))
- `logsCollection.containers.excludePaths` config parameter ([#312](https://github.com/signalfx/splunk-otel-collector-chart/pull/312))
- Splunk Platform sourcetype precedence order ([#276](https://github.com/signalfx/splunk-otel-collector-chart/pull/276))

### Removed

- Busybox image dependency ([#275](https://github.com/signalfx/splunk-otel-collector-chart/pull/275))
- `extraArgs` config parameter ([#313](https://github.com/signalfx/splunk-otel-collector-chart/pull/313))

## [0.37.1] - 2021-11-01

### 🚀 New components 🚀

- Add initContainer for log checkpoint migration from Fluentd to Otel agent ([#253](https://github.com/signalfx/splunk-otel-collector-chart/pull/253))
- Add index routing for Splunk Enterprise/Cloud customers ([#256](https://github.com/signalfx/splunk-otel-collector-chart/pull/256))

### 🧰 Bug fixes 🧰

- Fix metrics/logs disabling for Splunk Platform destination ([#259](https://github.com/signalfx/splunk-otel-collector-chart/pull/259))
- Fix kubernetes events in Observability IMM by adding `kubernetes_cluster`
  attribute ([#261](https://github.com/signalfx/splunk-otel-collector-chart/pull/261))

## [0.37.0] - 2021-10-26

[Upgrade
guidelines](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0362-to-0370)

### 🚀 New components 🚀

- Add recommended Kubernetes labels ([#217](https://github.com/signalfx/splunk-otel-collector-chart/pull/217))
- Add an option to skip RBAC resources creation ([#231](https://github.com/signalfx/splunk-otel-collector-chart/pull/231))
- Enable container metadata. This gives all collected logs new attributes:
  `container.image.name` and `container.image.tag`. Also the native OTel logs
  collection gets `container.id` attribute that allows container level
  correlation in Splunk Observability Cloud closing a feature parity gap with
  fluentd ([#238](https://github.com/signalfx/splunk-otel-collector-chart/pull/238))
- Add strict values.yaml schema validation ([#227](https://github.com/signalfx/splunk-otel-collector-chart/pull/227),[#234](https://github.com/signalfx/splunk-otel-collector-chart/pull/#234),[#239](https://github.com/signalfx/splunk-otel-collector-chart/pull/#239))

### 💡 Enhancements 💡

- BREAKING CHANGE: Reorder resource detectors, moving the `system` detector
  to the end of the list. Applying this change in an EC2 or Azure environment
  may change the `host.name` dimension and the resource ID dimension
  on some MTSes, possibly causing detectors to fire.
- BREAKING CHANGE: Reduce scope of host mounted volumes on linux systems ([#232](https://github.com/signalfx/splunk-otel-collector-chart/pull/232))
- Change `run_id` log resource attribute to `k8s.container.restart_count` ([#226](https://github.com/signalfx/splunk-otel-collector-chart/pull/226))
- Use only `splunkPlatform.endpoint` and `splunkObservability.realm` parameters
  to identify which destination is enabled, remove default value for
  `splunkObservability.realm` ([#230](https://github.com/signalfx/splunk-otel-collector-chart/pull/230),[#233](https://github.com/signalfx/splunk-otel-collector-chart/pull/#233))
- Upgrade splunk-otel-collector image to 0.37.1 ([#237](https://github.com/signalfx/splunk-otel-collector-chart/pull/237),[#249](https://github.com/signalfx/splunk-otel-collector-chart/pull/#249))
- Simplify configuration for switching to native OTel logs collection ([#246](https://github.com/signalfx/splunk-otel-collector-chart/pull/246))

### 🧰 Bug fixes 🧰

- Fix setting of SPLUNK_MEMORY_TOTAL_MIB env var in otelAgent daemonset ([#240](https://github.com/signalfx/splunk-otel-collector-chart/pull/240))
- Enable OTLP HTTP ports (4318 and 55681) in otelAgent daemonset ([#243](https://github.com/signalfx/splunk-otel-collector-chart/pull/243))

## [0.36.2] - 2021-10-08

### 🧰 Bug fixes 🧰

- Exclude redundant `groupbyattrs/logs` processor from native logs collection
  pipeline ([#219](https://github.com/signalfx/splunk-otel-collector-chart/pull/219))
- Fix deprecation messages for old `<telemetry>Enabled` parameters ([#220](https://github.com/signalfx/splunk-otel-collector-chart/pull/220))

## [0.36.1] - 2021-10-07

### 🧰 Bug fixes 🧰

- Fix backward compatibility for `splunkRealm` parameter ([#218](https://github.com/signalfx/splunk-otel-collector-chart/pull/218))

## [0.36.0] - 2021-10-06

### 🚀 New components 🚀

- Support k8s clusters with Windows nodes ([#190](https://github.com/signalfx/splunk-otel-collector-chart/pull/190))

### 💡 Enhancements 💡

- Change configuration interface to be able to send data to Splunk
  Enterprise/Cloud and to Splunk Observability ([#209](https://github.com/signalfx/splunk-otel-collector-chart/pull/209))
- Improve multiline logs configuration for native logs collection ([#208](https://github.com/signalfx/splunk-otel-collector-chart/pull/208))

## [0.35.3] - 2021-09-29

### 🚀 New components 🚀

- Add an option to provide additional custom RBAC rules ([#206](https://github.com/signalfx/splunk-otel-collector-chart/pull/206))

## [0.35.2] - 2021-09-28

### 🚀 New components 🚀

- Send k8s events additionally to Splunk HEC endpoint ([#202](https://github.com/signalfx/splunk-otel-collector-chart/pull/202))

## [0.35.1] - 2021-09-23

### 🚀 New components 🚀

- Add support for OpenShift distribution ([#196](https://github.com/signalfx/splunk-otel-collector-chart/pull/196))
- Add native OTel logs collection as an option ([#197](https://github.com/signalfx/splunk-otel-collector-chart/pull/197))

### Removed

- Remove PodSecurityPolicy installation option ([#195](https://github.com/signalfx/splunk-otel-collector-chart/pull/195))

## [0.35.0] - 2021-09-17

### 🚀 New components 🚀

- Add an option to collect k8s events with smartagent/kubernetes-events receiver ([#187](https://github.com/signalfx/splunk-otel-collector-chart/pull/187))

### 💡 Enhancements 💡

- Move k8s metadata enrichment from fluentd to otel-collector ([#192](https://github.com/signalfx/splunk-otel-collector-chart/pull/192))

## [0.31.0] - 2021-08-10

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.31.0 ([#183](https://github.com/signalfx/splunk-otel-collector-chart/pull/183))
- Set more frequent checks for memory_limiter ([#178](https://github.com/signalfx/splunk-otel-collector-chart/pull/178))
- Make Fluentd init container image variable ([#182](https://github.com/signalfx/splunk-otel-collector-chart/pull/182))

### 🧰 Bug fixes 🧰

- All missing attributes are added to prometheus metrics reported
  by gateway and k8s-cluster-receiver collector deployments ([#170](https://github.com/signalfx/splunk-otel-collector-chart/pull/170))
- Fix pod affinity setting ([#181](https://github.com/signalfx/splunk-otel-collector-chart/pull/181))

## [0.29.1] - 2021-07-09

### 🧰 Bug fixes 🧰

- Fix generation of service.name log attribute in istio environment ([#176](https://github.com/signalfx/splunk-otel-collector-chart/pull/176))

## [0.29.0] - 2021-07-08

### 💡 Enhancements 💡

- Change internal metrics port from 8888 to 8889 ([#172](https://github.com/signalfx/splunk-otel-collector-chart/pull/172))
- Upgrade splunk-otel-collector image version to 0.29.0 ([#174](https://github.com/signalfx/splunk-otel-collector-chart/pull/174))

## [0.28.2] - 2021-07-07

### 🚀 New components 🚀

- Add Istio specific configurations ([#171](https://github.com/signalfx/splunk-otel-collector-chart/pull/171))
- Enable OTLP receiver in logs pipeline ([#167](https://github.com/signalfx/splunk-otel-collector-chart/pull/167))

### Removed

- BREAKING: Remove SAPM receiver from default config ([#168](https://github.com/signalfx/splunk-otel-collector-chart/pull/168))

## [0.28.1] - 2021-06-18

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.28.1 ([#166](https://github.com/signalfx/splunk-otel-collector-chart/pull/166))

## [0.28.0] - 2021-06-16

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector image to 0.28.0 ([#164](https://github.com/signalfx/splunk-otel-collector-chart/pull/164))

## [0.27.0] - 2021-06-15

### 💡 Enhancements 💡

- BREAKING CHANGE: Auto-detection of prometheus metrics is disabled by default ([#163](https://github.com/signalfx/splunk-otel-collector-chart/pull/163)). See
  [Upgrade guideline](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/UPGRADING.md#0264-to-0270)

## [0.26.4] - 2021-06-09

### 🧰 Bug fixes 🧰

- Fix container runtime detection when metrics pipeline disabled ([#161](https://github.com/signalfx/splunk-otel-collector-chart/pull/161))

## [0.26.3] - 2021-06-08

- Add an option to add extra labels to pods ([#158](https://github.com/signalfx/splunk-otel-collector-chart/pull/158))
- Add an option to add extra annotations to deployments, daemonset, pods and service account ([#158](https://github.com/signalfx/splunk-otel-collector-chart/pull/158))
- Add an option to mount extra volumes to gateway-mode and k8s cluster receiver collectors ([#157](https://github.com/signalfx/splunk-otel-collector-chart/pull/157))

## [0.26.2] - 2021-05-28

### 💡 Enhancements 💡

- Automatically detect container runtime using initContainers and apply
  relevant parsing config instead of asking user to specify criTimeFormat.
  This is an important change to enable smooth transition from deprecated docker
  to containerd runtime ([#154](https://github.com/signalfx/splunk-otel-collector-chart/pull/154))

## [0.26.1] - 2021-05-25

### 🚀 New components 🚀

- Add an option to mount extra volumes using `otelAgent.extraVolumes` and `otelAgent.extraVolumeMounts` ([#151](https://github.com/signalfx/splunk-otel-collector-chart/pull/151))

## [0.26.0] - 2021-05-21

### 🚀 New components 🚀

- Add signalfx metrics receiver to the agent ([#136](https://github.com/signalfx/splunk-otel-collector-chart/pull/136))

### 💡 Enhancements 💡

- fluentd logs are now sent through the collector instead of being sent directly to the backend ([#109](https://github.com/signalfx/splunk-otel-collector-chart/pull/109))
- Logs are sent through the OpenTelemetry Agent on the local node by default. `otelAgent.enabled` value must be set to `true` when using logs ([#127](https://github.com/signalfx/splunk-otel-collector-chart/pull/127))
- `otelAgent.ports` and `otelCollector.ports` are selectively enabled depending on what telemetry types are enabled with `metricsEnabled`, `tracesEnabled`, and `logsEnabled`
- Removed setting `host.name` through the `resource` processor as it is already set by the `resourcedetection/system` processor
- Upgraded to Splunk OpenTelemetry Collector 0.26.0
- Kubernetes cluster metrics now have a dimension `receiver:k8scluster` to ensure that
  MTS do not conflict with Kubernetes metrics sent by Smart Agent for the same cluster. ([#134](https://github.com/signalfx/splunk-otel-collector-chart/pull/134))

### Removed

- Removed `ingestHost`, `ingestPort`, `ingestProtocol`, use `ingestUrl` instead ([#123](https://github.com/signalfx/splunk-otel-collector-chart/pull/123))
- Removed `logsBackend`, configure `splunk_hec` exporter directly ([#123](https://github.com/signalfx/splunk-otel-collector-chart/pull/123))
- Removed `splunk.com/index` annotation for logs ([#123](https://github.com/signalfx/splunk-otel-collector-chart/pull/123))
- Removed `fluentd.config.indexFields` as all fields sent are indexed ([#123](https://github.com/signalfx/splunk-otel-collector-chart/pull/123))
- Removed `fluentforward` receiver from gateway ([#127](https://github.com/signalfx/splunk-otel-collector-chart/pull/127))
- Removed `service.ports`, sourced from `otelCollector.ports` instead ([#140](https://github.com/signalfx/splunk-otel-collector-chart/pull/140))

## [0.25.0] - 2021-05-07

### 💡 Enhancements 💡

- Upgrade splunk-otel-collector docker image to 0.25.0 ([#131](https://github.com/signalfx/splunk-otel-collector-chart/pull/131))

### 🚀 New components 🚀

- Pre-rendered manifests can be found in [rendered](rendered) directory

## [0.24.13] - 2021-05-04

### 💡 Enhancements 💡

- Remove internal fluentd metrics sent as logs with monitor_agent. Prometheus
  metrics exposed on 0.0.0.0:24231 should be used instead ([#122](https://github.com/signalfx/splunk-otel-collector-chart/pull/122))

## [0.24.12] - 2021-05-03

### 🧰 Bug fixes 🧰

- Fix logs collection configuration for CRI-O / containerd runtimes ([#120](https://github.com/signalfx/splunk-otel-collector-chart/pull/120))

## [0.24.11] - 2021-04-29

### 💡 Enhancements 💡

- Change the way to configure "concat" filter for container logs ([#117](https://github.com/signalfx/splunk-otel-collector-chart/pull/117))

## [0.24.10] - 2021-04-21

### 💡 Enhancements 💡

- Disable fluentd metrics collection by default ([#108](https://github.com/signalfx/splunk-otel-collector-chart/pull/108))

## [0.24.9] - 2021-04-18

### 💡 Enhancements 💡

- Change OTLP port from deprecated 55680 to default 4317 ([#103](https://github.com/signalfx/splunk-otel-collector-chart/pull/103))

### 🧰 Bug fixes 🧰

- Open port for signalfx-forwarder on the agent ([#106](https://github.com/signalfx/splunk-otel-collector-chart/pull/106))

## [0.24.8] - 2021-04-16

### 🧰 Bug fixes 🧰

- Fix traces enrichment with k8s metadata ([#102](https://github.com/signalfx/splunk-otel-collector-chart/pull/102))

## [0.24.7] - 2021-04-15

### 💡 Enhancements 💡

- Switch to stable Splunk OTel Collector image 0.24.3 ([#100](https://github.com/signalfx/splunk-otel-collector-chart/pull/100))

## [0.24.6] - 2021-04-15

### 🚀 New components 🚀

- Enable smartagent/signalfx-forwarder in the default agent trace pipeline ([#98](https://github.com/signalfx/splunk-otel-collector-chart/pull/98))

## [0.24.5] - 2021-04-13

### 🚀 New components 🚀

- Enable batch processor in the default metrics pipelines ([#90](https://github.com/signalfx/splunk-otel-collector-chart/pull/90))

### 💡 Enhancements 💡

- Ensure all metrics and traces are routed through the gateway deployment if
  it's enabled ([#96](https://github.com/signalfx/splunk-otel-collector-chart/pull/96))

## [0.24.4] - 2021-04-12

### 🚀 New components 🚀

- Add an option to set extra environment variables ([#91](https://github.com/signalfx/splunk-otel-collector-chart/pull/91))

## [0.24.3] - 2021-04-12

### 🧰 Bug fixes 🧰

- Fix resource attribute in the default traces pipeline ([#88](https://github.com/signalfx/splunk-otel-collector-chart/pull/88))
- Add metric_source:kubernetes for all k8s cluster metrics ([#89](https://github.com/signalfx/splunk-otel-collector-chart/pull/89))
- Fix host.name attribute in logs ([#87](https://github.com/signalfx/splunk-otel-collector-chart/pull/87))

## [0.24.2] - 2021-04-07

### 🚀 New components 🚀

- Add host.name attribute to logs ([#86](https://github.com/signalfx/splunk-otel-collector-chart/pull/86))

## [0.24.1] - 2021-04-07

### 💡 Enhancements 💡

- Remove deprecated opencensus receiver ([#85](https://github.com/signalfx/splunk-otel-collector-chart/pull/85))

## [0.24.0] - 2021-04-07

### 💡 Enhancements 💡

- Upgrade image to 0.24.0 version ([#84](https://github.com/signalfx/splunk-otel-collector-chart/pull/84))
- Add system detector to default metrics and traces pipelines ([#84](https://github.com/signalfx/splunk-otel-collector-chart/pull/84))
