# Example of chart configuration

## Send Kubernetes events to Splunk Observability /v3/event

This configuration enables the `sendK8sEventsToSplunkO11y` feature gate, which configures
the existing `logs` pipeline in the cluster receiver to route Kubernetes events to the
Splunk Observability `/v3/event` endpoint. It adds an additional `otlp_http/o11y_events`
exporter to the pipeline that collects events via the `k8s_events` receiver and sends them
with the `o11yevents` routing header.

### Prerequisites

- `splunkObservability.realm` and `splunkObservability.accessToken` must be set.
- `clusterReceiver.eventsEnabled` must be `true` (default).

### Key values

```yaml
featureGates:
  sendK8sEventsToSplunkO11y: true
```

The `logs` pipeline applies the same processors (`attributes/drop_event_attrs`,
`transform/k8sevents`, etc.) whether exporting to Splunk Platform or Splunk Observability,
so events arrive at the /v3/event endpoint enriched with the same k8s metadata.
