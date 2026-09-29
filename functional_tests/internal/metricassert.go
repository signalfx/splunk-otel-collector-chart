// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
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
}

// MetricsAssertionOption configures snapshot generation.
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

// WithScopeVersionRegex writes scope versions as pmetricassert `version/regex` matchers.
func WithScopeVersionRegex(pattern string) MetricsAssertionOption {
	return func(cfg *metricsAssertionConfig) {
		cfg.scopeVersionRegex = pattern
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

// AssertMetricsSnapshot waits for a live batch that matches the assertion.
func AssertMetricsSnapshot(t *testing.T, sink *consumertest.MetricsSink, targetMetric, assertionFile string, timeout, interval time.Duration, opts ...MetricsAssertionOption) {
	t.Helper()

	if shouldUpdateExpectedResults() {
		selected := waitForMetricSet(targetMetric, sink, timeout, interval)
		require.NotNil(t, selected, "No metrics batch found containing target metric: %s", targetMetric)
		require.NoError(t, WriteMetricsAssertion(t, assertionFile, *selected, opts...))
		t.Logf("Wrote updated expected metric assertion to %s", assertionFile)
		return
	}

	selected, assertErr := selectMetricSetByAssertionWithTimeout(targetMetric, sink, assertionFile, timeout, interval)
	require.NotNil(t, selected, "No metrics batch found containing target metric: %s", targetMetric)
	require.NoError(t, assertErr, "Metric assertion failed for %s", assertionFile)
	t.Logf("Metric assertion passed for %d metrics (%s)", selected.MetricCount(), assertionFile)
}

func selectMetricSetByAssertionWithTimeout(targetMetric string, sink *consumertest.MetricsSink, assertionFile string, timeout, interval time.Duration) (*pmetric.Metrics, error) {
	deadline := time.Now().Add(timeout)
	checked := 0
	for time.Now().Before(deadline) {
		batches := sink.AllMetrics()
		if checked > len(batches) {
			checked = 0
		}
		for _, metrics := range batches[checked:] {
			if !containsMetric(metrics, targetMetric) {
				continue
			}
			if err := pmetricassert.AssertMetrics(assertionFile, metrics); err == nil {
				return &metrics, nil
			}
		}
		checked = len(batches)
		time.Sleep(interval)
	}

	selected := richestMetricSet(targetMetric, sink)
	if selected == nil {
		return nil, nil
	}
	return selected, pmetricassert.AssertMetrics(assertionFile, *selected)
}

func waitForMetricSet(targetMetric string, sink *consumertest.MetricsSink, timeout, interval time.Duration) *pmetric.Metrics {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if current := richestMetricSet(targetMetric, sink); current != nil {
			return current
		}
		time.Sleep(interval)
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

// WriteMetricsAssertion writes an assertion snapshot with flexible attribute matchers.
func WriteMetricsAssertion(tb testing.TB, file string, actual pmetric.Metrics, opts ...MetricsAssertionOption) error {
	tb.Helper()
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
	if err := pmetricassert.WriteAssertionFile(tb, file, actual, writeOpts...); err != nil {
		return fmt.Errorf("write assertion file %s: %w", file, err)
	}
	return markScopeVersionRegex(file, cfg.scopeVersionRegex)
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
