# Example of chart configuration

## Enable traces sampling

This example shows how to change default OTel Collector configuration to add
[Probabilistic Sampling Processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/probabilisticsamplerprocessor).
This approach can be used for any other OTel Collector re-configuration as well.
Final OTel config will be created by merging the custom config provided in
`agent.config` into [default configuration of agent-mode
collector](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/helm-charts/splunk-otel-collector/templates/config/_otel-agent.tpl).

In the example, first we define a new processor, then add it to the
default traces pipeline. The pipeline has to be fully redefined, because
lists cannot merge - they have to be overridden.
