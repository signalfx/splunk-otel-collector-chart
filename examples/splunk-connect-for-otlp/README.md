# Example of chart configuration

## Send logs to Splunk Platform through Splunk Connect for OTLP

This example configures the collector to send Kubernetes logs to a Splunk Connect
for OTLP Technical Add-on endpoint instead of sending logs directly to Splunk
Platform with HEC.

The Splunk Connect for OTLP Technical Add-on must be installed and configured in
Splunk Enterprise or Splunk Cloud Platform with an OTLP receiver reachable from
the Kubernetes cluster. Set `splunkPlatform.otlpIngest.endpoint` to that receiver,
for example `splunk-connect-for-otlp.example.com:4317` for gRPC or
`http://splunk-connect-for-otlp.example.com:4318` for HTTP.

With this configuration, logs use the rendered `otlp/platform_logs` exporter.
The `splunkPlatform.endpoint` and `splunkPlatform.token` HEC settings are not
required unless metrics or traces are also sent to Splunk Platform.
