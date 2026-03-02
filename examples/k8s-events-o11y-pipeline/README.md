# Example of chart configuration

## Send Kubernetes events to Splunk Observability events/v3

This configuration enables the `sendK8sEventsToO11yEvents` feature gate, which creates a
dedicated `logs/k8s_events_o11y` pipeline in the cluster receiver. The pipeline collects
Kubernetes events via the `k8s_events` receiver and routes them to the Splunk Observability
`events/v3` endpoint using an OTLP HTTP exporter with an `o11yevents` routing header.

### Prerequisites

- `splunkObservability.realm` and `splunkObservability.accessToken` must be set.
- `clusterReceiver.eventsEnabled` must be `true` (default).

### Key values

```yaml
featureGates:
  sendK8sEventsToO11yEvents: true
```

The rendered `logs/k8s_events_o11y` pipeline applies the same processors as the standard
k8s events logs pipeline (`attributes/drop_event_attrs`, `transform/k8sevents`, etc.) so
events arrive at the events/v3 endpoint enriched with the same k8s metadata.
