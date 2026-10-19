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

// RetainNumberMetricDatapointsForPod keeps number datapoints for one pod and
// removes number metrics, scopes, and resources left empty by the filter.
func RetainNumberMetricDatapointsForPod(metrics *pmetric.Metrics, podNamePrefix string) {
	removeDatapoint := func(attributes pcommon.Map) bool {
		podName, ok := attributes.Get("k8s.pod.name")
		return !ok || podName.Type() != pcommon.ValueTypeStr || !strings.HasPrefix(podName.Str(), podNamePrefix)
	}

	metrics.ResourceMetrics().RemoveIf(func(resourceMetrics pmetric.ResourceMetrics) bool {
		resourceMetrics.ScopeMetrics().RemoveIf(func(scopeMetrics pmetric.ScopeMetrics) bool {
			scopeMetrics.Metrics().RemoveIf(func(metric pmetric.Metric) bool {
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					metric.Gauge().DataPoints().RemoveIf(func(datapoint pmetric.NumberDataPoint) bool {
						return removeDatapoint(datapoint.Attributes())
					})
					return metric.Gauge().DataPoints().Len() == 0
				case pmetric.MetricTypeSum:
					metric.Sum().DataPoints().RemoveIf(func(datapoint pmetric.NumberDataPoint) bool {
						return removeDatapoint(datapoint.Attributes())
					})
					return metric.Sum().DataPoints().Len() == 0
				case pmetric.MetricTypeEmpty, pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeSummary:
				}
				return false
			})
			return scopeMetrics.Metrics().Len() == 0
		})
		return resourceMetrics.ScopeMetrics().Len() == 0
	})
}
