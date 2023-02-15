# Example of chart configuration

## Only collect logs with the otel logs engine

This example shows how you can use Otel logs instead of fluentd logs (no metric,
no traces). Otel logs get higher throughput performance and avoid installing an
extra container compared to fluentd.
