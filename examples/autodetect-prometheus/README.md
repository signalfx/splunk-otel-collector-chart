# Example of chart configuration

## Deploy the OTel Collector in with automatic detection of additional metric sources

This configuration will install the collector with the default settings and enable automatic detection of pods
that have prometheus-style annotations like "prometheus.io/scrape"

```yaml
autodetect:
  prometheus: true
```
