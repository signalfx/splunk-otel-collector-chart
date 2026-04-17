// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// requireEnv fetches an environment variable and fails the test if it is unset.
func requireEnv(t *testing.T, key string) string {
	value, set := os.LookupEnv(key)
	require.True(t, set, "the environment variable %s must be set", key)
	return value
}

// metricMatchFn decides whether a given metric (with its resource attributes)
// should be counted. Return true to accept the metric.
type metricMatchFn func(resAttrs pcommon.Map, metric pmetric.Metric) bool

// checkMetricsAreEmitted waits until all named metrics appear in the sink.
func checkMetricsAreEmitted(t *testing.T, mc *consumertest.MetricsSink, metricNames []string) {
	checkMetrics(t, mc, metricNames, "", nil)
}

// checkMetrics waits until all metricNames are found in the sink. An optional
// metricMatchFn can narrow results (e.g. by resource attrs or data-point attrs).
// label is used only for log messages; pass "" when not needed.
func checkMetrics(t *testing.T, mc *consumertest.MetricsSink, metricNames []string, label string, match metricMatchFn) {
	metricsToFind := map[string]bool{}
	for _, name := range metricNames {
		metricsToFind[name] = false
	}
	require.Eventuallyf(t, func() bool {
		for _, m := range mc.AllMetrics() {
			for i := 0; i < m.ResourceMetrics().Len(); i++ {
				rm := m.ResourceMetrics().At(i)
				resAttrs := rm.Resource().Attributes()
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						metric := sm.Metrics().At(k)
						if match == nil || match(resAttrs, metric) {
							metricsToFind[metric.Name()] = true
						}
					}
				}
			}
		}
		var stillMissing, found []string
		for _, name := range metricNames {
			if metricsToFind[name] {
				found = append(found, name)
			} else {
				stillMissing = append(stillMissing, name)
			}
		}
		if label != "" {
			t.Logf("[%s] found=%d missing=%d (%s)", label, len(found), len(stillMissing), strings.Join(stillMissing, ", "))
		} else {
			t.Logf("found=%d missing=%d (%s)", len(found), len(stillMissing), strings.Join(stillMissing, ", "))
		}
		return len(stillMissing) == 0
	}, 3*time.Minute, 10*time.Second,
		"failed to receive all metrics %s in 3 minutes", label)
}

// hasAttrMatch returns true when the attribute map contains key with the given string value.
func hasAttrMatch(attrs pcommon.Map, key, expected string) bool {
	v, ok := attrs.Get(key)
	return ok && v.Str() == expected
}

// anyDataPointMatches returns true if predicate matches the attributes of any
// data point inside metric, regardless of the metric type.
func anyDataPointMatches(metric pmetric.Metric, predicate func(pcommon.Map) bool) bool {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < metric.Gauge().DataPoints().Len(); i++ {
			if predicate(metric.Gauge().DataPoints().At(i).Attributes()) {
				return true
			}
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
			if predicate(metric.Sum().DataPoints().At(i).Attributes()) {
				return true
			}
		}
	case pmetric.MetricTypeHistogram:
		for i := 0; i < metric.Histogram().DataPoints().Len(); i++ {
			if predicate(metric.Histogram().DataPoints().At(i).Attributes()) {
				return true
			}
		}
	case pmetric.MetricTypeSummary:
		for i := 0; i < metric.Summary().DataPoints().Len(); i++ {
			if predicate(metric.Summary().DataPoints().At(i).Attributes()) {
				return true
			}
		}
	case pmetric.MetricTypeExponentialHistogram:
		for i := 0; i < metric.ExponentialHistogram().DataPoints().Len(); i++ {
			if predicate(metric.ExponentialHistogram().DataPoints().At(i).Attributes()) {
				return true
			}
		}
	}
	return false
}

// metricDataPointsHaveKey checks whether any data point in a metric has the given attribute key.
func metricDataPointsHaveKey(metric pmetric.Metric, key string) bool {
	return anyDataPointMatches(metric, func(attrs pcommon.Map) bool {
		_, ok := attrs.Get(key)
		return ok
	})
}

// metricDataPointsHaveAttrs checks whether any data point in a metric carries
// all of the given key/value pairs. Pairs are passed as alternating key, value strings.
func metricDataPointsHaveAttrs(metric pmetric.Metric, kvPairs ...string) bool {
	return anyDataPointMatches(metric, func(attrs pcommon.Map) bool {
		for i := 0; i < len(kvPairs)-1; i += 2 {
			if !hasAttrMatch(attrs, kvPairs[i], kvPairs[i+1]) {
				return false
			}
		}
		return true
	})
}
