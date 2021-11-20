# Log architecture

Splunk OpenTelemetry Collector for Kubernetes deploys a DaemonSet on each node. In the DaemonSet, an OpenTelemetry container runs and does the collecting job. Splunk OpenTelemetry Collector for Kubernetes uses the [node logging agent](https://kubernetes.io/docs/concepts/cluster-administration/logging/#using-a-node-logging-agent) method. See [Kubernetes Logging Architecture](https://kubernetes.io/docs/concepts/cluster-administration/logging/) for an overview of the types of Kubernetes logs from which you may wish to collect data, as well as information on how to set up those logs.
Splunk OpenTelemetry Collector for Kubernetes collects the following types of data:

* Logs: Splunk OpenTelemetry Collector for Kubernetes collects two types of logs:
  * Logs from Kubernetes system components (https://kubernetes.io/docs/concepts/overview/components/)
  * Applications (container) logs
