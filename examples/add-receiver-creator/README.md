# Example of chart configuration

## Add Receiver Creator
This example shows how to add a receiver creator to the OTel Collector configuration
[Receiver Creator](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/receivercreator).
In this example we will configure it to observe kubernetes pods and in case there is a pod
using port 5432 the collector will dynamically create a smartagent/postgresql receiver to monitor it
and using port 5433 the collector will dynamically create an OpenTelemetry postgresql receiver.

By default, the receiver_creator receiver is part of the metrics pipeline, for example:
```yaml
pipelines:
  metrics:
    receivers:
      - hostmetrics
      - kubeletstats
      - otlp
      - receiver_creator
      - signalfx
```
