// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// requireEnv fetches an environment variable and fails the test if it is unset.
func requireEnv(t *testing.T, key string) string {
	value, set := os.LookupEnv(key)
	require.True(t, set, "the environment variable %s must be set", key)
	return value
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
