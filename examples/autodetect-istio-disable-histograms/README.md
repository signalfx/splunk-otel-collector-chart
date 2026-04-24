# Example of chart configuration

## Istio environment without control plane histograms

If the Splunk OTel Collector is installed in the cluster with Istio, the following configuration is
recommended to scrape Istio control plane metrics and ensure that all traces, metrics, and logs
reported by Istio are collected in a unified manner.

Disable the `useControlPlaneMetricsHistogramData` to not report control plane metrics as histograms.

```yaml
autodetect:
  istio: true

featureGates:
  useControlPlaneMetricsHistogramData: false
```
