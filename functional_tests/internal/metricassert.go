// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"gopkg.in/yaml.v3"
)

type metricsAssertionConfig struct {
	volatileAttrs                  []string
	regexAttrs                     map[string]string
	scopeVersionRegex              string
	exactDatapointAttrs            map[string]struct{}
	includeHistogramExplicitBounds bool
	waitForSnapshotMatch           bool
	metricsFilter                  func(*pmetric.Metrics)
}

// MetricsAssertionOption configures snapshot selection and generation.
type MetricsAssertionOption func(*metricsAssertionConfig)

const (
	// ContainerIDRegex matches container IDs with or without common runtime prefixes.
	ContainerIDRegex       = `(containerd://|cri-o://|docker://)?[0-9a-f]{64}`
	ContainerImageRegex    = `[-./:0-9a-z_]+`
	ContainerImageTagRegex = `[-.0-9A-Za-z_]+`
	// K8sNameRegex matches Kubernetes DNS label and DNS subdomain names.
	K8sNameRegex = `[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*`
	K8sUIDRegex  = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`
	// K8sAPIVerbRegex matches canonical request verbs reported in API server metrics.
	K8sAPIVerbRegex           = `^(APPLY|CONNECT|CREATE|DELETE|DELETECOLLECTION|GET|LIST|PATCH|POST|PROXY|PUT|UPDATE|WATCH|WATCHLIST|other)$`
	KubeletVersionRegex       = `v[0-9]+\.[0-9]+\.[0-9]+([-+][-.0-9A-Za-z]+)?`
	OtelCollectorVersionRegex = `v[0-9]+\.[0-9]+\.[0-9]+([-+][-.0-9A-Za-z]+)?`
)

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

// WithScopeVersionRegex writes scope versions as pmetricassert `version/regex` matchers.
func WithScopeVersionRegex(pattern string) MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.scopeVersionRegex = pattern
	}
}

// WithWaitForSnapshotMatch retries complete live batches until one matches.
func WithWaitForSnapshotMatch() MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.waitForSnapshotMatch = true
	}
}

// WithMetricsFilter applies a test-specific filter to a copy of each live batch.
func WithMetricsFilter(filter func(*pmetric.Metrics)) MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.metricsFilter = filter
	}
}

// WithHistogramExplicitBounds includes histogram bucket boundaries in the snapshot.
func WithHistogramExplicitBounds() MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.includeHistogramExplicitBounds = true
	}
}

// WithDatapointAttributesAsExistsExcept writes all datapoint attributes except
// the named identity attributes as pmetricassert `/exists` matchers.
func WithDatapointAttributesAsExistsExcept(exactAttrs ...string) MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.exactDatapointAttrs = make(map[string]struct{}, len(exactAttrs))
		for _, attr := range exactAttrs {
			cfg.exactDatapointAttrs[attr] = struct{}{}
		}
	}
}

// MetricNameFilter optionally limits which observed metrics contribute names.
type MetricNameFilter func(pcommon.Map, pmetric.Metric) bool

// AssertMetricNames checks names observed across every batch in the sink.
// Projecting to one canonical resource and scope preserves name-only checks:
// resource, scope, metric type, datapoints, and values remain unconstrained.
// An optional filter can restrict which observed metrics contribute names.
func AssertMetricNames(t *testing.T, sink *consumertest.MetricsSink, assertionFile string, timeout, interval time.Duration, filters ...MetricNameFilter) {
	t.Helper()
	require.LessOrEqual(t, len(filters), 1)
	var filter MetricNameFilter
	if len(filters) == 1 {
		filter = filters[0]
	}
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		names := make(map[string]struct{})
		for _, batch := range sink.AllMetrics() {
			for i := 0; i < batch.ResourceMetrics().Len(); i++ {
				rm := batch.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					metrics := rm.ScopeMetrics().At(j).Metrics()
					for k := 0; k < metrics.Len(); k++ {
						metric := metrics.At(k)
						if filter == nil || filter(rm.Resource().Attributes(), metric) {
							names[metric.Name()] = struct{}{}
						}
					}
				}
			}
		}

		projected := pmetric.NewMetrics()
		metrics := projected.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
		for name := range names {
			metric := metrics.AppendEmpty()
			metric.SetName(name)
			metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(0)
		}
		assert.NoError(tt, pmetricassert.AssertMetrics(assertionFile, projected))
	}, timeout, interval, "Metric name assertion failed for %s", assertionFile)
}

// AssertMetricsSnapshot selects a complete live batch and checks its assertion.
func AssertMetricsSnapshot(t *testing.T, sink *consumertest.MetricsSink, targetMetric, assertionFile string, timeout, interval time.Duration, opts ...MetricsAssertionOption) {
	t.Helper()
	filter := newMetricsAssertionConfig(opts...).metricsFilter

	if shouldUpdateExpectedResults() {
		var selected *pmetric.Metrics
		if _, err := os.Stat(assertionFile); err == nil {
			wantResources, wantMetrics, countErr := assertionExpectedCounts(assertionFile)
			require.NoError(t, countErr, "Failed to read expected counts from %s", assertionFile)
			selected = selectMetricSetByCountsWithTimeout(targetMetric, sink, wantResources, wantMetrics, timeout, interval, filter)
		} else {
			require.True(t, os.IsNotExist(err), "Failed to inspect assertion file %s: %v", assertionFile, err)
			selected = waitForMetricSet(targetMetric, sink, timeout, interval, filter)
		}
		require.NotNil(t, selected, "No metrics batch found containing target metric: %s", targetMetric)
		require.NoError(t, WriteMetricsAssertion(t, assertionFile, *selected, opts...))
		t.Logf("Wrote updated expected metric assertion to %s", assertionFile)
		return
	}

	wantResources, wantMetrics, err := assertionExpectedCounts(assertionFile)
	require.NoError(t, err, "Failed to read expected counts from %s", assertionFile)
	if newMetricsAssertionConfig(opts...).waitForSnapshotMatch {
		selected, assertErr := selectMetricSetByAssertionWithTimeout(targetMetric, sink, wantResources, wantMetrics, assertionFile, timeout, interval, filter)
		require.NotNil(t, selected, "No metrics batch found containing target metric: %s", targetMetric)
		require.NoError(t, assertErr, "Metric assertion failed for %s", assertionFile)
		t.Logf("Metric assertion passed for %d metrics (%s)", selected.MetricCount(), assertionFile)
		return
	}

	selected := selectMetricSetByCountsWithTimeout(targetMetric, sink, wantResources, wantMetrics, timeout, interval, filter)
	require.NotNil(t, selected, "No metrics batch found containing target metric: %s", targetMetric)
	require.NoError(t, pmetricassert.AssertMetrics(assertionFile, *selected), "Metric assertion failed for %s", assertionFile)
	t.Logf("Metric assertion passed for %d metrics (%s)", selected.MetricCount(), assertionFile)
}

func selectMetricSetByAssertionWithTimeout(targetMetric string, sink *consumertest.MetricsSink, wantResources, wantMetrics int, assertionFile string, timeout, interval time.Duration, filter func(*pmetric.Metrics)) (*pmetric.Metrics, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if selected := selectMetricSetByCounts(targetMetric, sink, wantResources, wantMetrics, filter); selected != nil {
			if err := pmetricassert.AssertMetrics(assertionFile, *selected); err == nil {
				return selected, nil
			}
		}
		time.Sleep(interval)
	}

	selected := richestMetricSet(targetMetric, sink, filter)
	if selected == nil {
		return nil, nil
	}
	return selected, pmetricassert.AssertMetrics(assertionFile, *selected)
}

func selectMetricSetByCountsWithTimeout(targetMetric string, sink *consumertest.MetricsSink, wantResources, wantMetrics int, timeout, interval time.Duration, filter func(*pmetric.Metrics)) *pmetric.Metrics {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if selected := selectMetricSetByCounts(targetMetric, sink, wantResources, wantMetrics, filter); selected != nil {
			return selected
		}
		time.Sleep(interval)
	}
	return richestMetricSet(targetMetric, sink, filter)
}

func selectMetricSetByCounts(targetMetric string, sink *consumertest.MetricsSink, wantResources, wantMetrics int, filter func(*pmetric.Metrics)) *pmetric.Metrics {
	batches := sink.AllMetrics()
	for i := len(batches) - 1; i >= 0; i-- {
		if !containsMetric(batches[i], targetMetric) {
			continue
		}
		metrics := filteredMetricSet(batches[i], filter)
		if metrics.ResourceMetrics().Len() == wantResources && metrics.MetricCount() == wantMetrics {
			return &metrics
		}
	}
	return nil
}

func assertionExpectedCounts(file string) (int, int, error) {
	data, readErr := os.ReadFile(file)
	if readErr != nil {
		return 0, 0, fmt.Errorf("read assertion file %s: %w", file, readErr)
	}
	var doc struct {
		Resources []struct {
			Scopes []struct {
				Metrics []any `yaml:"metrics"`
			} `yaml:"scopes"`
		} `yaml:"resources"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return 0, 0, fmt.Errorf("parse assertion file %s: %w", file, err)
	}
	metricCount := 0
	for _, resource := range doc.Resources {
		for _, scope := range resource.Scopes {
			metricCount += len(scope.Metrics)
		}
	}
	if len(doc.Resources) == 0 || metricCount == 0 {
		return 0, 0, fmt.Errorf("assertion file %s has no resources or metrics", file)
	}
	return len(doc.Resources), metricCount, nil
}

func waitForMetricSet(targetMetric string, sink *consumertest.MetricsSink, timeout, interval time.Duration, filter func(*pmetric.Metrics)) *pmetric.Metrics {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if current := richestMetricSet(targetMetric, sink, filter); current != nil {
			return current
		}
		time.Sleep(interval)
	}
	return nil
}

func richestMetricSet(targetMetric string, metricSink *consumertest.MetricsSink, filter func(*pmetric.Metrics)) *pmetric.Metrics {
	metrics := metricSink.AllMetrics()
	var best *pmetric.Metrics
	bestCount := -1
	for h := len(metrics) - 1; h >= 0; h-- {
		if !containsMetric(metrics[h], targetMetric) {
			continue
		}
		m := filteredMetricSet(metrics[h], filter)
		if m.MetricCount() > bestCount {
			best = &m
			bestCount = m.MetricCount()
		}
	}
	return best
}

func filteredMetricSet(metrics pmetric.Metrics, filter func(*pmetric.Metrics)) pmetric.Metrics {
	if filter == nil {
		return metrics
	}
	filtered := pmetric.NewMetrics()
	metrics.CopyTo(filtered)
	filter(&filtered)
	return filtered
}

// WriteMetricsAssertion writes an assertion snapshot with flexible attribute matchers.
func WriteMetricsAssertion(tb testing.TB, file string, actual pmetric.Metrics, opts ...MetricsAssertionOption) error {
	tb.Helper()
	previous, readErr := os.ReadFile(file)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read existing assertion file %s: %w", file, readErr)
	}
	tmp, createErr := os.CreateTemp(tb.TempDir(), "metric-assertion-*.yaml")
	if createErr != nil {
		return fmt.Errorf("create temporary assertion file: %w", createErr)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary assertion file: %w", err)
	}

	cfg := newMetricsAssertionConfig(opts...)
	var writeOpts []pmetricassert.WriteOption
	if cfg.includeHistogramExplicitBounds {
		writeOpts = append(writeOpts, pmetricassert.IncludeHistogramExplicitBounds())
	}
	exists := make(map[string]struct{}, len(cfg.volatileAttrs))
	for _, key := range cfg.volatileAttrs {
		exists[key] = struct{}{}
	}
	if cfg.exactDatapointAttrs != nil {
		for key := range nonIdentityDatapointAttrs(actual, cfg.exactDatapointAttrs) {
			exists[key] = struct{}{}
		}
	}
	for key := range cfg.regexAttrs {
		delete(exists, key)
	}
	if len(exists) > 0 {
		keys := make([]string, 0, len(exists))
		for key := range exists {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		writeOpts = append(writeOpts, pmetricassert.WithAttributeExists(keys...))
	}
	if len(cfg.regexAttrs) > 0 {
		writeOpts = append(writeOpts, pmetricassert.WithAttributeRegex(cfg.regexAttrs))
	}
	if err := pmetricassert.WriteAssertionFile(tb, tmp.Name(), actual, writeOpts...); err != nil {
		return fmt.Errorf("write assertion file %s: %w", file, err)
	}
	if err := markScopeVersionRegex(tmp.Name(), cfg.scopeVersionRegex); err != nil {
		return err
	}
	generated, generatedReadErr := os.ReadFile(tmp.Name())
	if generatedReadErr != nil {
		return fmt.Errorf("read generated assertion file: %w", generatedReadErr)
	}
	if len(previous) > 0 {
		merged, mergeErr := preserveDatapointMatchers(previous, generated)
		if mergeErr != nil {
			return fmt.Errorf("preserve assertion matchers in %s: %w", file, mergeErr)
		}
		generated = merged
	}
	if err := os.WriteFile(tmp.Name(), generated, 0o600); err != nil {
		return fmt.Errorf("write generated assertion for validation: %w", err)
	}
	if err := pmetricassert.AssertMetrics(tmp.Name(), actual); err != nil {
		return fmt.Errorf("generated assertion does not match live metrics: %w", err)
	}
	//nolint:gosec // Assertion snapshots are committed testdata.
	if err := os.WriteFile(file, generated, 0o644); err != nil {
		return fmt.Errorf("write assertion file %s: %w", file, err)
	}
	return nil
}

// preserveDatapointMatchers keeps deliberate include and count constraints when
// refreshing an assertion. The upstream writer emits exact datapoint lists.
func preserveDatapointMatchers(previous, generated []byte) ([]byte, error) {
	var oldDoc, newDoc map[string]any
	if err := yaml.Unmarshal(previous, &oldDoc); err != nil {
		return nil, fmt.Errorf("parse existing assertion: %w", err)
	}
	if err := yaml.Unmarshal(generated, &newDoc); err != nil {
		return nil, fmt.Errorf("parse generated assertion: %w", err)
	}
	selections := map[string]map[string]any{}
	if err := visitAssertionMetrics(oldDoc, func(metric map[string]any) error {
		include, hasInclude := metric["datapoints/include"]
		count, hasCount := metric["datapoints/count"]
		if !hasInclude && !hasCount {
			return nil
		}
		name, ok := metric["name"].(string)
		if !ok || name == "" {
			return errors.New("metric with datapoint matchers has no name")
		}
		if _, duplicate := selections[name]; duplicate {
			return fmt.Errorf("metric %q has duplicate datapoint matchers", name)
		}
		selection := map[string]any{}
		if hasInclude {
			selection["datapoints/include"] = include
		}
		if hasCount {
			selection["datapoints/count"] = count
		}
		selections[name] = selection
		return nil
	}); err != nil {
		return nil, err
	}
	if len(selections) == 0 {
		return generated, nil
	}
	seen := map[string]bool{}
	if err := visitAssertionMetrics(newDoc, func(metric map[string]any) error {
		name, _ := metric["name"].(string)
		selection, ok := selections[name]
		if !ok {
			return nil
		}
		if seen[name] {
			return fmt.Errorf("generated assertion has duplicate metric %q", name)
		}
		seen[name] = true
		delete(metric, "datapoints")
		for key, value := range selection {
			metric[key] = value
		}
		return nil
	}); err != nil {
		return nil, err
	}
	for name := range selections {
		if !seen[name] {
			return nil, fmt.Errorf("generated assertion is missing metric %q", name)
		}
	}
	return yaml.Marshal(newDoc)
}

func visitAssertionMetrics(doc map[string]any, visit func(map[string]any) error) error {
	resources, resourcesOK := doc["resources"].([]any)
	if !resourcesOK {
		return errors.New("assertion resources must be a list")
	}
	for _, rawResource := range resources {
		resource, resourceOK := rawResource.(map[string]any)
		if !resourceOK {
			return errors.New("assertion resource must be a map")
		}
		scopes, scopesOK := resource["scopes"].([]any)
		if !scopesOK {
			return errors.New("assertion scopes must be a list")
		}
		for _, rawScope := range scopes {
			scope, scopeOK := rawScope.(map[string]any)
			if !scopeOK {
				return errors.New("assertion scope must be a map")
			}
			metrics, metricsOK := scope["metrics"].([]any)
			if !metricsOK {
				return errors.New("assertion metrics must be a list")
			}
			for _, rawMetric := range metrics {
				metric, metricOK := rawMetric.(map[string]any)
				if !metricOK {
					return errors.New("assertion metric must be a map")
				}
				if err := visit(metric); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func nonIdentityDatapointAttrs(metrics pmetric.Metrics, exact map[string]struct{}) map[string]struct{} {
	attrs := make(map[string]struct{})
	add := func(m pcommon.Map) {
		m.Range(func(key string, _ pcommon.Value) bool {
			if _, ok := exact[key]; !ok {
				attrs[key] = struct{}{}
			}
			return true
		})
	}
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		for j := 0; j < metrics.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			ms := metrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				metric := ms.At(k)
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					for n := 0; n < metric.Gauge().DataPoints().Len(); n++ {
						add(metric.Gauge().DataPoints().At(n).Attributes())
					}
				case pmetric.MetricTypeSum:
					for n := 0; n < metric.Sum().DataPoints().Len(); n++ {
						add(metric.Sum().DataPoints().At(n).Attributes())
					}
				case pmetric.MetricTypeHistogram:
					for n := 0; n < metric.Histogram().DataPoints().Len(); n++ {
						add(metric.Histogram().DataPoints().At(n).Attributes())
					}
				case pmetric.MetricTypeExponentialHistogram:
					for n := 0; n < metric.ExponentialHistogram().DataPoints().Len(); n++ {
						add(metric.ExponentialHistogram().DataPoints().At(n).Attributes())
					}
				case pmetric.MetricTypeSummary:
					for n := 0; n < metric.Summary().DataPoints().Len(); n++ {
						add(metric.Summary().DataPoints().At(n).Attributes())
					}
				}
			}
		}
	}
	return attrs
}

// Scope version matchers are not yet supported by WriteAssertionFile options.
func markScopeVersionRegex(file, pattern string) error {
	if pattern == "" {
		return nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read assertion file %s: %w", file, err)
	}
	var doc map[string]any
	if err = yaml.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("parse assertion file %s: %w", file, err)
	}
	resources, resourcesOK := doc["resources"].([]any)
	if !resourcesOK {
		return fmt.Errorf("parse assertion file %s: resources must be a list", file)
	}
	for _, resource := range resources {
		res, resourceOK := resource.(map[string]any)
		if !resourceOK {
			return fmt.Errorf("parse assertion file %s: resource must be a map", file)
		}
		scopes, scopesOK := res["scopes"].([]any)
		if !scopesOK {
			return fmt.Errorf("parse assertion file %s: scopes must be a list", file)
		}
		for _, scope := range scopes {
			s, scopeOK := scope.(map[string]any)
			if !scopeOK {
				return fmt.Errorf("parse assertion file %s: scope must be a map", file)
			}
			delete(s, "version")
			s["version/regex"] = pattern
		}
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal assertion file %s: %w", file, err)
	}
	//nolint:gosec // Assertion snapshots are committed testdata.
	if err = os.WriteFile(file, out, 0o644); err != nil {
		return fmt.Errorf("write assertion file %s: %w", file, err)
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
