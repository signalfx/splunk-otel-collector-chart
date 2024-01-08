# Advanced Configuration

The
[values.yaml](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/helm-charts/splunk-otel-collector/values.yaml)
lists all supported configurable parameters for this chart, along with detailed
explanation. Read through it to understand how to configure this chart.

Also check [examples of chart configuration](../examples/README.md). This also includes a guide to deploy for the k8s cluster with the windows worker node.

At the minimum you need to configure the following values to send data to Splunk
Enterprise/Cloud.

```yaml
splunkPlatform:
  token: xxxxxx
  endpoint: http://localhost:8088/services/collector
```

At the minimum you need to configure the following values to send data to Splunk
Observability Cloud.

```yaml
splunkObservability:
  accessToken: xxxxxx
  realm: us0
clusterName: my-k8s-cluster
```

## Provide tokens as a secret

Instead of having the tokens as clear text in the values, those can be provided via a secret that is created before deploying the chart. See [secret-splunk.yaml](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/helm-charts/splunk-otel-collector/templates/secret-splunk.yaml) for the required fields.

```yaml
secret:
  create: false
  name: your-secret
```

## Cloud provider

Use the `cloudProvider` parameter to provide information about the cloud
provider, if any.

- `aws` - Amazon Web Services
- `gcp` - Google Cloud
- `azure` - Microsoft Azure

This value can be omitted if none of the values apply.

## Kubernetes distribution

Use the `distribution` parameter to provide information about underlying
Kubernetes deployment. This parameter allows the collector to automatically
scrape additional metadata. The supported options are:

- `aks` - Azure AKS
- `eks` - Amazon EKS
- `eks/fargate` - Amazon EKS with Fargate profiles
- `gke` - Google GKE / Standard mode
- `gke/autopilot` - Google GKE / Autopilot mode
- `openshift` - Red Hat OpenShift

This value can be omitted if none of the values apply.

## Deployment environment

Optional `environment` parameter can be used to specify an additional `deployment.environment`
attribute that will be added to all the telemetry data. It will help Splunk Observability
users to investigate data coming from different source separately.
Value examples: development, staging, production, etc.

```yaml
environment: production
```

## Disable particular types of telemetry

By default only metrics and traces are sent to Splunk Observability destination,
and only logs are sent to Splunk Platform destination. It's possible to enable
or disable any kind of telemetry for a specific destination. For example, with
the following configuration Splunk OTel Collector will send all collected
telemetry data to Splunk Observability and Splunk Platform assuming they are
both properly configured.

```yaml
splunkObservability:
  metricsEnabled: true
  tracesEnabled: true
  logsEnabled: true
splunkPlatform:
  metricsEnabled: true
  logsEnabled: true
```

## Windows worker nodes support

Splunk OpenTelemetry Collector for Kubernetes supports collection of metrics,
traces and logs (using OTel native logs collection only) from Windows nodes.

All windows images are available in a separate `quay.io` repository:
`quay.io/signalfx/splunk-otel-collector-windows` with two release tracking tags
available: `latest` (Server 2019) and `latest-2022` (Server 2022). Version tags
follow the convention of `<appVersion>` (2019) and `<appVersion>-2022` (2022).
The digests for each release are detailed at
https://github.com/signalfx/splunk-otel-collector/releases.

Use the following values.yaml configuration to install the helm chart on Windows
worker nodes:

```yaml
isWindows: true
image:
  otelcol:
    repository: quay.io/signalfx/splunk-otel-collector-windows
    tag: <appVersion>-2022
logsEngine: otel
readinessProbe:
  initialDelaySeconds: 60
livenessProbe:
  initialDelaySeconds: 60
```

If you have both Windows and Linux worker nodes in your Kubernetes cluster, you
need to install the helm chart twice. One of the installations with default
configuration `isWindows: false` will be applied on Linux nodes. Another
installation with values.yaml configuration that provided above will be applied
on Windows nodes. And it's important to disable `clusterReceiver` on one of the
installations to avoid cluster-wide metrics duplication, add the following line
to values.yaml of one of the installations:

```yaml
clusterReceiver:
  enabled: false
```

## Discovery Mode

*At this time only Linux installations are supported*

The agent daemonset can be configured to run in [`--discovery` mode](https://docs.splunk.com/observability/en/gdi/opentelemetry/discovery-mode.html), initiating a "preflight" discovery phase to test bundled metric receiver configurations against `k8s_observer` discovered endpoints. Successfully discovered instances are then incorporated in the existing service config.

You can additionally configure any necessary discovery properties for providing required auth or service-specific information:

```yaml
agent:
  discovery:
    enabled: true # disabled by default
    properties:
      extensions:
        k8s_observer:
          config:
            auth_type: serviceAccount  # default auth_type value
      receivers:
        postgres:
          config:
            # auth fields reference environment variables populated by secret data below
            username: '${env:POSTGRES_USER}'
            password: '${env:POSTGRES_PASSWORD}'
            tls:
              insecure: true

  extraEnvs:
    # environment variables using a manually created "postgres-monitoring" secret
    - name: POSTGRES_USER
      valueFrom:
        secretKeyRef:
          name: postgres-monitoring
          key: username
    - name: POSTGRES_PASSWORD
      valueFrom:
        secretKeyRef:
          name: postgres-monitoring
          key: password
```

For discovery progress and statement evaluations, see the agent startup logs in product or via kubectl.

```bash
$ kubectl -n monitoring logs splunk-otel-collector-agent | grep -i disco
Discovering for next 10s...
Successfully discovered "postgresql" using "k8s_observer" endpoint "k8s_observer/e8a10f52-4f2a-468c-be7b-7f3c673b1c8e/(5432)".
Discovery complete.
```

By default, the `docker_observer` and `host_observer` extensions are disabled for discovery in the helm chart.

## GKE Autopilot support

If you want to run Splunk OTel Collector in [Google Kubernetes Engine
Autopilot](https://cloud.google.com/kubernetes-engine/docs/concepts/autopilot-overview),
make sure to set `distribution` setting to `gke/autopilot`:

```yaml
distribution: gke/autopilot
```

Sometimes Splunk OTel Collector agent daemonset can have [problems scheduling in
Autopilot](https://cloud.google.com/kubernetes-engine/docs/concepts/daemonset#autopilot-ds-best-practices)
If you run into these issues, you can assign the daemonset a higher [priority
class](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/),
this will make sure that the daemonset pods are always present on each node:

1. Create a new priority class for Splunk OTel Collector agent:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: splunk-otel-agent-priority
value: 1000000
globalDefault: false
description: "Higher priority class for Splunk OpenTelemetry Collector pods."
EOF
```

2. Use the created priority class in the helm install/upgrade command:
with `--set="priorityClassName=splunk-otel-agent-priority"` cli argument or add
the following line to your custom values.yaml:

```yaml
priorityClassName: splunk-otel-agent-priority
```

## GKE ARM support

We support ARM workloads on GKE with default configurations of this helm chart.
Make sure to set the required `distribution` value to `gke`:

```yaml
distribution: gke
```

## EKS Fargate support

If you want to run the Splunk OpenTelemetry Collector in [Amazon Elastic Kubernetes Service
with Fargate profiles](https://docs.aws.amazon.com/eks/latest/userguide/fargate.html),
make sure to set the required `distribution` value to `eks/fargate`:

```yaml
distribution: eks/fargate
```

**NOTE:** Fluentd and Native OTel logs collection are not yet automatically configured in EKS with Fargate profiles

This distribution will operate similarly to the `eks` distribution but with the following distinctions:

1. The Collector agent daemonset is not applied since Fargate doesn't support daemonsets. Any desired Collector instances
running as agents must be configured manually as sidecar containers in your custom deployments. This includes any application
logging services like Fluentd. We recommend setting the `gateway.enabled` to `true` and configuring your instrumented
applications to report metrics, traces, and logs to the gateway's `<installed-chart-name>-splunk-otel-collector` service address.
Any desired agent instances that would run as a daemonset should instead run as sidecar containers in your pods.

2. Since Fargate nodes use a VM boundary to prevent access to host-based resources used by other pods, pods are not able to reach their own kubelet. The cluster receiver
for the Fargate distribution has two primary differences between regular `eks` to work around this limitation:
    * The configured cluster receiver is deployed as a 2-replica StatefulSet instead of a Deployment and uses a
    [Kubernetes Observer extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/extension/observer/k8sobserver/README.md)
    that discovers the cluster's nodes and, on the second replica, its pods for user-configurable receiver creator additions. It uses this observer to dynamically create
    [Kubelet Stats receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/kubeletstatsreceiver/README.md)
    instances that will report kubelet metrics for all observed Fargate nodes. The first replica will monitor the cluster with a `k8s_cluster` receiver
    and the second will monitor all kubelets except its own (due to an EKS/Fargate networking restriction).

    * The first replica's collector will monitor the second's kubelet. This is made possible by a Fargate-specific `splunk-otel-eks-fargate-kubeletstats-receiver-node`
    node label. The Collector's ClusterRole for `eks/fargate` will allow the `patch` verb on `nodes` resources for the default API groups to allow the cluster
    receiver's init container to add this node label for designated self monitoring.

## Control Plane metrics

By setting `agent.controlPlaneMetrics.{component}.enabled=true` the helm chart will set up the otel-collector agent to
collect metrics from a particular control plane component. Most metrics can be collected from the control plane
with no extra configuration, however, extra configuration steps must be taken to collect metrics from etcd (
[see below](#setting-up-etcd-metrics)
) due to TLS security requirements.

To collect control plane metrics, the helm chart has the otel-collector agent on each node use the
[receiver creator](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/receivercreator/README.md)
to instantiate control plane receivers at runtime. The receiver creator has a set of discovery rules to know
which control plane receivers to create. The default discovery rules can vary depending on the Kubernetes distribution
and version. If your control plane is using nonstandard specs, then you can provide a custom configuration (
[see below](#using-custom-configurations-for-nonstandard-control-plane-components)
) so the otel-collector agent can still successfully connect.

The otel-collector agent relies on having pod level network access to collect metrics from the control plane pods.
Since most cloud Kubernetes as a service distributions don't expose the control plane pods to the
end user, collecting metrics from these distributions is not supported.

* Supported Distributions:
  * kubernetes 1.22 (kops created)
  * openshift v4.9
* Unsupported Distributions:
  * aks
  * eks
  * eks/fargate
  * gke
  * gke/autopilot

The default configurations for the control plane receivers can be found in
[_otel-agent.tpl](../helm-charts/splunk-otel-collector/templates/config/_otel-agent.tpl).

### Receiver documentation

Here are the documentation links that contain configuration options and supported metrics information for each receiver
used to collect metrics from the control plane.
* [smartagent/coredns](https://docs.splunk.com/Observability/gdi/coredns/coredns.html)
* [smartagent/etcd](https://docs.splunk.com/Observability/gdi/etcd/etcd.html)
* [smartagent/kube-controller-manager](https://docs.splunk.com/Observability/gdi/kube-controller-manager/kube-controller-manager.html)
* [smartagent/kubernetes-apiserver](https://docs.splunk.com/Observability/gdi/kubernetes-apiserver/kubernetes-apiserver.html)
* [smartagent/kubernetes-proxy](https://docs.splunk.com/Observability/gdi/kubernetes-proxy/kubernetes-proxy.html)
* [smartagent/kubernetes-scheduler](https://docs.splunk.com/Observability/gdi/kubernetes-scheduler/kubernetes-scheduler.html)

### Setting up etcd metrics

The etcd metrics cannot be collected out of box because etcd requires TLS authentication for communication. Below, we
have supplied a couple methods for setting up TLS authentication between etcd and the otel-collector agent. The etcd TLS
client certificate and key play a critical role in the security of the cluster, handle them with care and avoid storing
them in unsecured locations. To limit unnecessary access to the etcd certificate and key, you should deploy the helm
chart into a namespace that is isolated from other unrelated resources.

#### Method 1: Deploy the helm chart with the etcd certificate and key as values
The easiest way to set up the TLS authentication for etcd metrics is to retrieve the client certificate and key from an
etcd pod and directly use them in the values.yaml (or using --set=). The helm chart will set up the rest. The helm chart
will add the client certificate and key to a newly created kubernetes secret and then configure the etcd receiver to use
them.

You can get the contents of the certificate and key by running these commands. The path to the certificate and key can
vary depending on your Kubernetes distribution.
```bash
# The steps for kubernetes and openshift are listed here.
# For kubernetes:
etcd_pod_name=$(kubectl get pods -n kube-system -l k8s-app=etcd-manager-events -o=name |  sed "s/^.\{4\}//" | head -n 1)
kubectl exec -it -n kube-system {etcd_pod_name} cat /etc/kubernetes/pki/etcd-manager/etcd-clients-ca.crt
kubectl exec -it -n kube-system {etcd_pod_name} cat /etc/kubernetes/pki/etcd-manager/etcd-clients-ca.key
# For openshift:
etcd_pod_name=$(kubectl get pods -n openshift-etcd -l k8s-app=etcd -o=name |  sed "s/^.\{4\}//" | head -n 1)
kubectl exec -it -n openshift-etcd {etcd_pod_name} cat /etc/kubernetes/static-pod-certs/secrets/etcd-all-certs/etcd-serving-metrics-{etcd_pod_name}.crt
kubectl exec -it -n openshift-etcd {etcd_pod_name} cat /etc/kubernetes/static-pod-certs/secrets/etcd-all-certs/etcd-serving-metrics-{etcd_pod_name}.key
```

Once you have the contents of your certificate and key, insert them into your values.yaml. Since the helm chart will
create the secret, you must specify agent.controlPlaneMetrics.etcd.secret.create=true. Then install your helm chart.
```yaml
agent:
  controlPlaneMetrics:
    etcd:
      enabled: true
      secret:
        create: true
        # The PEM-format CA certificate for this client.
        clientCert: |
          -----BEGIN CERTIFICATE-----
          ...
          -----END CERTIFICATE-----
        # The private key for this client.
        clientKey: |
          -----BEGIN RSA PRIVATE KEY-----
          ...
          -----END RSA PRIVATE KEY-----
        # Optional. The CA cert that has signed the TLS cert.
        # caFile: |
```

#### Method 2: Deploy the helm chart with a secret that contains the etcd certificate and key
To set up the TLS authentication for etcd metrics with this method, the otel-collector agents will need access to a
kubernetes secret that contains the etcd TLS client certificate and key. The name of this kubernetes secret must be
supplied in the helm chart (.Values.agent.controlPlaneMetrics.etcd.secret.name). When installed, the helm chart will
mount the specified kubernetes secret onto the /otel/etc/etcd directory of the otel-collector agent containers so the
agent can use it.

Here are the commands for creating a kubernetes secret named splunk-monitoring-etcd.
```bash
# The steps for kubernetes and openshift are listed here.
# For kubernetes:
etcd_pod_name=$(kubectl get pods -n kube-system -l k8s-app=etcd-manager-events -o=name |  sed "s/^.\{4\}//" | head -n 1)
kubectl exec -it -n kube-system $etcd_pod_name -- cat /etc/kubernetes/pki/etcd-manager/etcd-clients-ca.crt > ./tls.crt
kubectl exec -it -n kube-system $etcd_pod_name -- cat /etc/kubernetes/pki/etcd-manager/etcd-clients-ca.key > ./tls.key
# For openshift:
etcd_pod_name=$(kubectl get pods -n openshift-etcd -l k8s-app=etcd -o=name |  sed "s/^.\{4\}//" | head -n 1)
kubectl exec -it -n openshift-etcd {etcd_pod_name} cat /etc/kubernetes/static-pod-certs/secrets/etcd-all-certs/etcd-serving-metrics-{etcd_pod_name}.crt > ./tls.crt
kubectl exec -it -n openshift-etcd {etcd_pod_name} cat /etc/kubernetes/static-pod-certs/secrets/etcd-all-certs/etcd-serving-metrics-{etcd_pod_name}.key > ./tls.key

# Create the the secret.
# The input file names must be one of:  tls.crt, tls.key, cacert.pem
kubectl create secret generic splunk-monitoring-etcd --from-file=./tls.crt --from-file=./tls.key
# Optional. Include the CA cert that has signed the TLS cert.
# kubectl create secret generic splunk-monitoring-etcd --from-file=./tls.crt --from-file=./tls.key --from-file=cacert.pem

# Cleanup the local files.
rm ./tls.crt
rm ./tls.key
```

Once your kubernetes secret is created, specify the secret's name in values.yaml. Since the helm chart will be using the
secret you created, make sure to set .Values.agent.controlPlaneMetrics.etc.secret.create=false. Then install your helm
chart.
```yaml
agent:
  controlPlaneMetrics:
    etcd:
      enabled: true
      secret:
        create: false
        name: splunk-monitoring-etcd
```

### Using custom configurations for nonstandard control plane components

A user may need to override the default configuration values used to connect to the control plane for a couple different
reason. If your control plane uses nonstandard ports or custom TLS settings, then you will need to override the default
configurations. Here is an example of how you could connect to a nonstandard apiserver that uses port 3443 for metrics
and custom TLS certs stored in the /etc/myapiserver/ directory.

```yaml
agent:
  config:
    receivers:
      receiver_creator:
        receivers:
          # Template for overriding the discovery rule and config.
          # smartagent/{control_plane_receiver}:
          #   rule: {rule_value}
          #   config:
          #     {config_value}
          smartagent/kubernetes-apiserver:
            rule: type == "port" && port == 3443 && pod.labels["k8s-app"] == "kube-apiserver"
            config:
              clientCertPath: /etc/myapiserver/clients-ca.crt
              clientKeyPath: /etc/myapiserver/clients-ca.key
              skipVerify: true
              useHTTPS: true
              useServiceAccount: false
```

### Known issues

Kube Proxy
* `10249: connect: connection refused`
  * Issue
    * When using a Kubernetes cluster with non-default configurations for kube proxy, there is a reported network connectivity issue that prevents the collection of proxy metrics.
  * Solution
    * Update the kube proxy metric bind address (--metrics-bind-address) in the cluster spec.
Set the kubeProxy metrics bind address to 0.0.0.0 or another value based on your Kubernetes cluster distribution.
For this particular issue, the solution may vary depending on the Kubernetes cluster distribution. It is recommended to research what your Kubernetes distribution recommends for addressing this issue.
  * Related Issue Links
    * [kubernetes - Expose kube-proxy metrics on 0.0.0.0 by default ](https://github.com/kubernetes/kubernetes/pull/74300)
    * [kubernetes - kube-proxy TLS support](https://github.com/kubernetes/kubernetes/issues/106870)
    * [splunk-otel-collector-chart - Error connecting to kubernetes-proxy](https://github.com/signalfx/splunk-otel-collector-chart/issues/758)
    * [kops - expose metrics-bind-address configuration for kube-proxy](https://github.com/kubernetes/kops/issues/6472)
    * [prometheus - prometheus-kube-stack - kube-proxy metrics status with connection refused](https://github.com/prometheus-community/helm-charts/issues/977)

## Logs collection

The helm chart utilizes OpenTelemetry Collector for Kubernetes logs collection, but it also provides an option to use
[fluentd](https://docs.fluentd.org/) which will be deployed as a sidecar. Logs collected with fluentd are sent through
Splunk OTel Collector agent which does all the necessary metadata enrichment. The fluentd was initially introduced
before the native OpenTelemetry logs collection was available. It will be deprecated and removed at some point in future.

Use the following configuration to switch between Fluentd and OpenTelemetry logs collection:

```yaml
logsEngine: <fluentd|otel>
```

### Difference between Fluentd and OpenTelemetry logs collection

#### Emitted logs

There is almost no difference in the logs emitted by default by the two engines. The only difference is that
Fluentd logs have an additional attribute called `fluent.tag`, which has a value similar to the `source` HEC field.

#### Performance and resource usage

Fluend logs collection requires an additional sidecar container responsible for collecting logs and sending them to the
OTel collector container for further enrichment. No sidecar containers are required for the OpenTelemetry logs collection.
OpenTelemetry logs collection is multi-threaded, so it can handle more logs per second without additional configuration.
Our internal benchmarks show that OpenTelemetry logs collection provides higher throughput with less resource usage.

#### Configuration

Fluentd logs collection is configured using the `fluentd.config` section in values.yaml. OpenTelemetry logs
collection is configured using the `logsCollection` section in values.yaml. The configuration options are
different between the two engines, but they provide similar functionality.

### Add log files from Kubernetes host machines/volumes

You can add additional log files to be ingested from Kubernetes host machines and Kubernetes volumes by configuring `agent.extraVolumes`, `agent.extraVolumeMounts` and `logsCollection.extraFileLogs` in the values.yaml file used to deploy Splunk OpenTelemetry Collector for Kubernetes.

Example of adding audit logs from Kubernetes host machines

```yaml
logsCollection:
  extraFileLogs:
    filelog/audit-log:
      include: [/var/log/kubernetes/apiserver/audit.log]
      start_at: beginning
      include_file_path: true
      include_file_name: false
      resource:
        com.splunk.source: /var/log/kubernetes/apiserver/audit.log
        host.name: 'EXPR(env("K8S_NODE_NAME"))'
        com.splunk.sourcetype: kube:apiserver-audit
agent:
  extraVolumeMounts:
    - name: audit-log
      mountPath: /var/log/kubernetes/apiserver
  extraVolumes:
    - name: audit-log
      hostPath:
        path: /var/log/kubernetes/apiserver
```

### Processing multi-line logs

Splunk OpenTelemetry Collector for Kubernetes supports parsing of multi-line logs to help read, understand, and troubleshoot the multi-line logs in a better way.
Process multi-line logs by configuring `logsCollection.containers.multilineConfigs` section in values.yaml.

```yaml
logsCollection:
  containers:
    multilineConfigs:
      - namespaceName:
          value: default
        podName:
          value: buttercup-app-.*
          useRegexp: true
        containerName:
          value: server
        firstEntryRegex: ^[^\s].*
        combineWith: ""
```

Use https://regex101.com/ to find a golang regex that works for your format and specify it in the config file for the config option `firstEntryRegex`.


### Collect journald events

Splunk OpenTelemetry Collector for Kubernetes can collect journald events from kubernetes environment.
Process journald events by configuring `logsCollection.journald` section in values.yaml.

```yaml
logsCollection:
  journald:
    enabled: true
    directory: /run/log/journal
    # List of service units to collect and configuration for each. Please update the list as needed.
    units:
      - name: kubelet
        priority: info
      - name: docker
        priority: info
      - name: containerd
        priority: info
    # Route journald logs to its own Splunk Index by specifying the index value below, else leave it blank. Please make sure the index exist in Splunk and is configured to receive HEC traffic (Not applicable to Splunk Observability).
    index: ""
```

### Managing Log Ingestion by Using Annotations

Manage Splunk OTel Collector Logging with these supported annotations.

* Use `splunk.com/index` annotation on pod and/or namespace to tell which Splunk platform indexes to ingest to. Pod annotation will take precedence over namespace annotation when both are annotated.
  For example, the following command will make logs from `kube-system` namespace to be sent to `k8s_events` index: `kubectl annotate namespace kube-system splunk.com/index=k8s_events`
* Filter logs using pod and/or namespace annotation
  * If `logsCollection.containers.useSplunkIncludeAnnotation` is `false` (default: false), set `splunk.com/exclude` annotation to `true` on pod and/or namespace to exclude its logs from ingested.
  * If `logsCollection.containers.useSplunkIncludeAnnotation` is `true` (default: false), set `splunk.com/include` annotation to `true` on pod and/or namespace to only include its logs from ingested. All other logs will be ignored.
* Use `splunk.com/sourcetype` annotation on pod to overwrite `sourcetype` field. If not set, it is dynamically generated to be `kube:container:CONTAINER_NAME`.

### Performance of native OpenTelemetry logs collection

Some configurations used with the OpenTelemetry Collector (as set using the Splunk OpenTelemetry Collector for Kubernetes helm chart) can have an impact on overall performance of log ingestion. The more receivers, processors, exporters, and extensions that are added to any of the pipelines, the greater the performance impact.

Splunk OpenTelemetry Collector for Kubernetes can exceed the default throughput of the The HTTP Event Collector (HEC). To best address capacity needs, monitor the HEC throughput and back pressure on Splunk OpenTelemetry Collector for Kubernetes deployments and be prepared to add additional nodes as needed.

Here is the summary of performance benchmarks run internally.
| Log Generator Count | Event Size (byte) | Agent CPU Usage | Agent EPS |
|---------------------|-------------------|-----------------|-----------|
|                   1 |               256 |             1.8 |    30,000 |
|                   1 |               516 |             1.8 |    28,000 |
|                   1 |              1024 |             1.8 |    24,000 |
|                   5 |               256 |             3.2 |    54,000 |
|                   7 |               256 |               3 |    52,000 |
|                  10 |               256 |             3.2 |    53,000 |

The data pipelines for these test runs involved reading container logs as they are being written, then parsing filename for metadata, enriching it with kubernetes metadata, reformatting data structure, and sending them (without compression) to Splunk HEC endpoint.

## Running the container in non-root user mode
Collecting logs often requires reading log files that are owned by the root user. By default, the container runs with `securityContext.runAsUser = 0` which gives the `root` user permission to read those files.
To run the container in `non-root` user mode, set `.agent.securityContext`. The log data permissions will be adjusted to match the securityContext configurations. For instance:
```yaml
agent:
  securityContext:
     runAsUser: 20000
     runAsGroup: 20000
```

Note: Running the collector agent for log collection in non-root mode is not currently supported in CRI-O and OpenShift environments at this time, for more details see the
[related GitHub feature request issue](https://github.com/signalfx/splunk-otel-collector-chart/issues/891).

## Searching for event metadata in Splunk Enterprise/Cloud
Splunk OpenTelemetry Collector for Kubernetes sends events to Splunk which can contain extra meta-data attached to each event. Metadata values such as "pod", "namespace", "container_name","container_id", "cluster_name" will appear as fields when viewing the event data inside Splunk.

Since Splunk version 9.0 searching for indexed fields is turned on by default so there shouldn't be a problem with searching for them.
If searching for indexed fields is turned off or you are running an older version of splunk, there are two solutions for running searches in Splunk on metadata:

* Modify search to use`fieldname::value` instead of `fieldname=value`.
* Configure `fields.conf` on your downstream Splunk system to have your meta-data fields available to be searched using `fieldname=value`. Examples: [Fieldsconf](https://docs.splunk.com/Documentation/Splunk/latest/admin/Fieldsconf)

For more information on index time field extraction please view this [guide](https://docs.splunk.com/Documentation/Splunk/latest/Data/Configureindex-timefieldextraction#Where_to_put_the_configuration_changes_in_a_distributed_environment).

## Additional telemetry sources

Use `autodetect` config option to enable additional telemetry sources.

Set `autodetect.prometheus=true` if you want the otel-collector agent to scrape
prometheus metrics from pods that have generic prometheus-style annotations:

- `prometheus.io/scrape: true`: Prometheus metrics will be scraped only from
  pods having this annotation;
- `prometheus.io/path`: path to scrape the metrics from, default `/metrics`;
- `prometheus.io/port`: port to scrape the metrics from, default `9090`.

Set `autodetect.istio=true`, if the otel-collector agent in running in Istio
environment, to make sure that all traces, metrics and logs reported by Istio
collected in a unified manner.

For example to enable both Prometheus and Istio telemetry add the following
lines to your `values.yaml` file:

```yaml
autodetect:
  istio: true
  prometheus: true
```

## Using feature gates

Enable or disable features of the otel-collector agent, clusterReceiver, and gateway (respectively) using feature
gates. Use the agent.featureGates, clusterReceiver.featureGates, and gateway.featureGates configs to enable or disable
features, these configs will be used to populate the otelcol binary startup argument "--feature-gates". For more
details see the
[feature gate documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/service/featuregate/README.md).

Helm Install Example:
```bash
helm install {name} --set agent.featureGates=+feature1 --set clusterReceiver.featureGates=feature2 --set gateway.featureGates=-feature2 {other_flags}
```
Would result in the agent having feature1 enabled, the clusterReceiver having feature2 enabled, and the gateway having
feature2 disabled.

## Override underlying OpenTelemetry agent configuration

If you want to use your own OpenTelemetry Agent configuration, you can override it by providing a custom configuration in the `agent.config` parameter in the values.yaml, which will be merged into the default agent configuration, list parts of the configuration (for example, `service.pipelines.logs.processors`) to be fully re-defined.

## Manually setting Pod Security Policy

Support of Pod Security Policies (PSP) was [removed](https://kubernetes.io/docs/concepts/security/pod-security-policy/)
in Kubernetes 1.25. If you still rely on PSPs in an older cluster, you can add them manually along with the helm chart
installation.

1. Run the following command to install the PSP (don't forget to add `--namespace` kubectl argument if needed):

```
cat <<EOF | kubectl apply -f -
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: splunk-otel-collector-psp
  labels:
    app: splunk-otel-collector-psp
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: 'runtime/default'
    apparmor.security.beta.kubernetes.io/allowedProfileNames: 'runtime/default'
    seccomp.security.alpha.kubernetes.io/defaultProfileName:  'runtime/default'
    apparmor.security.beta.kubernetes.io/defaultProfileName:  'runtime/default'
spec:
  privileged: false
  allowPrivilegeEscalation: false
  hostNetwork: true
  hostIPC: false
  hostPID: false
  volumes:
  - 'configMap'
  - 'emptyDir'
  - 'hostPath'
  - 'secret'
  runAsUser:
    rule: 'RunAsAny'
  seLinux:
    rule: 'RunAsAny'
  supplementalGroups:
    rule: 'RunAsAny'
  fsGroup:
    rule: 'RunAsAny'
EOF
```

2. Add the following custom ClusterRole rule in your values.yaml file along with all other required fields like
`clusterName`, `splunkObservability` or `splunkPlatform`:

```yaml
rbac:
  customRules:
    - apiGroups:     [extensions]
      resources:     [podsecuritypolicies]
      verbs:         [use]
      resourceNames: [splunk-otel-collector-psp]
```

3. Install the helm chart (assuming your custom values.yaml is called `my_values.yaml`):

```
helm install my-splunk-otel-collector -f my_values.yaml splunk-otel-collector-chart/splunk-otel-collector
```

## Data Persistence

By default, without any configuration, data is queued in memory only. When data cannot be sent it is retried a few times (up to 5 mins. by default) and then dropped.

If for any reason, the collector is restarted in this period, the queued data will be gone.

If you want the queue to be persisted on disk across collector restarts, set `splunkPlatform.sendingQueue.persistentQueue.enabled` to enable support for logs, metrics and traces.

By default, data is persisted in `/var/addon/splunk/exporter_queue` directory.
Override this behaviour by setting `splunkPlatform.sendingQueue.persistentQueue.storagePath` option.

Check [Data Persistence in the OpenTelemetry Collector
](https://community.splunk.com/t5/Community-Blog/Data-Persistence-in-the-OpenTelemetry-Collector/ba-p/624583) for detailed explantion.

Note: Data Persistence is only applicable for agent daemonset.

Use following in values.yaml to disable data persistense for logs or metrics or traces:

```yaml
agent:
  config:
    exporters:
       splunk_hec/platform_logs:
         sending_queue:
           storage: null
```
or
```yaml
agent:
  config:
    exporters:
       splunk_hec/platform_metrics:
         sending_queue:
           storage: null
```
or
```yaml
agent:
  config:
    exporters:
       splunk_hec/platform_traces:
         sending_queue:
           storage: null
```

### Support for persistent queue

* `GKE/Autopilot` and `EKS/Fargate` support
  * Both of the above distributions doesn't allow volume mounts, as they are kind of `serverless` and we don't manage the underlying infrastructure.
  * Persistent buffering is not supported for them, as directory needs to be mounted via `hostPath`.
  * Refer [aws/fargate](https://docs.aws.amazon.com/eks/latest/userguide/fargate.html) and [gke/autopilot](https://cloud.google.com/kubernetes-engine/docs/concepts/autopilot-security#built-in-security).
* Gateway support
  * The filestorage extention acquires an exclusive lock for the queue directory.
  * It is not possible to run the persistent buffering if there are multiple replicas of a pod and `gateway` runs 3 replicas by default.
  * Even if support is somehow provided, only one of the pods will be able to acquire the lock and run, while the others will be blocked and unable to operate.
* Cluster Receiver support
  * Cluster receiver is a 1-replica deployment of Open-temlemetry collector.
  * As any available node can be selected by the Kubernetes control plane to run the cluster receiver pod (unless we explicitly specify the `clusterReceiver.nodeSelector` to pin the pod to a specific node), `hostPath` or `local` volume mounts wouldn't work for such envrionments.
  * Data Persistence is currently not applicable to the k8s cluster metrics and k8s events.

### Using OpenTelemetry eBPF helm chart with Splunk OpenTelemetry Collector for Kubernetes

[OpenTelemetry eBPF Helm Chart](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-ebpf)
can be used to collect network metrics from linux kernel. The metrics collected by eBPF can be
sent to Splunk OpenTelemetry Collector for Kubernetes gateway. The gateway will then forward the metrics to Splunk
Observability Cloud or Splunk Enterprise.

To use the OpenTelemetry eBPF helm chart with Splunk OpenTelemetry Collector for Kubernetes, follow the steps below:

1. Make sure the Splunk OpenTelemetry Collector helm chart is installed with the gateway enabled:

```yaml
gateway:
  enabled: true
```

2. Grab name of the Splunk OpenTelemetry Collector gateway service:

```bash
kubectl get svc | grep splunk-otel-collector-gateway
```

3. Install the upstream OpenTelemetry eBPF helm chart pointing to the Splunk OpenTelemetry Collector gateway service:

```bash
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update open-telemetry
helm install my-opentelemetry-ebpf --set=endpoint.address=<my-splunk-otel-collector-gateway> open-telemetry/opentelemetry-ebpf
```

where <my-splunk-otel-collector-gateway> is the gateway service name captured in the step 2.
