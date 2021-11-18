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
