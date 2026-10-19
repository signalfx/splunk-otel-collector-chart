package internal

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// RemoveFlakyMetrics removes all metrics with names in flakyMetrics input.
func RemoveFlakyMetrics(metrics *pmetric.Metrics, flakyMetrics []string) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		resourceMetrics := metrics.ResourceMetrics().At(i)
		for j := 0; j < resourceMetrics.ScopeMetrics().Len(); j++ {
			scopeMetrics := resourceMetrics.ScopeMetrics().At(j)
			metricSlice := scopeMetrics.Metrics()
			metricSlice.RemoveIf(func(metric pmetric.Metric) bool {
				for _, flakyMetric := range flakyMetrics {
					if metric.Name() == flakyMetric {
						return true
					}
				}
				return false
			})
		}
	}
}

// RetainNumberMetricDatapointsForPod keeps number datapoints for one pod.
func RetainNumberMetricDatapointsForPod(metrics *pmetric.Metrics, podNamePrefix string) {
	removeDatapoint := func(attributes pcommon.Map) bool {
		podName, ok := attributes.Get("k8s.pod.name")
		return !ok || podName.Type() != pcommon.ValueTypeStr || !strings.HasPrefix(podName.Str(), podNamePrefix)
	}

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		scopeMetrics := metrics.ResourceMetrics().At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			metricSlice := scopeMetrics.At(j).Metrics()
			for k := 0; k < metricSlice.Len(); k++ {
				metric := metricSlice.At(k)
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					metric.Gauge().DataPoints().RemoveIf(func(datapoint pmetric.NumberDataPoint) bool {
						return removeDatapoint(datapoint.Attributes())
					})
				case pmetric.MetricTypeSum:
					metric.Sum().DataPoints().RemoveIf(func(datapoint pmetric.NumberDataPoint) bool {
						return removeDatapoint(datapoint.Attributes())
					})
				case pmetric.MetricTypeEmpty, pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeSummary:
				}
			}
		}
	}
}
