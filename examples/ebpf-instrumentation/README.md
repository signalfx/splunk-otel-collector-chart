# OpenTelemetry eBPF Instrumentation Example

This example demonstrates how to enable zero-code instrumentation using [OpenTelemetry eBPF Instrumentation] (OBI) with the Splunk OpenTelemetry Collector Helm chart.

See the [Zero-Code eBPF Instrumentation Documentation] for more details about deploying and troubleshooting OBI.

[OpenTelemetry eBPF Instrumentation]: https://opentelemetry.io/docs/zero-code/obi/
[Zero-Code eBPF Instrumentation Documentation]: https://opentelemetry.io/docs/zero-code/ebi/

## Installation

First check that you meet the [platform requirements].
Then install the chart with OBI enabled using the provided `values.yaml`:

```bash
# Install the chart with eBPF instrumentation enabled
helm install splunk-otel-collector splunk-otel-collector-chart/splunk-otel-collector \
  -f examples/ebpf-instrumentation/values.yaml \
  --set="splunkObservability.realm=<SPLUNK_REALM>" \
  --set="splunkObservability.accessToken=<SPLUNK_ACCESS_TOKEN>" \
  --set="clusterName=<CLUSTER_NAME>" \
  --set="environment=<ENVIRONMENT>"
```

Refer to OBI [documentation for deployment on AKS/EKS](https://opentelemetry.io/docs/zero-code/obi/security/#deploy-on-akseks) if needed.

[platform requirements]: ../../docs/zero-code-ebpf-instrumentation.md#platform-requirements

## Configuration Options

The example enables basic application observability.
Additional features can be configured by modifying the `obi.config.data` section in `values.yaml`.

For complete configuration options, refer to the [OBI Chart documentation].

[OBI Chart documentation]: https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-ebpf-instrumentation

## Verification

After installation, verify that OBI pods are running:

```bash
# Check if eBPF instrumentation pods are running
kubectl get pods -l app.kubernetes.io/name=obi

# Check pod logs for any warnings about missing capabilities
kubectl logs -l app.kubernetes.io/name=obi -f
```

If the pods are running and logs show no errors, OBI is successfully deployed.

See the [troubleshooting documentation] for help with common issues.

[troubleshooting documentation]: ../../docs/zero-code-ebpf-instrumentation.md#troubleshooting
