// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestRetainNumberMetricDatapointsForPodMixedBatch(t *testing.T) {
	t.Parallel()

	metrics := pmetric.NewMetrics()
	resource := metrics.ResourceMetrics().AppendEmpty()
	scope := resource.ScopeMetrics().AppendEmpty()

	gauge := scope.Metrics().AppendEmpty()
	gauge.SetName("container.memory.usage")
	gauge.SetEmptyGauge()
	for _, pod := range []string{"agent-abc", "nodejs-test-abc"} {
		datapoint := gauge.Gauge().DataPoints().AppendEmpty()
		datapoint.Attributes().PutStr("k8s.pod.name", pod)
		datapoint.SetIntValue(42)
	}
	sum := scope.Metrics().AppendEmpty()
	sum.SetName("k8s.pod.network.io")
	sum.SetEmptySum()
	for _, pod := range []string{"prometheus-test-abc", "agent-abc"} {
		datapoint := sum.Sum().DataPoints().AppendEmpty()
		datapoint.Attributes().PutStr("k8s.pod.name", pod)
		datapoint.SetIntValue(99)
	}
	nodejs := scope.Metrics().AppendEmpty()
	nodejs.SetName("process.runtime.nodejs.memory.heap.used")
	nodejs.SetEmptyGauge().DataPoints().AppendEmpty().Attributes().PutStr("k8s.pod.name", "nodejs-test-abc")
	prometheus := scope.Metrics().AppendEmpty()
	prometheus.SetName("test_requests_total")
	prometheus.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutStr("k8s.pod.name", "prometheus-test-abc")

	// Empty groups must not affect snapshot resource and metric counts either.
	resource.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
	metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptySum().DataPoints().AppendEmpty()

	RetainNumberMetricDatapointsForPod(&metrics, "agent-")

	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	require.Equal(t, 1, resource.ScopeMetrics().Len())
	require.Equal(t, 2, scope.Metrics().Len())
	assert.Equal(t, "container.memory.usage", scope.Metrics().At(0).Name())
	assert.Equal(t, "k8s.pod.network.io", scope.Metrics().At(1).Name())
	require.Equal(t, 1, gauge.Gauge().DataPoints().Len())
	require.Equal(t, 1, sum.Sum().DataPoints().Len())
	for _, datapoint := range []pmetric.NumberDataPoint{gauge.Gauge().DataPoints().At(0), sum.Sum().DataPoints().At(0)} {
		pod, ok := datapoint.Attributes().Get("k8s.pod.name")
		require.True(t, ok)
		assert.Equal(t, "agent-abc", pod.Str())
	}
	assert.Equal(t, int64(42), gauge.Gauge().DataPoints().At(0).IntValue())
	assert.Equal(t, int64(99), sum.Sum().DataPoints().At(0).IntValue())
}

func TestRetainNumberMetricDatapointsForPodNoMatch(t *testing.T) {
	t.Parallel()

	metrics := pmetric.NewMetrics()
	scope := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	for _, metricType := range []pmetric.MetricType{pmetric.MetricTypeGauge, pmetric.MetricTypeSum} {
		metric := scope.Metrics().AppendEmpty()
		var datapoints pmetric.NumberDataPointSlice
		if metricType == pmetric.MetricTypeGauge {
			datapoints = metric.SetEmptyGauge().DataPoints()
		} else {
			datapoints = metric.SetEmptySum().DataPoints()
		}
		datapoints.AppendEmpty() // Missing pod name.
		datapoints.AppendEmpty().Attributes().PutInt("k8s.pod.name", 42)
		datapoints.AppendEmpty().Attributes().PutStr("k8s.pod.name", "other-agent-abc")
	}
	metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	metrics.ResourceMetrics().AppendEmpty()

	RetainNumberMetricDatapointsForPod(&metrics, "agent-")

	assert.Zero(t, metrics.ResourceMetrics().Len())
}

func TestRetainNumberMetricDatapointsForPodPreservesOtherMetricTypes(t *testing.T) {
	t.Parallel()

	metrics := pmetric.NewMetrics()
	scope := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	scope.Metrics().AppendEmpty().SetEmptyHistogram().DataPoints().AppendEmpty()
	scope.Metrics().AppendEmpty().SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
	scope.Metrics().AppendEmpty().SetEmptySummary().DataPoints().AppendEmpty()
	scope.Metrics().AppendEmpty()
	want := pmetric.NewMetrics()
	metrics.CopyTo(want)

	RetainNumberMetricDatapointsForPod(&metrics, "agent-")

	assert.Equal(t, want, metrics)
}
