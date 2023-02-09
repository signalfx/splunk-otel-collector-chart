# Example of chart configuration

## Enable OTel Collector in gateway mode with more advanced options

This configuration installs the collector as a gateway deployment along with
regular components. All the telemetry will be routed through this collector.

By default, the gateway-mode collector is deployed with 3 replicas, 4 CPU cores,
and 8Gb of memory each, but this can be easily changed as in this example.
`resources` can be adjusted for other components as well: `agent`,
`clusterReceiver`, `fluentd`.
In this example we modify the gateway-mode collector to be deployed with 1
replicas, 2 CPU cores, and 4Gb of memory.
