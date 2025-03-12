// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package histogram

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	otlpReceiverPort = 4317

	valuesDir = "values"
)

func deployChart(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}
	replacements := map[string]any{
		"IngestURL": fmt.Sprintf("http://%s:%d", hostEp, otlpReceiverPort),
	}
	valuesFile, err := filepath.Abs(filepath.Join("testdata", valuesDir, "test_values.yaml.tmpl"))
	require.NoError(t, err)
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements)
}

func teardown(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	internal.ChartUninstall(t, testKubeConfig)
}

func Test_Histograms(t *testing.T) {
	otlpMetricsSink := internal.SetupSignalfxReceiver(t, otlpReceiverPort)

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t)
	}
	// deploy the chart and applications.
	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		deployChart(t)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	t.Run("histogram metrics captured", testHistogramMetrics)
}

type TestInput struct {
	FileName   string // component specific metrics file
	MetricName string // metric name which is expected to be present for the component only
}

func testHistogramMetrics(t *testing.T) {
	k8sVersion := os.Getenv("K8S_VERSION")
	majorMinor := k8sVersion[0:strings.LastIndex(k8sVersion, ".")]

	testDir := filepath.Join("testdata", "expected", majorMinor)

	internal.WaitForMetrics(t, 5, otlpMetricsSink)

	testInputs := []TestInput{
		{"scheduler_metrics.yaml", "scheduler_queue_incoming_pods_total"},
		{"kubeproxy_metrics.yaml", "kubeproxy_sync_proxy_rules_iptables_total"},
		{"api_server_metrics.yaml", "apiserver_request_total"},
		{"controller_manager_metrics.yaml", "endpoint_slice_controller_endpoints_removed_per_sync"},
		{"coredns_metrics.yaml", "coredns_dns_request_duration_seconds"},
		{"etcd_metrics.yaml", "etcd_cluster_version"},
	}

	expectedMetrics := make(map[string]*pmetric.Metrics)
	for _, input := range testInputs {
		expected, _ := golden.ReadMetrics(filepath.Join(testDir, input.FileName))
		expectedMetrics[input.FileName] = &expected
	}

	var actualMetrics = make(map[string]*pmetric.Metrics)

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		t.Log("checking for metrics matching components")

		for h := len(otlpMetricsSink.AllMetrics()) - 1; h >= 0; h-- {
			m := otlpMetricsSink.AllMetrics()[h]
		OUTER:
			for i := 0; i < m.ResourceMetrics().Len(); i++ {
				for j := 0; j < m.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
					for k := 0; k < m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
						metricToConsider := m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
						for _, input := range testInputs {
							if metricToConsider.Name() == input.MetricName && matchesExpectedMetrics(&m, expectedMetrics[input.FileName], false) {
								actualMetrics[input.FileName] = &m
								break OUTER
							}
						}
					}
				}
			}
		}

		for _, input := range testInputs {
			assert.NotNil(tt, actualMetrics[input.FileName], "Did not receive any metrics for component %s", strings.TrimSuffix(input.FileName, "_metrics.yaml"))
		}
	}, 3*time.Minute, 5*time.Second)

	for _, input := range testInputs {
		t.Run(input.FileName, func(t *testing.T) {
			actual := actualMetrics[input.FileName]
			component := strings.TrimSuffix(input.FileName, "_metrics.yaml")
			assert.NotNil(t, actual, "Did not receive any metrics for component %s", component)
			compareOptions, err := getCompareMetricsOptions(input.FileName, expectedMetrics[input.FileName], actual)
			err = pmetrictest.CompareMetrics(*expectedMetrics[input.FileName], *actual, compareOptions...)
			assert.NoError(t, err, "Error occurred while comparing metrics for component %s", component)
			if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
				internal.WriteNewExpectedMetricsResult(t, filepath.Join(testDir, input.FileName), actual)
			}
		})
	}
}

func matchesExpectedMetrics(actual, expected *pmetric.Metrics, ignoreLen bool) bool {
	if actual == nil || expected == nil {
		return false
	}
	if ignoreLen {
		return actual.ResourceMetrics().Len() == expected.ResourceMetrics().Len()
	}
	return actual.MetricCount() == expected.MetricCount() && actual.ResourceMetrics().Len() == expected.ResourceMetrics().Len()
}

func getCompareMetricsOptions(file string, expectedMetrics *pmetric.Metrics, actualMetrics *pmetric.Metrics) ([]pmetrictest.CompareMetricsOption, error) {
	commonIgnoreMetricAttributes := []pmetrictest.CompareMetricsOption{
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.port"),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id"),
		pmetrictest.IgnoreMetricAttributeValue("server.address"),
	}

	commonIgnoreOptions := []pmetrictest.CompareMetricsOption{
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreDatapointAttributesOrder(),
	}

	var componentIgnoreOptions []pmetrictest.CompareMetricsOption

	switch file {
	case "coredns_metrics.yaml":
		componentIgnoreOptions = []pmetrictest.CompareMetricsOption{
			pmetrictest.IgnoreResourceAttributeValue("k8s.pod.uid"),
			pmetrictest.IgnoreResourceAttributeValue("k8s.pod.name"),
			pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
			pmetrictest.IgnoreResourceAttributeValue("net.host.name"),
			pmetrictest.IgnoreResourceAttributeValue("server.address"),
			pmetrictest.IgnoreMetricAttributeValue("to", "coredns_forward_request_duration_seconds", "coredns_proxy_request_duration_seconds"),
			pmetrictest.IgnoreMetricAttributeValue("rcode", "coredns_forward_request_duration_seconds", "coredns_proxy_request_duration_seconds"),
			pmetrictest.IgnoreSubsequentDataPoints("coredns_forward_request_duration_seconds", "coredns_proxy_request_duration_seconds"),
		}
	case "scheduler_metrics.yaml":
		componentIgnoreOptions = []pmetrictest.CompareMetricsOption{
			pmetrictest.IgnoreMetricAttributeValue("extension_point", "scheduler_plugin_execution_duration_seconds"),
			pmetrictest.IgnoreMetricAttributeValue("operation", "scheduler_goroutines"),
			pmetrictest.IgnoreMetricAttributeValue("plugin", "scheduler_plugin_execution_duration_seconds", "scheduler_unschedulable_pods"),
			pmetrictest.IgnoreSubsequentDataPoints("scheduler_plugin_execution_duration_seconds", "scheduler_goroutines", "scheduler_queue_incoming_pods_total", "scheduler_unschedulable_pods"),
		}
	case "kubeproxy_metrics.yaml":
		componentIgnoreOptions = []pmetrictest.CompareMetricsOption{
			pmetrictest.IgnoreMetricAttributeValue("version", "go_info"),
			pmetrictest.IgnoreMetricAttributeValue("build_date", "kubernetes_build_info"),
			pmetrictest.IgnoreMetricAttributeValue("git_commit", "kubernetes_build_info"),
			pmetrictest.IgnoreMetricAttributeValue("git_version", "kubernetes_build_info"),
			pmetrictest.IgnoreMetricAttributeValue("go_version", "kubernetes_build_info"),
			pmetrictest.IgnoreMetricAttributeValue("minor", "kubernetes_build_info"),
			pmetrictest.IgnoreMetricAttributeValue("platform", "kubernetes_build_info"),
			pmetrictest.IgnoreMetricAttributeValue("server_go_version", "etcd_server_go_version"),
		}
	case "api_server_metrics.yaml":
		metricNamesMap := make(map[string]struct{})
		for i := 0; i < expectedMetrics.ResourceMetrics().Len(); i++ {
			for j := 0; j < expectedMetrics.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
				for k := 0; k < expectedMetrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
					metric := expectedMetrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
					metricNamesMap[metric.Name()] = struct{}{}
				}
			}
		}
		var metricNames []string
		for name := range metricNamesMap {
			metricNames = append(metricNames, name)
		}

		// these metrics do not show up consistently from apiserver
		flakyMetrics := []string{
			"apiserver_request_post_timeout_total",
			"apiserver_request_terminations_total",
			"apiserver_init_events_total",
		}
		internal.RemoveFlakyMetrics(actualMetrics, flakyMetrics)

		// apiserver_request_total metric has many datapoints each with different combination of below attributes; removing these for reduced failures in comparison
		removeAttr := []string{"code", "resource", "subresource", "verb", "component", "scope", "version"}
		removeAttributes(expectedMetrics, "apiserver_request_total", removeAttr)
		removeAttributes(actualMetrics, "apiserver_request_total", removeAttr)

		// apiserver_watch_events_total metric can conditionally have below attributes
		removeAttributes(expectedMetrics, "apiserver_watch_events_total", []string{"group"})

		componentIgnoreOptions = []pmetrictest.CompareMetricsOption{
			pmetrictest.IgnoreSubsequentDataPoints(metricNames...),
			pmetrictest.IgnoreMetricAttributeValue("kind", "apiserver_watch_events_total"),
			pmetrictest.IgnoreMetricAttributeValue("request_kind", "apiserver_current_inflight_requests", "apiserver_current_inqueue_requests"),
			pmetrictest.IgnoreMetricAttributeValue("operation", "apiserver_admission_step_admission_duration_seconds_summary_quantile", "apiserver_admission_step_admission_duration_seconds_summary_count", "apiserver_admission_step_admission_duration_seconds_summary_sum"),
			pmetrictest.IgnoreMetricAttributeValue("resource", "apiserver_storage_objects", "apiserver_watch_cache_events_dispatched_total", "etcd_bookmark_counts"),
			pmetrictest.IgnoreMetricAttributeValue("method", "rest_client_requests_total"),
			pmetrictest.IgnoreMetricAttributeValue("rejected", "apiserver_admission_step_admission_duration_seconds_summary_count", "apiserver_admission_step_admission_duration_seconds_summary_sum"),
			pmetrictest.IgnoreMetricAttributeValue("type", "apiserver_admission_step_admission_duration_seconds_summary_count", "apiserver_admission_step_admission_duration_seconds_summary_sum"),
			pmetrictest.IgnoreMetricAttributeValue("flow_schema", "apiserver_flowcontrol_dispatched_requests_total"),
			pmetrictest.IgnoreMetricAttributeValue("priority_level", "apiserver_flowcontrol_dispatched_requests_total"),
			pmetrictest.IgnoreMetricAttributeValue("version", "apiserver_request_total"),
		}
	case "controller_manager_metrics.yaml":
		componentIgnoreOptions = []pmetrictest.CompareMetricsOption{
			pmetrictest.IgnoreResourceAttributeValue("k8s.pod.uid"),
			// pmetrictest.IgnoreResourceAttributeValue("service.name"),
			pmetrictest.IgnoreResourceAttributeValue("server.port"),
		}
	case "etcd_metrics.yaml":
		componentIgnoreOptions = []pmetrictest.CompareMetricsOption{
			pmetrictest.IgnoreMetricAttributeValue("build", "etcd_server_info"),
			pmetrictest.IgnoreMetricAttributeValue("server_go_version", "etcd_server_go_version"),
			pmetrictest.IgnoreMetricAttributeValue("server_version", "etcd_server_version"),
			pmetrictest.IgnoreMetricAttributeValue("version", "go_info"),
			pmetrictest.IgnoreMetricAttributeValue("client_api_version", "etcd_server_client_requests_total"),
		}
	default:
		return nil, fmt.Errorf("unknown metrics file: %s", file)
	}

	allOptions := append(commonIgnoreMetricAttributes, componentIgnoreOptions...)
	allOptions = append(allOptions, commonIgnoreOptions...)

	return allOptions, nil
}

func removeAttributes(metrics *pmetric.Metrics, metricName string, attributes []string) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == metricName {
					if metric.Type() == pmetric.MetricTypeSum {
						for l := 0; l < metric.Sum().DataPoints().Len(); l++ {
							dp := metric.Sum().DataPoints().At(l)
							for _, key := range attributes {
								if _, ok := dp.Attributes().Get(key); ok {
									dp.Attributes().Remove(key)
								}
							}
						}
					}
				}
			}
		}
	}
}
