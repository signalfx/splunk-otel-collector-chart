# Troubleshooting

This document covers troubleshooting scenarios specific to Kubernetes
environment only. For general troubleshooting of Splunk OpenTelemetry Collector
see [Splunk OpenTelemetry Collector troubleshooting
documentation](https://github.com/signalfx/splunk-otel-collector/blob/main/docs/troubleshooting.md).

## Gathering Support Information

Existing Splunk Observability customers unable to determine why something is not
working can [email support](mailto:signalfx-support@splunk.com). Splunk
Enterprise/Cloud customers with Splunk support entitlement can reach out to
[Splunk
Support](https://www.splunk.com/en_us/about-splunk/contact-us.html#tabs/tab_parsys_tabs_CustomerSupport_4)

When opening a support request, it is important to include as much information
about the issue as possible including:

- What did you try to do?
- What happened?
- What did you expect to happen?
- Have you found any workaround?
- How impactful is the issue?
- How can we reproduce the issue?

In addition, it is important to gather support information including:

- Which destination is configured: Splunk Platform or Splunk Observability?
- Helm chart version.
- Custom `values.yaml` file that was applied with `helm install` command or `--set`
  arguments.
- Are there any manual customization done to the Kubernetes resources once the
  chart is installed?
- Kubernetes cluster details:
  - Kubernetes version.
  - Managed or on premises: if managed, which cloud provider and distribution?
- Logs from problematic pods:
  - `kubectl logs my-splunk-otel-collector-agent-fzn4q otel-collector > my-splunk-otel-collector-agent.log`
  - `kubectl logs my-splunk-otel-collector-agent-fzn4q fluentd > my-splunk-otel-collector-agent-fluentd.log`
  - `kubectl logs my-splunk-otel-collector-k8s-cluster-receiver-7545499bc7-vqdsl > my-splunk-otel-collector-k8s-cluster-receiver.log`

Support bundle scripts are provided to make it easier to collect information:

- Kubernetes: [kubectl-splunk](https://github.com/signalfx/kubectl-splunk/blob/main/docs/kubectl-splunk_support.md)

## Debug logging for the Splunk Otel Collector in Kubernetes
You can change the logging level of the collector from info to debug for troubleshooting. This is done by setting the
[service.telemetry.logs.level](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/troubleshooting.md)
configuration.

The collector's logs are not exported by default. If you already export your logs to Splunk Platform or Splunk
Observability, then you may want to export the collector's logs too. This is done by setting the
[logCollection.container.excludeAgentLogs](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/helm-charts/splunk-otel-collector/values.yaml)
configuration.

Here is a configuration example that enables the collector to output debug logs and export them to Splunk Platform or
Splunk Observability.
```yaml
agent:
  config:
    service:
      telemetry:
        logs:
          # Enable debug logging from the collector.
          level: debug
# Optional for exporting logs.
logsCollection:
  containers:
    # Enable the logs from the collector/agent to be collected at the container level.
    excludeAgentLogs: false
```
View the logs using:
- `kubectl logs {splunk-otel-collector-agent-pod}`

## Sizing Splunk OTel Collector containers

Resources allocated to Splunk OTel Collector should be set based on the amount
of data it's expected to handle, see [sizing
guidelines](https://github.com/signalfx/splunk-otel-collector/blob/main/docs/sizing.md).
Use the following configuration to bump resource limits for the agent:

```yaml
agent:
  resources:
    limits:
      cpu: 500m
      memory: 1Gi
```

Resources allocated to cluster receiver deployment should be based on the
cluster size. For a cluster with 100 nodes you would need the following
resources:

```yaml
clusterReceiver:
  resources:
    limits:
      cpu: 1
      memory: 2Gi
```

## Possible problems with Splunk OTel Collector containers

### Splunk OTel Collector container running out of memory

Even if you didn't provide enough resources the OTel Collector containers, under
normal circumstances collector should not run out of memory. OOM can happen only
if the collector is heavily throttled by the backend and exporter sending queue
growing faster than collector can control memory utilization. In that case you
should see 429 errors for metrics and traces or 503 errors for logs. Similar to
the following:

```
2021-11-12T00:22:32.172Z	info	exporterhelper/queued_retry.go:325	Exporting failed. Will retry the request after interval.	{"kind": "exporter", "name": "sapm", "error": "server responded with 429", "interval": "4.4850027s"}
2021-11-12T00:22:38.087Z	error	exporterhelper/queued_retry.go:190	Dropping data because sending_queue is full. Try increasing queue_size.	{"kind": "exporter", "name": "sapm", "dropped_items": 1348}
```

If throttling cannot be fixed by bumping limits on the backend or reducing
amount of data sent through the collector, you can avoid OOMs by reducing
sending queue of the failing exporter, e.g. to reduce `sending_queue` for the
`sapm` exporter (tracing):

```
agent:
  config:
    exporters:
      sapm:
        sending_queue:
          queue_size: 512
```

Similar can be applied to any other failing exporter.

## Possible problems with Kubernetes and container runtimes

A Kubernetes cluster using an incompatible container runtime for its version or
configuration could experience these issues cluster-wide:
- Stats from containers, pods, or nodes being absent or malformed. As a result,
  the Splunk OTel Collector that requires these stats will not produce the
  desired corresponding metrics.
- Containers, pods, and nodes failing to start successfully or stop cleanly.
- The Kubelet process on a node being in a defunct state.

Kubernetes requires you to install a
[container runtime](https://kubernetes.io/docs/setup/production-environment/container-runtimes/)
on each node in the cluster so that pods can run there. Multiple container
runtimes such as containerd, CRI-O, Docker, and Marantis (formerly Docker
Engine – Enterprise) are well-supported. The compatability level of a specific
Kubernetes version and container runtime can vary, it is recommended to use a
version of Kubernetes and a container runtime that has been documented to be
compatible.

### Troubleshooting Kubernetes and container runtime incompatibility

- Find out what Kubernetes and container runtime are being used.
   - In the example below, node-1 uses Kubernetes 1.19.6 and containerd 1.4.1.
      ```
      kubectl get nodes -o wide
      NAME         STATUS   VERSION   CONTAINER-RUNTIME
      node-1       Ready    v1.19.6   containerd://1.4.1
      ```
- Verify that you are using a container runtime that has been documented to
  work with your Kubernetes version. Container runtime creators document
  compatibility in their respective projects, you can view the documentation for
  the mentioned container runtimes with the links below.
   - [containerd](https://containerd.io/releases/#kubernetes-support)
   - [CRI-O](https://github.com/cri-o/cri-o#compatibility-matrix-cri-o--kubernetes)
   - [Mirantis](https://docs.mirantis.com/container-cloud/latest/compat-matrix.html)
- Use the Kubelet "summary" API to verify container, pod, and node stats.
  - In this section we will verify the cpu, memory, and networks stats that are
    used to generate the
    [Kubelet Stats Receiver metrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/kubeletstatsreceiver/documentation.md#metrics)
    by the collector are present. You can expand these techniques to evaluate
    other Kubernetes stats that are available. All the stats in these commands
    and sample outputs below should be present unless otherwise noted. If your
    output is missing stats or your stat values appear to be in a different
    format, your Kubernetes cluster and container runtime might not be fully
    compatible.
    <details>
    <summary>1) Verify a node's stats</summary>

    ```
    # Get the names of the nodes in your cluster.
    kubectl get nodes -o wide
    # Pick a node to evaluate and set its name to an environment variable.
    NODE_NAME=node-1
    # Verify the node has proper stats with this command and sample output.
    kubectl get --raw "/api/v1/nodes/"${NODE_NAME}"/proxy/stats/summary" | jq '{"node": {"name": .node.nodeName, "cpu": .node.cpu, "memory": .node.memory, "network": .node.network}} | del(.node.network.interfaces)'
    {
      "node": {
        "name": "node-1",
        "cpu": {
          "time": "2022-05-20T18:12:08Z",
          "usageNanoCores": 149771849,
          "usageCoreNanoSeconds": 2962750554249399
        },
        "memory": {
          "time": "2022-05-20T18:12:08Z",
          "availableBytes": 2701385728,  # Could be absent if node memory allocations were missing.
          "usageBytes": 3686178816,
          "workingSetBytes": 1421492224,
          "rssBytes": 634343424,
          "pageFaults": 18632526,
          "majorPageFaults": 726
        },
        "network": {
          "time": "2022-05-20T18:12:08Z",
          "name": "eth0",
          "rxBytes": 105517219156,
          "rxErrors": 0,
          "txBytes": 98151853779,
          "txErrors": 0
        }
      }
    }

    # For reference, here is the mapping for the node stat names to the Splunk Otel Collector metric names.
    # cpu.usageNanoCores        -> k8s.node.cpu.utilization
    # cpu.usageCoreNanoSeconds  -> k8s.node.cpu.time
    # memory.availableBytes     -> k8s.node.memory.available
    # memory.usageBytes         -> k8s.node.filesystem.usage
    # memory.workingSetBytes    -> k8s.node.memory.working_set
    # memory.rssBytes           -> k8s.node.memory.rss
    # memory.pageFaults         -> k8s.node.memory.page_faults
    # memory.majorPageFaults    -> k8s.node.memory.major_page_faults
    # network.rxBytes           -> k8s.node.network.io{direction="receive"}
    # network.rxErrors          -> k8s.node.network.errors{direction="receive"}
    # network.txBytes           -> k8s.node.network.io{direction="transmit"}
    # network.txErrors          -> k8s.node.network.error{direction="transmit"}
    ```
    </details>

    <details>
    <summary>2) Verify a pod's stats</summary>

    ```
    # Get the names of the pods in your node.
    kubectl get --raw "/api/v1/nodes/"${NODE_NAME}"/proxy/stats/summary" | jq '.pods[].podRef.name'
    # Pick a pod to evaluate and set its name to an environment variable.
    POD_NAME=splunk-otel-collector-agent-6llkr
    # Verify the pod has proper stats with this command and sample output.
    kubectl get --raw "/api/v1/nodes/"${NODE_NAME}"/proxy/stats/summary" | jq '.pods[] | select(.podRef.name=='\"$POD_NAME\"') | {"pod": {"name": .podRef.name, "cpu": .cpu, "memory": .memory, "network": .network}} | del(.pod.network.interfaces)'
    {
      "pod": {
        "name": "splunk-otel-collector-agent-6llkr",
        "cpu": {
          "time": "2022-05-20T18:38:47Z",
          "usageNanoCores": 10774467,
          "usageCoreNanoSeconds": 1709095026234
        },
        "memory": {
          "time": "2022-05-20T18:38:47Z",
          "availableBytes": 781959168, # Could be absent if pod memory limits were missing.
          "usageBytes": 267563008,
          "workingSetBytes": 266616832,
          "rssBytes": 257036288,
          "pageFaults": 0,
          "majorPageFaults": 0
        },
        "network": {
          "time": "2022-05-20T18:38:55Z",
          "name": "eth0",
          "rxBytes": 105523812442,
          "rxErrors": 0,
          "txBytes": 98159696431,
          "txErrors": 0
        }
      }
    }

    # For reference, here is the mapping for the pod stat names to the Splunk Otel Collector metric names.
    # Some of these metrics have a current and a legacy name, current names will be listed first.
    # pod.cpu.usageNanoCores        -> k8s.pod.cpu.utilization
    # pod.cpu.usageCoreNanoSeconds  -> k8s.pod.cpu.time
    # pod.memory.availableBytes     -> k8s.pod.memory.available
    # pod.memory.usageBytes         -> k8s.pod.filesystem.usage
    # pod.memory.workingSetBytes    -> k8s.pod.memory.working_set
    # pod.memory.rssBytes           -> k8s.pod.memory.rss
    # pod.memory.pageFaults         -> k8s.pod.memory.page_faults
    # pod.memory.majorPageFaults    -> k8s.pod.memory.major_page_faults
    # pod.network.rxBytes           -> k8s.pod.network.io{direction="receive"} or pod_network_receive_bytes_total
    # pod.network.rxErrors          -> k8s.pod.network.errors{direction="receive"} or pod_network_receive_errors_total
    # pod.network.txBytes           -> k8s.pod.network.io{direction="transmit"} or pod_network_transmit_bytes_total
    # pod.network.txErrors          -> k8s.pod.network.error{direction="transmit"} or pod_network_transmit_errors_total
    ```

    </details>

    <details>
    <summary>3) Verify a container's stats</summary>

    ```
    # Get the names of the containers in your pod.
    kubectl get --raw "/api/v1/nodes/"${NODE_NAME}"/proxy/stats/summary" | jq '.pods[] | select(.podRef.name=='\"$POD_NAME\"') | .containers[].name'
    # Pick a container to evaluate and set it's name to an enviroment variable.
    CONTAINER_NAME=otel-collector
    # Verify the container has proper stats with this command and sample output.
    kubectl get --raw "/api/v1/nodes/"${NODE_NAME}"/proxy/stats/summary" | jq '.pods[] | select(.podRef.name=='\"$POD_NAME\"') | .containers[] | select(.name=='\"$CONTAINER_NAME\"') | {"container": {"name": .name, "cpu": .cpu, "memory": .memory}}'
    {
      "container": {
        "name": "otel-collector",
        "cpu": {
          "time": "2022-05-20T18:42:15Z",
          "usageNanoCores": 6781417,
          "usageCoreNanoSeconds": 1087899649154
        },
        "memory": {
          "time": "2022-05-20T18:42:15Z",
          "availableBytes": 389480448, # Could be absent if container memory limits were missing.
          "usageBytes": 135753728,
          "workingSetBytes": 134807552,
          "rssBytes": 132923392,
          "pageFaults": 93390,
          "majorPageFaults": 0
        }
      }
    }

    # For reference, here is the mapping for the container stat names to the Splunk Otel Collector metric names.
    # container.cpu.usageNanoCores        -> container.cpu.utilization
    # container.cpu.usageCoreNanoSeconds  -> container.cpu.time
    # container.memory.availableBytes     -> container.memory.available
    # container.memory.usageBytes         -> container.memory.usage
    # container.memory.workingSetBytes    -> container.memory.working_set
    # container.memory.rssBytes           -> container.memory.rss
    # container.memory.pageFaults         -> container.memory.page_faults
    # container.memory.majorPageFaults    -> container.memory.major_page_faults
    ```
    </details>

### Reported incompatible Kubernetes and container runtime issues

- Note: Managed Kubernetes services might use a modified container runtime,
  the service provider may have applied custom patches or bug fixes that aren't
  present within an unmodified container runtime.
- Kubernetes 1.21.0-1.21.11 using containerd - Memory and network stats/metrics
  can be missing
  <details>
  <summary>Expand for more details</summary>

  - Affected metrics:
      - k8s.pod.network.io{direction="receive"} or
        pod_network_receive_bytes_total
      - k8s.pod.network.errors{direction="receive"} or
        pod_network_receive_errors_total
      - k8s.pod.network.io{direction="transmit"} or
        pod_network_transmit_bytes_total
      - k8s.pod.network.error{direction="transmit"} or
        pod_network_transmit_errors_total
      - container.memory.available
      - container.memory.usage
      - container.memory.rssBytes
      - container.memory.page_faults
      - container.memory.major_page_faults
  - Resolutions:
    - Upgrading Kubernetes to at least 1.21.12 fixed all the missing metrics.
    - Upgrading containerd to a newer version of 1.4.x or 1.5.x is still
      recommended.
  </details>
- Kubernetes 1.22.0-1.22.8 using containerd 1.4.0-1.4.12 - Memory and network
  stats/metrics can be missing
  <details>
  <summary>Expand for more details</summary>

  - Affected metrics:
    - k8s.pod.network.io{direction="receive"} or
      pod_network_receive_bytes_total
    - k8s.pod.network.errors{direction="receive"} or
      pod_network_receive_errors_total
    - k8s.pod.network.io{direction="transmit"} or
      pod_network_transmit_bytes_total
    - k8s.pod.network.error{direction="transmit"} or
      pod_network_transmit_errors_total
    - k8s.pod.memory.available
    - container.memory.available
    - container.memory.usage
    - container.memory.rssBytes
    - container.memory.page_faults
    - container.memory.major_page_faults
  - Resolutions:
    - Upgrading Kubernetes to at least 1.22.9 fixed the missing container
      memory and pod network metrics.
    - Upgrading containerd to at least 1.4.13 or 1.5.0 fixed the missing pod
      memory metrics.
  </details>
- Kubernetes 1.23.0-1.23.6 using containerd - Memory stats/metrics can be
  missing
  <details>
  <summary>Expand for more details</summary>

  - Affected metrics:
    - k8s.pod.memory.available
  - Resolutions:
    - No resolutions have been documented as of 2022-05-2.
  </details>
