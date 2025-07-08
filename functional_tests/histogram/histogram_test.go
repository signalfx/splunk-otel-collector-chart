// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package histogram

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	signalFxReceiverPort = 4317

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
		"IngestURL": fmt.Sprintf("http://%s:%d", hostEp, signalFxReceiverPort),
	}
	valuesFile, err := filepath.Abs(filepath.Join("testdata", valuesDir, "test_values.yaml.tmpl"))
	require.NoError(t, err)
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 0, internal.GetDefaultChartOptions())
}

func teardown(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	internal.ChartUninstall(t, testKubeConfig)
}

type TestInput struct {
	ServiceName            string // service name
	NonHistogramMetricName string // metric name which is expected to be present for the component
	HistogramMetricName    string // metric name which is expected to be present for the component only
}

var testInputs = []TestInput{
	{
		ServiceName:            "kubernetes-scheduler",
		NonHistogramMetricName: "scheduler_queue_incoming_pods_total",
		HistogramMetricName:    "scheduler_scheduling_algorithm_duration_seconds",
	},
	{
		ServiceName:            "kubernetes-proxy",
		NonHistogramMetricName: "kubeproxy_sync_proxy_rules_service_changes_total",
		HistogramMetricName:    "kubeproxy_sync_proxy_rules_duration_seconds",
	},
	{
		ServiceName:            "kubernetes-apiserver",
		NonHistogramMetricName: "apiserver_request_total",
		HistogramMetricName:    "apiserver_request_duration_seconds",
	},
	{
		ServiceName:            "kube-controller-manager",
		NonHistogramMetricName: "workqueue_retries_total",
		HistogramMetricName:    "workqueue_queue_duration_seconds",
	},
	{
		ServiceName:            "coredns",
		NonHistogramMetricName: "coredns_dns_requests_total",
		HistogramMetricName:    "coredns_dns_request_duration_seconds",
	},
	{
		ServiceName:            "etcd",
		NonHistogramMetricName: "etcd_server_is_leader",
		HistogramMetricName:    "etcd_disk_wal_fsync_duration_seconds",
	},
}

func Test_ControlPlaneMetrics(t *testing.T) {
	metricsSink := internal.SetupSignalfxReceiver(t, signalFxReceiverPort)

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t)
	}

	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		// generate some traffic to coredns to reduce metric flakiness
		performDNSQueries(t)
		deployChart(t)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	for _, isHistogram := range []bool{true, false} {
		for _, input := range testInputs {
			t.Run(fmt.Sprintf("%s_histograms=%t", input.ServiceName, isHistogram), func(t *testing.T) {
				runMetricsTest(t, isHistogram, metricsSink, input)
			})
		}
	}
}

func runMetricsTest(t *testing.T, isHistogram bool, metricsSink *consumertest.MetricsSink, input TestInput) {
	k8sVersion := os.Getenv("K8S_VERSION")
	majorMinor := k8sVersion[0:strings.LastIndex(k8sVersion, ".")]

	testDir := filepath.Join("testdata", "expected", majorMinor)

	internal.WaitForMetrics(t, 5, metricsSink)

	fileName := input.ServiceName + "_metrics.yaml"
	if isHistogram {
		fileName = input.ServiceName + "_histogram_metrics.yaml"
	}
	expected, err := golden.ReadMetrics(filepath.Join(testDir, fileName))
	require.NoError(t, err, "Failed to read expected metrics from %s", filepath.Join(testDir, fileName))
	require.NotNil(t, expected, "Expected metrics should not be nil")
	expectedMetrics := &expected

	var actualMetrics *pmetric.Metrics

	t.Logf("checking for metrics matching component %s", input.ServiceName)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		for h := len(metricsSink.AllMetrics()) - 1; h >= 0; h-- {
			m := metricsSink.AllMetrics()[h]
		OUTER:
			for i := 0; i < m.ResourceMetrics().Len(); i++ {
				for j := 0; j < m.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
					for k := 0; k < m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
						metricToConsider := m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
						metricName := input.NonHistogramMetricName
						if isHistogram {
							metricName = input.HistogramMetricName
						}
						if metricToConsider.Name() == metricName {
							actualMetrics = &m
							break OUTER
						}
					}
				}
			}
		}

		assert.NotNil(tt, actualMetrics, "Did not receive any metrics for component %s", input.ServiceName)
	}, 3*time.Minute, 5*time.Second)

	// Set GENERATE_EXPECTED to true to get a sample of the metrics for component - only for dev purposes
	// The max datapoint count per metric can be adjusted as input to internal.ReduceDatapoints
	if os.Getenv("GENERATE_EXPECTED") == "true" {
		outputDir := filepath.Join("testdata", "expected", majorMinor)
		require.NoError(t, os.MkdirAll(outputDir, 0o755))
		require.NotNil(t, actualMetrics, "Did not receive any metrics for component %s", input.ServiceName)
		internal.ReduceDatapoints(actualMetrics, 1)
		err := golden.WriteMetrics(t, filepath.Join(outputDir, fileName), *actualMetrics)
		require.NoError(t, err)
	}

	require.NotNil(t, actualMetrics, "Did not receive any metrics for component %s", input.ServiceName)
	internal.MaybeUpdateExpectedMetricsResults(t, filepath.Join(testDir, fileName), actualMetrics)
	err = checkMetrics(t, isHistogram, expectedMetrics, actualMetrics, input.ServiceName)
	if err != nil {
		t.Errorf("Error occurred while checking metrics for component %s: %v", input.ServiceName, err)
	}
}

func performDNSQueries(t *testing.T) {
	overrides := `{"spec": {"dnsPolicy": "ClusterFirst"}}`
	cmd := exec.Command("kubectl", "run", "--rm", "-i", "--tty", "dns-query", "--image=busybox", "--restart=Never", "--overrides="+overrides, "--", "sh", "-c", "for i in $(seq 1 10); do nslookup kubernetes.default.svc.cluster.local; sleep 1; done")
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	if err != nil {
		t.Logf("DNS query failed: %v", err)
		t.Logf("Standard Output: %s", out.String())
		t.Logf("Standard Error: %s", stderr.String())
	}
}

func checkMetrics(t *testing.T, isHistogram bool, expected, actual *pmetric.Metrics, component string) error {
	require.NotNil(t, expected, "Expected metrics should not be nil")
	require.NotNil(t, actual, "Actual metrics should not be nil")

	commonAttrs := map[string]string{
		"host.name":           "kind-control-plane",
		"k8s.cluster.name":    "sock",
		"k8s.namespace.name":  "kube-system",
		"k8s.node.name":       "kind-control-plane",
		"k8s.pod.uid":         ".*",
		"os.type":             "linux",
		"server.address":      ".*",
		"server.port":         ".*",
		"url.scheme":          "http",
		"service.instance.id": ".*:.*",
	}

	componentAttrs := map[string]map[string]string{
		"coredns": {
			"k8s.pod.name": "coredns-.*",
			"service.name": "coredns",
		},
		"etcd": {
			"k8s.pod.name": "etcd-kind-control-plane",
			"service.name": "etcd",
		},
		"kube-controller": {
			"k8s.pod.name": "kube-controller-manager-kind-control-plane",
			"service.name": "kube-controller-manager",
		},
		"kubernetes-apiserver": {
			"k8s.pod.name": "kube-apiserver-kind-control-plane",
			"service.name": "kubernetes-apiserver",
		},
		"kubernetes-proxy": {
			"k8s.pod.name": "kube-proxy-.*",
			"service.name": "kubernetes-proxy",
		},
		"kubernetes-scheduler": {
			"k8s.pod.name": "kube-scheduler-kind-control-plane",
			"service.name": "kubernetes-scheduler",
		},
	}

	mergedAttrs := mergeMaps(commonAttrs, componentAttrs[component])

	if isHistogram {
		return checkHistogramMetrics(t, expected, actual, component, mergedAttrs)
	}
	return checkNonHistogramMetrics(t, expected, actual, component, mergedAttrs)
}

func mergeMaps(map1, map2 map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range map1 {
		merged[k] = v
	}
	for k, v := range map2 {
		merged[k] = v
	}
	return merged
}

func checkHistogramMetrics(t *testing.T, expected, actual *pmetric.Metrics, component string, attrs map[string]string) error {
	for i := 0; i < expected.ResourceMetrics().Len(); i++ {
		for j := 0; j < actual.ResourceMetrics().Len(); j++ {
			actualRm := actual.ResourceMetrics().At(j)
			if err := checkAttributes(t, actualRm.Resource().Attributes(), attrs, component); err != nil {
				return err
			}
		}
	}

	expectedNames := internal.GetMetricNames(expected)
	actualNames := internal.GetMetricNames(actual)
	for _, name := range expectedNames {
		if !assert.Contains(t, actualNames, name, "Metric name %s not found in received metrics for component %s", name, component) {
			return fmt.Errorf("metric name %s not found in received metrics for component %s", name, component)
		}
	}
	return nil
}

func checkNonHistogramMetrics(t *testing.T, expected, actual *pmetric.Metrics, component string, attrs map[string]string) error {
	for i := 0; i < expected.ResourceMetrics().Len(); i++ {
		for j := 0; j < actual.ResourceMetrics().Len(); j++ {
			actualRm := actual.ResourceMetrics().At(j)
			for k := 0; k < actualRm.ScopeMetrics().Len(); k++ {
				sm := actualRm.ScopeMetrics().At(k)
				for l := 0; l < sm.Metrics().Len(); l++ {
					metric := sm.Metrics().At(l)
					switch metric.Type() {
					case pmetric.MetricTypeSum:
						for m := 0; m < metric.Sum().DataPoints().Len(); m++ {
							dp := metric.Sum().DataPoints().At(m)
							if err := checkAttributes(t, dp.Attributes(), attrs, component); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeGauge:
						for m := 0; m < metric.Gauge().DataPoints().Len(); m++ {
							dp := metric.Gauge().DataPoints().At(m)
							if err := checkAttributes(t, dp.Attributes(), attrs, component); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeHistogram:
						for m := 0; m < metric.Histogram().DataPoints().Len(); m++ {
							dp := metric.Histogram().DataPoints().At(m)
							if err := checkAttributes(t, dp.Attributes(), attrs, component); err != nil {
								return err
							}
						}
					case pmetric.MetricTypeSummary:
						for m := 0; m < metric.Summary().DataPoints().Len(); m++ {
							dp := metric.Summary().DataPoints().At(m)
							if err := checkAttributes(t, dp.Attributes(), attrs, component); err != nil {
								return err
							}
						}
					}
				}
			}
		}
	}

	expectedNames := internal.GetMetricNames(expected)
	actualNames := internal.GetMetricNames(actual)
	for _, name := range expectedNames {
		if !assert.Contains(t, actualNames, name, "Metric name %s not found in actual metrics", name) {
			return fmt.Errorf("metric name %s not found in actual metrics", name)
		}
	}
	return nil
}

func checkAttributes(t *testing.T, attrs pcommon.Map, expectedAttrs map[string]string, component string) error {
	for key, regex := range expectedAttrs {
		val, _ := attrs.Get(key)
		if !assert.Regexp(t, regex, val.AsString(), "Attribute %s does not match regex %s for component %s", key, regex, component) {
			return fmt.Errorf("attribute %s does not match regex %s for component %s", key, regex, component)
		}
	}
	return nil
}
