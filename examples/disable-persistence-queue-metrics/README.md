# Example of chart configuration

## Disable Persistent Queue for metrics only

This example will show how to disable data persistence for metrics data.
Persistent queue will keep track of unexported data. It will continue from previously saved offsets, if any, after collector restarts and export it to Splunk platform.

Refer to: https://github.com/signalfx/splunk-otel-collector-chart/blob/main/docs/advanced-configuration.md#data-persistence
