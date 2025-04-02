package internal

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// ReduceDatapoints reduces the number of datapoints found in any metric in input to maxDPCount.
func ReduceDatapoints(metrics *pmetric.Metrics, maxDPCount int) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					dp := metric.Sum().DataPoints()
					if dp.Len() > maxDPCount {
						newDP := pmetric.NewNumberDataPointSlice()
						// Copy the first maxDPCount data points to the new slice
						for l := 0; l < maxDPCount; l++ {
							dp.At(l).CopyTo(newDP.AppendEmpty())
						}
						// Remove all existing data points
						dp.RemoveIf(func(pmetric.NumberDataPoint) bool { return true })
						// Copy the reduced data points back
						for l := 0; l < newDP.Len(); l++ {
							newDP.At(l).CopyTo(dp.AppendEmpty())
						}
					}
				case pmetric.MetricTypeGauge:
					dp := metric.Gauge().DataPoints()
					if dp.Len() > maxDPCount {
						newDP := pmetric.NewNumberDataPointSlice()
						for l := 0; l < maxDPCount; l++ {
							dp.At(l).CopyTo(newDP.AppendEmpty())
						}
						dp.RemoveIf(func(pmetric.NumberDataPoint) bool { return true })
						for l := 0; l < newDP.Len(); l++ {
							newDP.At(l).CopyTo(dp.AppendEmpty())
						}
					}
				case pmetric.MetricTypeHistogram:
					dp := metric.Histogram().DataPoints()
					if dp.Len() > maxDPCount {
						newDP := pmetric.NewHistogramDataPointSlice()
						for l := 0; l < maxDPCount; l++ {
							dp.At(l).CopyTo(newDP.AppendEmpty())
						}
						dp.RemoveIf(func(pmetric.HistogramDataPoint) bool { return true })
						for l := 0; l < newDP.Len(); l++ {
							newDP.At(l).CopyTo(dp.AppendEmpty())
						}
					}
				case pmetric.MetricTypeSummary:
					dp := metric.Summary().DataPoints()
					if dp.Len() > maxDPCount {
						newDP := pmetric.NewSummaryDataPointSlice()
						for l := 0; l < maxDPCount; l++ {
							dp.At(l).CopyTo(newDP.AppendEmpty())
						}
						dp.RemoveIf(func(pmetric.SummaryDataPoint) bool { return true })
						for l := 0; l < newDP.Len(); l++ {
							newDP.At(l).CopyTo(dp.AppendEmpty())
						}
					}
				}
			}
		}
	}
}

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

// GetMetricNames returns a slice of unique metric names from the input metrics.
func GetMetricNames(metrics *pmetric.Metrics) []string {
	names := make(map[string]struct{})
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		for j := 0; j < metrics.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			for k := 0; k < metrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
				metric := metrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
				names[metric.Name()] = struct{}{}
			}
		}
	}

	uniqueNames := make([]string, 0, len(names))
	for name := range names {
		uniqueNames = append(uniqueNames, name)
	}
	return uniqueNames
}
