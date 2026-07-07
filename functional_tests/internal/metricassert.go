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
	volatileAttrs                  []string
	regexAttrs                     map[string]string
	datapointAttrs                 map[string]struct{}
	firstDatapointOnly             []string
	firstDatapointOnlyAll          bool
	ignoreScopeVersion             bool
	expectedMetricsOnly            bool
	includeHistogramExplicitBounds bool
}

// MetricsAssertionOption configures preprocessing before pmetricassert comparison.
type MetricsAssertionOption func(*metricsAssertionConfig)

const (
	// ContainerIDRegex matches container IDs with or without common runtime prefixes.
	ContainerIDRegex       = `(containerd://|cri-o://|docker://)?[0-9a-f]{64}`
	ContainerImageRegex    = `[-./:0-9a-z_]+`
	ContainerImageTagRegex = `[-.0-9A-Za-z_]+`
	// K8sNameRegex matches Kubernetes DNS label and DNS subdomain names.
	K8sNameRegex = `[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*`
	K8sUIDRegex  = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`
	// K8sVersionRegex matches Kubernetes semantic versions with optional suffixes.
	K8sVersionRegex = `v[0-9]+\.[0-9]+\.[0-9]+([-+][-.0-9A-Za-z]+)?`
)

// CommonK8sMetricAssertionRegexAttrs holds shared Kubernetes attrs with stable value shapes.
var CommonK8sMetricAssertionRegexAttrs = map[string]string{
	"container.id":         ContainerIDRegex,
	"container.image.name": ContainerImageRegex,
	"container.image.tag":  ContainerImageTagRegex,
	"k8s.daemonset.uid":    K8sUIDRegex,
	"k8s.deployment.uid":   K8sUIDRegex,
	"k8s.kubelet.version":  K8sVersionRegex,
	"k8s.namespace.uid":    K8sUIDRegex,
	"k8s.node.name":        K8sNameRegex,
	"k8s.node.uid":         K8sUIDRegex,
	"k8s.pod.name":         K8sNameRegex,
	"k8s.pod.uid":          K8sUIDRegex,
	"k8s.replicaset.name":  K8sNameRegex,
	"k8s.replicaset.uid":   K8sUIDRegex,
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

// WithDatapointAttributes keeps only selected datapoint attributes before comparison.
func WithDatapointAttributes(attrs ...string) MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		if cfg.datapointAttrs == nil {
			cfg.datapointAttrs = map[string]struct{}{}
		}
		for _, attr := range attrs {
			cfg.datapointAttrs[attr] = struct{}{}
		}
	}
}

// WithFirstDatapointOnly preserves old pmetrictest.IgnoreSubsequentDataPoints behavior.
func WithFirstDatapointOnly(metricNames ...string) MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.firstDatapointOnly = append(cfg.firstDatapointOnly, metricNames...)
	}
}

// WithFirstDatapointOnlyForAllMetrics keeps one stable datapoint per metric.
func WithFirstDatapointOnlyForAllMetrics() MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.firstDatapointOnlyAll = true
	}
}

// WithIgnoredScopeVersion clears instrumentation scope versions before comparison.
func WithIgnoredScopeVersion() MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.ignoreScopeVersion = true
	}
}

// WithExpectedMetricsOnly ignores metrics not listed in the assertion snapshot.
func WithExpectedMetricsOnly() MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.expectedMetricsOnly = true
	}
}

// WithHistogramExplicitBounds keeps histogram bucket boundaries in generated snapshots.
func WithHistogramExplicitBounds() MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.includeHistogramExplicitBounds = true
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
	updated := maybeUpdateExpectedMetricsAssertion(t, assertionFile, actual, opts...)
	assertErr := pmetricassert.AssertMetrics(assertionFile, actual)
	if assertErr != nil {
		if !exactMatch {
			t.Logf("No exact-count match (want %d resources, %d metrics); selected payload has %d metrics",
				wantResources, wantMetrics, selected.MetricCount())
		}
		if !updated {
			maybeUpdateExpectedMetricsAssertion(t, assertionFile, actual, opts...)
		}
		require.NoError(t, assertErr, "Metric assertion failed for %s. Error: %v", assertionFile, assertErr)
	}

	t.Logf("Metric assertion passed for %d metrics (%s)", selected.MetricCount(), assertionFile)
}

// AssertMetricsDataSnapshot compares already-selected metrics with an assertion snapshot.
func AssertMetricsDataSnapshot(t *testing.T, assertionFile string, actual pmetric.Metrics, opts ...MetricsAssertionOption) {
	t.Helper()

	cfg := newMetricsAssertionConfig(opts...)
	prepared := prepareMetricsAssertion(actual, cfg)
	updated := maybeUpdateExpectedMetricsAssertion(t, assertionFile, prepared, opts...)
	compareMetrics := prepared
	if cfg.expectedMetricsOnly {
		metricNames, err := assertionMetricNames(assertionFile)
		if err == nil {
			compareMetrics = keepExpectedMetricsOnly(prepared, metricNames)
		} else if !os.IsNotExist(err) {
			require.NoError(t, err, "Failed to read expected metric names from %s", assertionFile)
		}
	}

	assertErr := pmetricassert.AssertMetrics(assertionFile, compareMetrics)
	if assertErr != nil {
		if !updated {
			maybeUpdateExpectedMetricsAssertion(t, assertionFile, prepared, opts...)
		}
		require.NoError(t, assertErr, "Metric assertion failed for %s. Error: %v", assertionFile, assertErr)
	}
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

func assertionMetricNames(file string) (map[string]struct{}, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if unmarshalErr := yaml.Unmarshal(b, &doc); unmarshalErr != nil {
		return nil, fmt.Errorf("parse assertion file %s: %w", file, unmarshalErr)
	}
	res, err := assertionSlice(file, "resources", doc["resources"])
	if err != nil {
		return nil, err
	}
	names := map[string]struct{}{}
	for i, r := range res {
		rm, ok := r.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("parse assertion file %s: resources[%d] must be a map", file, i)
		}
		scopes, scopeErr := assertionSlice(file, fmt.Sprintf("resources[%d].scopes", i), rm["scopes"])
		if scopeErr != nil {
			return nil, scopeErr
		}
		for j, s := range scopes {
			sm, scopeOK := s.(map[string]any)
			if !scopeOK {
				return nil, fmt.Errorf("parse assertion file %s: resources[%d].scopes[%d] must be a map", file, i, j)
			}
			metrics, metricErr := assertionSlice(file, fmt.Sprintf("resources[%d].scopes[%d].metrics", i, j), sm["metrics"])
			if metricErr != nil {
				return nil, metricErr
			}
			for k, metric := range metrics {
				metricMap, metricOK := metric.(map[string]any)
				if !metricOK {
					return nil, fmt.Errorf("parse assertion file %s: resources[%d].scopes[%d].metrics[%d] must be a map", file, i, j, k)
				}
				name, nameOK := metricMap["name"].(string)
				if !nameOK {
					return nil, fmt.Errorf("parse assertion file %s: resources[%d].scopes[%d].metrics[%d].name must be a string", file, i, j, k)
				}
				names[name] = struct{}{}
			}
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("parse assertion file %s: expected at least one metric", file)
	}
	return names, nil
}

func assertionSlice(file, path string, v any) ([]any, error) {
	s, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("parse assertion file %s: %s must be a list", file, path)
	}
	return s, nil
}

func maybeUpdateExpectedMetricsAssertion(t *testing.T, file string, actual pmetric.Metrics, opts ...MetricsAssertionOption) bool {
	if !shouldUpdateExpectedResults() {
		return false
	}
	require.NoError(t, WriteMetricsAssertion(t, file, actual, opts...))
	t.Logf("Wrote updated expected metric assertion to %s", file)
	return true
}

// WriteMetricsAssertion applies assertion preprocessing before writing.
func WriteMetricsAssertion(tb testing.TB, file string, actual pmetric.Metrics, opts ...MetricsAssertionOption) error {
	tb.Helper()
	cfg := newMetricsAssertionConfig(opts...)
	prepared := prepareMetricsAssertion(actual, cfg)
	writeOpts := []pmetricassert.WriteOption{}
	if cfg.includeHistogramExplicitBounds {
		writeOpts = append(writeOpts, pmetricassert.IncludeValues())
	}
	if err := pmetricassert.WriteAssertionFile(tb, file, prepared, writeOpts...); err != nil {
		return fmt.Errorf("write assertion file %s: %w", file, err)
	}
	return editAssertionFile(file, prepared, cfg)
}

// editAssertionFile applies local matcher options after pmetricassert writes exact values.
func editAssertionFile(file string, actual pmetric.Metrics, cfg metricsAssertionConfig) error {
	if len(cfg.volatileAttrs) == 0 && len(cfg.regexAttrs) == 0 && !cfg.includeHistogramExplicitBounds {
		return nil
	}
	vol := make(map[string]struct{}, len(cfg.volatileAttrs))
	for _, k := range cfg.volatileAttrs {
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
		if markErr := markAttrs(file, fmt.Sprintf("resources[%d].attributes", i), resMap["attributes"], vol, cfg.regexAttrs); markErr != nil {
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
				if cfg.includeHistogramExplicitBounds && metricMap["datapoints"] == nil {
					if bounds := histogramExplicitBounds(actual, metricMap["name"]); len(bounds) > 0 {
						metricMap["datapoints"] = []any{map[string]any{"explicit_bounds": bounds}}
					}
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
					stripDatapointValues(dpMap, cfg.includeHistogramExplicitBounds)
					if markErr := markAttrs(file, fmt.Sprintf("resources[%d].scopes[%d].metrics[%d].datapoints[%d].attributes", i, j, k, l), dpMap["attributes"], vol, cfg.regexAttrs); markErr != nil {
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

func histogramExplicitBounds(metrics pmetric.Metrics, rawName any) []float64 {
	name, ok := rawName.(string)
	if !ok {
		return nil
	}
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ms := rm.ScopeMetrics().At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				metric := ms.At(k)
				if metric.Name() != name || metric.Type() != pmetric.MetricTypeHistogram {
					continue
				}
				dps := metric.Histogram().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					bounds := dps.At(l).ExplicitBounds()
					if bounds.Len() == 0 {
						continue
					}
					out := make([]float64, bounds.Len())
					for m := 0; m < bounds.Len(); m++ {
						out[m] = bounds.At(m)
					}
					return out
				}
			}
		}
	}
	return nil
}

func stripDatapointValues(dp map[string]any, keepExplicitBounds bool) {
	delete(dp, "value")
	delete(dp, "count")
	delete(dp, "sum")
	delete(dp, "min")
	delete(dp, "max")
	delete(dp, "bucket_counts")
	if !keepExplicitBounds {
		delete(dp, "explicit_bounds")
	}
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
	if cfg.ignoreScopeVersion {
		clearScopeVersions(prepared)
	}
	keepDatapointAttributes(prepared, cfg.datapointAttrs)
	keepFirstDatapointOnly(prepared, cfg.firstDatapointOnly, cfg.firstDatapointOnlyAll, cfg.flexibleAttrs())
	return prepared
}

func clearScopeVersions(metrics pmetric.Metrics) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			rm.ScopeMetrics().At(j).Scope().SetVersion("")
		}
	}
}

func keepDatapointAttributes(metrics pmetric.Metrics, keep map[string]struct{}) {
	if keep == nil {
		return
	}
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ms := rm.ScopeMetrics().At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				keepMetricDatapointAttributes(ms.At(k), keep)
			}
		}
	}
}

func keepMetricDatapointAttributes(metric pmetric.Metric, keep map[string]struct{}) {
	remove := func(attrs pcommon.Map) {
		attrs.RemoveIf(func(k string, _ pcommon.Value) bool {
			_, ok := keep[k]
			return !ok
		})
	}
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			remove(dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			remove(dps.At(i).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			remove(dps.At(i).Attributes())
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := metric.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			remove(dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSummary:
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			remove(dps.At(i).Attributes())
		}
	}
}

func keepExpectedMetricsOnly(metrics pmetric.Metrics, names map[string]struct{}) pmetric.Metrics {
	filtered := pmetric.NewMetrics()
	metrics.CopyTo(filtered)
	filtered.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rm.ScopeMetrics().RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			sm.Metrics().RemoveIf(func(metric pmetric.Metric) bool {
				_, ok := names[metric.Name()]
				return !ok
			})
			return sm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})
	return filtered
}

func (cfg metricsAssertionConfig) flexibleAttrs() []string {
	attrs := append([]string{}, cfg.volatileAttrs...)
	for attr := range cfg.regexAttrs {
		attrs = append(attrs, attr)
	}
	return attrs
}

// keepFirstDatapointOnly preserves old comparison semantics for noisy multi-series metrics.
func keepFirstDatapointOnly(metrics pmetric.Metrics, metricNames []string, allMetrics bool, volatileAttrs []string) {
	if len(metricNames) == 0 && !allMetrics {
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
				if _, ok := names[metric.Name()]; !ok && !allMetrics {
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
