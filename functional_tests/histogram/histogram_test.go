// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package histogram

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	signalFxReceiverPort = 4317
	valuesDir            = "values"
	expectedDir          = "expected"
)

func deployChart(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}
	replacements := map[string]any{
		"IngestURL": internal.HostPortHTTP(hostEp, signalFxReceiverPort),
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
	ServiceName            string
	NonHistogramMetricName string
	HistogramMetricName    string
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
	internal.WaitForMetrics(t, 5, metricsSink)

	fileName := input.ServiceName + "_metrics_assertion.yaml"
	if isHistogram {
		fileName = input.ServiceName + "_histogram_metrics_assertion.yaml"
	}
	expectedFilePath := filepath.Join("testdata", expectedDir, fileName)

	metricName := input.NonHistogramMetricName
	if isHistogram {
		metricName = input.HistogramMetricName
	}

	var actualMetrics *pmetric.Metrics
	t.Logf("checking for metrics matching component %s", input.ServiceName)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		for h := len(metricsSink.AllMetrics()) - 1; h >= 0; h-- {
			m := metricsSink.AllMetrics()[h]
			for i := 0; i < m.ResourceMetrics().Len(); i++ {
				rm := m.ResourceMetrics().At(i)
				if resourceMetricsContains(rm, metricName) {
					filtered := pmetric.NewMetrics()
					rm.CopyTo(filtered.ResourceMetrics().AppendEmpty())
					actualMetrics = &filtered
					break
				}
			}
			if actualMetrics != nil {
				break
			}
		}

		assert.NotNil(tt, actualMetrics, "Did not receive any metrics for component %s", input.ServiceName)
	}, 3*time.Minute, 5*time.Second)

	require.NotNil(t, actualMetrics, "Did not receive any metrics for component %s", input.ServiceName)
	opts := []internal.MetricsAssertionOption{
		internal.WithRegexAttributes(histogramAssertionRegexAttrs(input.ServiceName)),
		internal.WithDatapointAttributes(histogramDatapointAttrs...),
		internal.WithFirstDatapointOnlyForAllMetrics(),
		internal.WithIgnoredScopeVersion(),
		internal.WithExpectedMetricsOnly(),
	}
	if isHistogram {
		opts = append(opts, internal.WithHistogramExplicitBounds())
	}
	internal.AssertMetricsDataSnapshot(t, expectedFilePath, *actualMetrics, opts...)
}

var histogramCommonRegexAttrs = map[string]string{
	"k8s.pod.uid":         internal.K8sUIDRegex,
	"server.address":      `.+`,
	"server.port":         `[0-9]+`,
	"service.instance.id": `.+`,
}

var histogramDatapointAttrs = []string{
	"host.name",
	"k8s.cluster.name",
	"k8s.namespace.name",
	"k8s.node.name",
	"k8s.pod.name",
	"k8s.pod.uid",
	"os.type",
	"server.address",
	"server.port",
	"service.instance.id",
	"service.name",
	"url.scheme",
}

var histogramComponentRegexAttrs = map[string]map[string]string{
	"coredns": {
		"k8s.pod.name": `coredns-.*`,
	},
	"kubernetes-proxy": {
		"k8s.pod.name": `kube-proxy-.*`,
	},
}

// histogramAssertionRegexAttrs keeps generated pod and endpoint attrs flexible.
func histogramAssertionRegexAttrs(component string) map[string]string {
	attrs := make(map[string]string, len(histogramCommonRegexAttrs)+len(histogramComponentRegexAttrs[component]))
	for k, v := range histogramCommonRegexAttrs {
		attrs[k] = v
	}
	for k, v := range histogramComponentRegexAttrs[component] {
		attrs[k] = v
	}
	return attrs
}

func resourceMetricsContains(rm pmetric.ResourceMetrics, name string) bool {
	for j := 0; j < rm.ScopeMetrics().Len(); j++ {
		for k := 0; k < rm.ScopeMetrics().At(j).Metrics().Len(); k++ {
			if rm.ScopeMetrics().At(j).Metrics().At(k).Name() == name {
				return true
			}
		}
	}
	return false
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
