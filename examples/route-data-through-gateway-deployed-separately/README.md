# Example of chart configuration

## Route telemetry data through a gateway deployed separately
The following configuration can be used to forward telemetry through an OTel
collector gateway deployed separately.
OTLP format is used between agent and gateway wherever possible for performance
reasons. OTLP is almost the same as internal data representation in OTel
Collector, so using it between agent and gateway reduce CPU cycles spent on
data format transformations.
