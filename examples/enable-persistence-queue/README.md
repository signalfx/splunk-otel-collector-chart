# Example of chart configuration

## Enable Persistent Queue for logs, metrics and traces

This example will show how to enable data persistence for log/metric/trace data. Data will be persisted in node's local filesystem. 
Persistent queue will keep track of unexported data. It will continue from previously saved offsets, if any, after collector restarts and export it to Splunk platform.

Refer to: https://github.com/signalfx/splunk-otel-collector-chart/blob/main/docs/advanced-configuration.md#data-persistence
