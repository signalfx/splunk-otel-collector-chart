# Example of chart configuration

## Multiline logs configuration

This example demonstrates how to configure multiline log processing for container logs.
Multiline logs (such as stack traces) that are written by containers to stdout are typically 
broken down into several one-line logs. The multilineConfigs feature allows you to recombine 
them using regex patterns that match the first line of each log batch.

The example shows three different configurations for different namespaces and pods, including
how to specify custom separators when recombining log lines.
