// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"gopkg.in/yaml.v3"
)

type metricsAssertionConfig struct {
	volatileAttrs      []string
	regexAttrs         map[string]string
	firstDatapointOnly []string
}

// MetricsAssertionOption configures preprocessing before pmetricassert comparison.
type MetricsAssertionOption func(*metricsAssertionConfig)

const (
	// ContainerIDRegex matches container IDs with or without common runtime prefixes.
	ContainerIDRegex       = `(containerd://|cri-o://|docker://)?[0-9a-f]{64}`
	ContainerImageRegex    = `[-./:0-9a-z_]+`
	ContainerImageTagRegex = `[-.0-9A-Za-z_]+`
	// K8sNameRegex matches Kubernetes DNS label and DNS subdomain names.
	K8sNameRegex        = `[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*`
	K8sUIDRegex         = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`
	KubeletVersionRegex = `v[0-9]+\.[0-9]+\.[0-9]+([-+][-.0-9A-Za-z]+)?`
)

// CommonK8sMetricAssertionExistsAttrs holds shared attrs asserted as present-only.
var CommonK8sMetricAssertionExistsAttrs []string

// CommonK8sMetricAssertionRegexAttrs holds shared Kubernetes attrs with stable value shapes.
var CommonK8sMetricAssertionRegexAttrs = map[string]string{
	"container.id":         ContainerIDRegex,
	"container.image.name": ContainerImageRegex,
	"container.image.tag":  ContainerImageTagRegex,
	"k8s.daemonset.uid":    K8sUIDRegex,
	"k8s.deployment.uid":   K8sUIDRegex,
	"k8s.kubelet.version":  KubeletVersionRegex,
	"k8s.namespace.uid":    K8sUIDRegex,
	"k8s.node.name":        K8sNameRegex,
	"k8s.node.uid":         K8sUIDRegex,
	"k8s.pod.name":         K8sNameRegex,
	"k8s.pod.uid":          K8sUIDRegex,
	"k8s.replicaset.name":  K8sNameRegex,
	"k8s.replicaset.uid":   K8sUIDRegex,
}

// ExtendMetricAssertionAttrs copies a shared attr list before adding test-specific attrs.
func ExtendMetricAssertionAttrs(base []string, attrs ...string) []string {
	out := make([]string, 0, len(base)+len(attrs))
	out = append(out, base...)
	return append(out, attrs...)
}

// ExtendMetricAssertionRegexAttrs copies shared regex attrs before adding test-specific attrs.
func ExtendMetricAssertionRegexAttrs(base map[string]string, attrs map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(attrs))
	for attr, pattern := range base {
		out[attr] = pattern
	}
	for attr, pattern := range attrs {
		out[attr] = pattern
	}
	return out
}

// WithVolatileAttributes writes selected attributes as pmetricassert `/exists` matchers.
func WithVolatileAttributes(attrs ...string) MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.volatileAttrs = append(cfg.volatileAttrs, attrs...)
	}
}

// WithRegexAttributes writes selected attributes as pmetricassert `/regex` matchers.
func WithRegexAttributes(attrs map[string]string) MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		if cfg.regexAttrs == nil {
			cfg.regexAttrs = map[string]string{}
		}
		for attr, pattern := range attrs {
			cfg.regexAttrs[attr] = pattern
		}
	}
}

// WithFirstDatapointOnly preserves old pmetrictest.IgnoreSubsequentDataPoints behavior.
func WithFirstDatapointOnly(metricNames ...string) MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.firstDatapointOnly = append(cfg.firstDatapointOnly, metricNames...)
	}
}

// AssertMetricsSnapshot waits for a live batch that matches the snapshot shape.
func AssertMetricsSnapshot(t *testing.T, sink *consumertest.MetricsSink, targetMetric, assertionFile string, timeout, interval time.Duration, opts ...MetricsAssertionOption) {
	t.Helper()

	cfg := newMetricsAssertionConfig(opts...)
	wantResources, wantMetrics, err := assertionExpectedCounts(assertionFile)
	require.NoError(t, err, "Failed to read expected counts from %s", assertionFile)

	selected, exactMatch := selectMetricSetByCountsWithTimeout(t, targetMetric, sink, wantResources, wantMetrics, timeout, interval)
	require.NotNil(t, selected, "No metrics batch found containing target metric: %s", targetMetric)

	actual := prepareMetricsAssertion(*selected, cfg)
	assertErr := pmetricassert.AssertMetrics(assertionFile, actual)
	if assertErr != nil {
		if !exactMatch {
			t.Logf("No exact-count match (want %d resources, %d metrics); selected payload has %d metrics",
				wantResources, wantMetrics, selected.MetricCount())
		}
		maybeUpdateExpectedMetricsAssertion(t, assertionFile, actual, opts...)
		require.NoError(t, assertErr, "Metric assertion failed for %s. Error: %v", assertionFile, assertErr)
	}

	t.Logf("Metric assertion passed for %d metrics (%s)", selected.MetricCount(), assertionFile)
}

func selectMetricSetByCountsWithTimeout(t *testing.T, targetMetric string, metricSink *consumertest.MetricsSink, wantResources, wantMetrics int, timeout, interval time.Duration) (*pmetric.Metrics, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m := selectMetricSetByCounts(targetMetric, metricSink, wantResources, wantMetrics); m != nil {
			t.Logf("Selected exact-count payload with target metric '%s': %d metrics, %d resources",
				targetMetric, m.MetricCount(), m.ResourceMetrics().Len())
			return m, true
		}
		time.Sleep(interval)
	}

	best := richestMetricSet(targetMetric, metricSink)
	require.NotNilf(t, best, "No payload containing metric %s found within %v", targetMetric, timeout)
	t.Logf("No exact-count payload (%d resources, %d metrics) for '%s' within %v; using best-effort fallback: %d metrics, %d resources",
		wantResources, wantMetrics, targetMetric, timeout, best.MetricCount(), best.ResourceMetrics().Len())
	return best, false
}

func selectMetricSetByCounts(targetMetric string, metricSink *consumertest.MetricsSink, wantResources, wantMetrics int) *pmetric.Metrics {
	metrics := metricSink.AllMetrics()
	for h := len(metrics) - 1; h >= 0; h-- {
		m := metrics[h]
		if !containsMetric(m, targetMetric) {
			continue
		}
		if m.ResourceMetrics().Len() == wantResources && m.MetricCount() == wantMetrics {
			return &metrics[h]
		}
	}
	return nil
}

func richestMetricSet(targetMetric string, metricSink *consumertest.MetricsSink) *pmetric.Metrics {
	metrics := metricSink.AllMetrics()
	bestIndex := -1
	bestCount := -1
	for h := len(metrics) - 1; h >= 0; h-- {
		m := metrics[h]
		if !containsMetric(m, targetMetric) {
			continue
		}
		if m.MetricCount() > bestCount {
			bestIndex = h
			bestCount = m.MetricCount()
		}
	}
	if bestIndex < 0 {
		return nil
	}
	return &metrics[bestIndex]
}

func assertionExpectedCounts(file string) (int, int, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return 0, 0, fmt.Errorf("read assertion file %s: %w", file, err)
	}
	var doc map[string]any
	if unmarshalErr := yaml.Unmarshal(b, &doc); unmarshalErr != nil {
		return 0, 0, fmt.Errorf("parse assertion file %s: %w", file, unmarshalErr)
	}
	res, err := assertionSlice(file, "resources", doc["resources"])
	if err != nil {
		return 0, 0, err
	}
	resources := len(res)
	metrics := 0
	for i, r := range res {
		rm, ok := r.(map[string]any)
		if !ok {
			return 0, 0, fmt.Errorf("parse assertion file %s: resources[%d] must be a map", file, i)
		}
		scopes, scopeErr := assertionSlice(file, fmt.Sprintf("resources[%d].scopes", i), rm["scopes"])
		if scopeErr != nil {
			return 0, 0, scopeErr
		}
		for j, s := range scopes {
			sm, scopeOK := s.(map[string]any)
			if !scopeOK {
				return 0, 0, fmt.Errorf("parse assertion file %s: resources[%d].scopes[%d] must be a map", file, i, j)
			}
			ms, metricErr := assertionSlice(file, fmt.Sprintf("resources[%d].scopes[%d].metrics", i, j), sm["metrics"])
			if metricErr != nil {
				return 0, 0, metricErr
			}
			metrics += len(ms)
		}
	}
	if resources == 0 || metrics == 0 {
		return 0, 0, fmt.Errorf("parse assertion file %s: expected at least one resource and metric", file)
	}
	return resources, metrics, nil
}

func assertionSlice(file, path string, v any) ([]any, error) {
	s, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("parse assertion file %s: %s must be a list", file, path)
	}
	return s, nil
}

func maybeUpdateExpectedMetricsAssertion(t *testing.T, file string, actual pmetric.Metrics, opts ...MetricsAssertionOption) {
	if !shouldUpdateExpectedResults() {
		return
	}
	require.NoError(t, WriteMetricsAssertion(t, file, actual, opts...))
	t.Logf("Wrote updated expected metric assertion to %s", file)
}

// WriteMetricsAssertion applies assertion preprocessing before writing.
func WriteMetricsAssertion(tb testing.TB, file string, actual pmetric.Metrics, opts ...MetricsAssertionOption) error {
	tb.Helper()
	cfg := newMetricsAssertionConfig(opts...)
	prepared := prepareMetricsAssertion(actual, cfg)
	if err := pmetricassert.WriteAssertionFile(tb, file, prepared); err != nil {
		return fmt.Errorf("write assertion file %s: %w", file, err)
	}
	return markFlexibleAttrs(file, cfg.volatileAttrs, cfg.regexAttrs)
}

// markFlexibleAttrs applies `/exists` and `/regex` after pmetricassert writes exact values.
func markFlexibleAttrs(file string, volatile []string, regex map[string]string) error {
	if len(volatile) == 0 && len(regex) == 0 {
		return nil
	}
	vol := make(map[string]struct{}, len(volatile))
	for _, k := range volatile {
		vol[k] = struct{}{}
	}

	b, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read assertion file %s: %w", file, err)
	}
	var doc map[string]any
	if unmarshalErr := yaml.Unmarshal(b, &doc); unmarshalErr != nil {
		return fmt.Errorf("parse assertion file %s: %w", file, unmarshalErr)
	}

	resources, err := assertionSlice(file, "resources", doc["resources"])
	if err != nil {
		return err
	}
	for i, res := range resources {
		resMap, ok := res.(map[string]any)
		if !ok {
			return fmt.Errorf("parse assertion file %s: resources[%d] must be a map", file, i)
		}
		if markErr := markAttrs(file, fmt.Sprintf("resources[%d].attributes", i), resMap["attributes"], vol, regex); markErr != nil {
			return markErr
		}
		scopes, scopeErr := assertionSlice(file, fmt.Sprintf("resources[%d].scopes", i), resMap["scopes"])
		if scopeErr != nil {
			return scopeErr
		}
		for j, scope := range scopes {
			scopeMap, scopeOK := scope.(map[string]any)
			if !scopeOK {
				return fmt.Errorf("parse assertion file %s: resources[%d].scopes[%d] must be a map", file, i, j)
			}
			metrics, metricErr := assertionSlice(file, fmt.Sprintf("resources[%d].scopes[%d].metrics", i, j), scopeMap["metrics"])
			if metricErr != nil {
				return metricErr
			}
			for k, metric := range metrics {
				metricMap, metricOK := metric.(map[string]any)
				if !metricOK {
					return fmt.Errorf("parse assertion file %s: resources[%d].scopes[%d].metrics[%d] must be a map", file, i, j, k)
				}
				var datapoints []any
				if rawDatapoints := metricMap["datapoints"]; rawDatapoints != nil {
					var datapointErr error
					datapoints, datapointErr = assertionSlice(file, fmt.Sprintf("resources[%d].scopes[%d].metrics[%d].datapoints", i, j, k), rawDatapoints)
					if datapointErr != nil {
						return datapointErr
					}
				}
				for l, dp := range datapoints {
					dpMap, datapointOK := dp.(map[string]any)
					if !datapointOK {
						return fmt.Errorf("parse assertion file %s: resources[%d].scopes[%d].metrics[%d].datapoints[%d] must be a map", file, i, j, k, l)
					}
					if markErr := markAttrs(file, fmt.Sprintf("resources[%d].scopes[%d].metrics[%d].datapoints[%d].attributes", i, j, k, l), dpMap["attributes"], vol, regex); markErr != nil {
						return markErr
					}
				}
			}
		}
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal assertion file %s: %w", file, err)
	}
	//nolint:gosec // Assertion snapshots are committed testdata.
	if writeErr := os.WriteFile(file, out, 0o644); writeErr != nil {
		return fmt.Errorf("write assertion file %s: %w", file, writeErr)
	}
	return nil
}

func markAttrs(file, path string, attrs any, vol map[string]struct{}, regex map[string]string) error {
	if attrs == nil {
		return nil
	}
	m, ok := attrs.(map[string]any)
	if !ok {
		return fmt.Errorf("parse assertion file %s: %s must be a map", file, path)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.HasSuffix(k, "/exists") || strings.HasSuffix(k, "/regex") {
			continue
		}
		if pattern, regexAttr := regex[k]; regexAttr {
			delete(m, k)
			m[k+"/regex"] = pattern
			continue
		}
		if _, volatile := vol[k]; volatile {
			delete(m, k)
			m[k+"/exists"] = true
		}
	}
	return nil
}

func newMetricsAssertionConfig(opts ...MetricsAssertionOption) metricsAssertionConfig {
	var cfg metricsAssertionConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func prepareMetricsAssertion(actual pmetric.Metrics, cfg metricsAssertionConfig) pmetric.Metrics {
	prepared := pmetric.NewMetrics()
	actual.CopyTo(prepared)
	keepFirstDatapointOnly(prepared, cfg.firstDatapointOnly, cfg.flexibleAttrs())
	return prepared
}

func (cfg metricsAssertionConfig) flexibleAttrs() []string {
	attrs := append([]string{}, cfg.volatileAttrs...)
	for attr := range cfg.regexAttrs {
		attrs = append(attrs, attr)
	}
	return attrs
}

// keepFirstDatapointOnly preserves old comparison semantics for noisy multi-series metrics.
func keepFirstDatapointOnly(metrics pmetric.Metrics, metricNames []string, volatileAttrs []string) {
	if len(metricNames) == 0 {
		return
	}
	names := make(map[string]struct{}, len(metricNames))
	for _, name := range metricNames {
		names[name] = struct{}{}
	}
	volatile := make(map[string]struct{}, len(volatileAttrs))
	for _, attr := range volatileAttrs {
		volatile[attr] = struct{}{}
	}
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rms := metrics.ResourceMetrics().At(i)
		for j := 0; j < rms.ScopeMetrics().Len(); j++ {
			ms := rms.ScopeMetrics().At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				metric := ms.At(k)
				if _, ok := names[metric.Name()]; !ok {
					continue
				}
				keepFirstDatapoint(metric, volatile)
			}
		}
	}
}

func keepFirstDatapoint(metric pmetric.Metric, volatile map[string]struct{}) {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		dps.Sort(func(a, b pmetric.NumberDataPoint) bool {
			return attrsLess(a.Attributes(), b.Attributes(), volatile)
		})
		n := 0
		dps.RemoveIf(func(pmetric.NumberDataPoint) bool {
			n++
			return n > 1
		})
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		dps.Sort(func(a, b pmetric.NumberDataPoint) bool {
			return attrsLess(a.Attributes(), b.Attributes(), volatile)
		})
		n := 0
		dps.RemoveIf(func(pmetric.NumberDataPoint) bool {
			n++
			return n > 1
		})
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		dps.Sort(func(a, b pmetric.HistogramDataPoint) bool {
			return attrsLess(a.Attributes(), b.Attributes(), volatile)
		})
		n := 0
		dps.RemoveIf(func(pmetric.HistogramDataPoint) bool {
			n++
			return n > 1
		})
	case pmetric.MetricTypeExponentialHistogram:
		dps := metric.ExponentialHistogram().DataPoints()
		dps.Sort(func(a, b pmetric.ExponentialHistogramDataPoint) bool {
			return attrsLess(a.Attributes(), b.Attributes(), volatile)
		})
		n := 0
		dps.RemoveIf(func(pmetric.ExponentialHistogramDataPoint) bool {
			n++
			return n > 1
		})
	case pmetric.MetricTypeSummary:
		dps := metric.Summary().DataPoints()
		dps.Sort(func(a, b pmetric.SummaryDataPoint) bool {
			return attrsLess(a.Attributes(), b.Attributes(), volatile)
		})
		n := 0
		dps.RemoveIf(func(pmetric.SummaryDataPoint) bool {
			n++
			return n > 1
		})
	}
}

func attrsLess(a, b pcommon.Map, volatile map[string]struct{}) bool {
	aHash := pdatautil.MapHash(sortableAttrs(a, volatile))
	bHash := pdatautil.MapHash(sortableAttrs(b, volatile))
	return bytes.Compare(aHash[:], bHash[:]) < 0
}

// sortableAttrs masks flexible attrs before hashing so first-datapoint choice is stable.
func sortableAttrs(attrs pcommon.Map, volatile map[string]struct{}) pcommon.Map {
	if len(volatile) == 0 {
		return attrs
	}
	out := pcommon.NewMap()
	attrs.CopyTo(out)
	for attr := range volatile {
		value, ok := out.Get(attr)
		if !ok {
			continue
		}
		zeroAttributeValue(value)
	}
	return out
}

func zeroAttributeValue(value pcommon.Value) {
	switch value.Type() {
	case pcommon.ValueTypeStr:
		value.SetStr("")
	case pcommon.ValueTypeBool:
		value.SetBool(false)
	case pcommon.ValueTypeInt:
		value.SetInt(0)
	case pcommon.ValueTypeDouble:
		value.SetDouble(0)
	}
}
