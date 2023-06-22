# Example of chart configuration

## Istio environment

If the Splunk OTel Collector is installed in the cluster with istio, the following configuration is
recommended to scrape istio control plane metrics and ensure that all traces, metrics and logs 
reported by Istio collected in a unified manner.

```yaml 
autodetect:
  istio: true
```