# Example of chart configuration

## Deploy the OTel Collector in gateway mode only
This configuration will install collector as a gateway deployment only.
No metrics (except internal collector metrics) or logs will be collected from
the gateway instance(s), the gateway can be used to forward telemetry data
through it for aggregation, enrichment purposes.
