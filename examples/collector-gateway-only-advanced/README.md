# Example of chart configuration

## Enable OTel Collector in the gateway mode with more advanced options
This configuration installs collector as a gateway deployment along with
regular components. All the telemetry will be routed through this collector.
By default, the gateway-mode collector deployed with 3 replicas with 4 CPU
cores and 8Gb of memory each, but this can be easily changed as in this example.
`resources` can be adjusted for other components as well: `agent`,
`clusterReceiver`, `fluentd`.
