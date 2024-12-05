# OpenTelemetry Collector CRDs

This chart contains the CRDs for _*installation*_ only right now for the opentelemetry-operator. This allows the Splunk OpenTelemetry Collector chart to work on install. You can see more discussion about this [here](https://github.com/open-telemetry/opentelemetry-helm-charts/issues/677) and [here](https://github.com/open-telemetry/opentelemetry-helm-charts/pull/1203).

This approach is inspired by the [opentelemetry-kube-stack chart](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-kube-stack) approach.

> [!NOTE]
> The splunk-otel-collector chart only supports and tests functionality related to the auto-instrumentation that requires the Instrumentation CRD.
> Other CRDs, such as OpenTelemetryCollector and OpAMPBridge, are included solely to allow the Operator to start up and are not currently supported or tested.

# Upgrade Notes

Right now, upgrades are NOT handled by this chart, however that could change in the future. This is what is run to bring in the CRDs today.

```bash
wget https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/main/config/crd/bases/opentelemetry.io_opentelemetrycollectors.yaml
wget https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/main/config/crd/bases/opentelemetry.io_opampbridges.yaml
wget https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/main/config/crd/bases/opentelemetry.io_instrumentations.yaml
```
