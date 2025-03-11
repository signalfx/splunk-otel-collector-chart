// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package histogram

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	otlpReceiverPort = 4317

	valuesDir = "values"
)

var setupRun = sync.Once{}

var histogramMetricsConsumer *consumertest.MetricsSink

func setupOnce(t *testing.T) *consumertest.MetricsSink {
	setupRun.Do(func() {
		histogramMetricsConsumer = internal.SetupSignalfxReceiver(t, otlpReceiverPort)

		if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
			teardown(t)
		}
		// deploy the chart and applications.
		if os.Getenv("SKIP_SETUP") == "true" {
			t.Log("Skipping setup as SKIP_SETUP is set to true")
			return
		}
		deployChartsAndApps(t)
	})

	return histogramMetricsConsumer
}

func deployChartsAndApps(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	chart := internal.LoadCollectorChart(t, "")

	valuesBytes, err := os.ReadFile(filepath.Join("testdata", valuesDir, "test_values.yaml.tmpl"))
	require.NoError(t, err)

	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}

	replacements := struct {
		IngestURL string
	}{
		fmt.Sprintf("http://%s:%d", hostEp, otlpReceiverPort),
	}
	tmpl, err := template.New("").Parse(string(valuesBytes))
	require.NoError(t, err)
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, replacements)
	require.NoError(t, err)
	var values map[string]interface{}
	err = yaml.Unmarshal(buf.Bytes(), &values)
	require.NoError(t, err)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format+"\n", v...)
	}); err != nil {
		require.NoError(t, err)
	}
	install := action.NewInstall(actionConfig)
	install.Namespace = "default"
	install.ReleaseName = "sock"
	_, err = install.Run(chart, values)
	if err != nil {
		t.Logf("error reported during helm install: %v\n", err)
		retryUpgrade := action.NewUpgrade(actionConfig)
		retryUpgrade.Namespace = "default"
		retryUpgrade.Install = true
		_, err = retryUpgrade.Run("sock", chart, values)
		require.NoError(t, err)
	}
}

func teardown(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format+"\n", v)
	}); err != nil {
		require.NoError(t, err)
	}
	uninstall := action.NewUninstall(actionConfig)
	uninstall.IgnoreNotFound = true
	uninstall.Wait = true
	_, _ = uninstall.Run("sock")
}

func Test_Histograms(t *testing.T) {
	_ = setupOnce(t)
	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	t.Run("histogram metrics captured", testHistogramMetrics)
}

func testHistogramMetrics(t *testing.T) {
	k8sVersion := os.Getenv("K8S_VERSION")
	majorMinor := k8sVersion[0:strings.LastIndex(k8sVersion, ".")]

	testDir := filepath.Join("testdata", "expected", majorMinor)

	otlpMetricsSink := setupOnce(t)
	internal.WaitForMetrics(t, 5, otlpMetricsSink)

	expectedKubeSchedulerMetricsFile := filepath.Join(testDir, "scheduler_metrics.yaml")
	expectedKubeSchedulerMetrics, err := golden.ReadMetrics(expectedKubeSchedulerMetricsFile)
	require.NoError(t, err)

	expectedKubeProxyMetricsFile := filepath.Join(testDir, "proxy_metrics.yaml")
	expectedKubeProxyMetrics, err := golden.ReadMetrics(expectedKubeProxyMetricsFile)
	require.NoError(t, err)

	expectedApiMetricsFile := filepath.Join(testDir, "api_metrics.yaml")
	expectedApiMetrics, err := golden.ReadMetrics(expectedApiMetricsFile)
	require.NoError(t, err)

	expectedControllerManagerMetricsFile := filepath.Join(testDir, "controller_manager_metrics.yaml")
	expectedControllerManagerMetrics, err := golden.ReadMetrics(expectedControllerManagerMetricsFile)
	require.NoError(t, err)

	expectedCoreDNSMetricsFile := filepath.Join(testDir, "coredns_metrics.yaml")
	expectedCoreDNSMetrics, err := golden.ReadMetrics(expectedCoreDNSMetricsFile)
	require.NoError(t, err)

	expectedEtcdMetricsFile := filepath.Join(testDir, "etcd_metrics.yaml")
	expectedEtcdMetrics, err := golden.ReadMetrics(expectedEtcdMetricsFile)
	require.NoError(t, err)

	var corednsMetrics *pmetric.Metrics
	var schedulerMetrics *pmetric.Metrics
	var kubeProxyMetrics *pmetric.Metrics
	var apiMetrics *pmetric.Metrics
	var controllerManagerMetrics *pmetric.Metrics
	var etcdMetrics *pmetric.Metrics

	require.EventuallyWithT(t, func(tt *assert.CollectT) {

		for h := len(otlpMetricsSink.AllMetrics()) - 1; h >= 0; h-- {
			m := otlpMetricsSink.AllMetrics()[h]
		OUTER:
			for i := 0; i < m.ResourceMetrics().Len(); i++ {
				for j := 0; j < m.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
					for k := 0; k < m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
						metricToConsider := m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
						if metricToConsider.Name() == "coredns_dns_request_duration_seconds" && m.MetricCount() == expectedCoreDNSMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedCoreDNSMetrics.ResourceMetrics().Len() {
							corednsMetrics = &m
							break OUTER
						} else if metricToConsider.Name() == "kubeproxy_sync_proxy_rules_iptables_total" && m.MetricCount() == expectedKubeProxyMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedKubeProxyMetrics.ResourceMetrics().Len() {
							kubeProxyMetrics = &m
							break OUTER
						} else if metricToConsider.Name() == "scheduler_queue_incoming_pods_total" && m.MetricCount() == expectedKubeSchedulerMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedKubeSchedulerMetrics.ResourceMetrics().Len() {
							schedulerMetrics = &m
							break OUTER
						} else if metricToConsider.Name() == "apiserver_audit_event_total" && m.MetricCount() == expectedApiMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedApiMetrics.ResourceMetrics().Len() {
							apiMetrics = &m
							break OUTER
						} else if metricToConsider.Name() == "workqueue_queue_duration_seconds" && m.MetricCount() == expectedControllerManagerMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedControllerManagerMetrics.ResourceMetrics().Len() {
							controllerManagerMetrics = &m
							break OUTER
						} else if metricToConsider.Name() == "etcd_cluster_version" && m.MetricCount() == expectedEtcdMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedEtcdMetrics.ResourceMetrics().Len() {
							etcdMetrics = &m
							break OUTER
						}
					}
				}
			}
		}
		assert.NotNil(tt, corednsMetrics)
		assert.NotNil(tt, schedulerMetrics)
		assert.NotNil(tt, kubeProxyMetrics)
		assert.NotNil(tt, apiMetrics)
		assert.NotNil(tt, controllerManagerMetrics)
		assert.NotNil(tt, etcdMetrics)

	}, 5*time.Minute, 5*time.Second)

	require.NotNil(t, corednsMetrics)
	require.NotNil(t, schedulerMetrics)
	require.NotNil(t, kubeProxyMetrics)
	require.NotNil(t, apiMetrics)
	require.NotNil(t, controllerManagerMetrics)
	require.NotNil(t, etcdMetrics)

	err = pmetrictest.CompareMetrics(expectedCoreDNSMetrics, *corednsMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreMetricAttributeValue("to", "coredns_forward_request_duration_seconds"),
		pmetrictest.IgnoreMetricAttributeValue("rcode", "coredns_forward_request_duration_seconds"),
		pmetrictest.IgnoreMetricAttributeValue("to", "coredns_proxy_request_duration_seconds"),
		pmetrictest.IgnoreMetricAttributeValue("rcode", "coredns_proxy_request_duration_seconds"),
		pmetrictest.IgnoreResourceAttributeValue("server.address"),
		pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.name"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.port"),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreSubsequentDataPoints("coredns_forward_request_duration_seconds"),
		pmetrictest.IgnoreSubsequentDataPoints("coredns_proxy_request_duration_seconds"),
	)
	assert.NoError(t, err)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedMetricsResult(t, expectedCoreDNSMetricsFile, corednsMetrics)
	}

	err = pmetrictest.CompareMetrics(expectedKubeSchedulerMetrics, *schedulerMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreMetricAttributeValue("extension_point", "scheduler_plugin_execution_duration_seconds"),
		pmetrictest.IgnoreMetricAttributeValue("plugin", "scheduler_plugin_execution_duration_seconds"),
		pmetrictest.IgnoreSubsequentDataPoints("scheduler_plugin_execution_duration_seconds"),
		pmetrictest.IgnoreResourceAttributeValue("server.address"),
		pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.name"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.port"), pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id"),
		pmetrictest.IgnoreMetricAttributeValue("server.address"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.port"),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	)
	assert.NoError(t, err)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedMetricsResult(t, expectedKubeSchedulerMetricsFile, etcdMetrics)
	}

	err = pmetrictest.CompareMetrics(expectedKubeProxyMetrics, *kubeProxyMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id"),
		pmetrictest.IgnoreMetricAttributeValue("server.address"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.port"),
		pmetrictest.IgnoreMetricAttributeValue("version", "go_info"),
		pmetrictest.IgnoreMetricAttributeValue("build_date", "kubernetes_build_info"),
		pmetrictest.IgnoreMetricAttributeValue("git_commit", "kubernetes_build_info"),
		pmetrictest.IgnoreMetricAttributeValue("git_version", "kubernetes_build_info"),
		pmetrictest.IgnoreMetricAttributeValue("go_version", "kubernetes_build_info"),
		pmetrictest.IgnoreMetricAttributeValue("minor", "kubernetes_build_info"),
		pmetrictest.IgnoreMetricAttributeValue("platform", "kubernetes_build_info"),
		pmetrictest.IgnoreMetricAttributeValue("server_go_version", "etcd_server_go_version"),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	)
	assert.NoError(t, err)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedMetricsResult(t, expectedKubeProxyMetricsFile, &expectedKubeProxyMetrics)
	}

	err = pmetrictest.CompareMetrics(expectedApiMetrics, *apiMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id"),
		pmetrictest.IgnoreMetricAttributeValue("server.address"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.port"),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	)
	assert.NoError(t, err)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedMetricsResult(t, expectedApiMetricsFile, apiMetrics)
	}

	err = pmetrictest.CompareMetrics(expectedControllerManagerMetrics, *controllerManagerMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id"),
		pmetrictest.IgnoreMetricAttributeValue("server.address"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.port"),
		pmetrictest.IgnoreResourceAttributeValue("server.address"),
		pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.name"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.port"),
		pmetrictest.IgnoreResourceAttributeValue("service.name"),
		pmetrictest.IgnoreResourceAttributeValue("server.port"),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	)
	assert.NoError(t, err)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedMetricsResult(t, expectedControllerManagerMetricsFile, controllerManagerMetrics)
	}

	err = pmetrictest.CompareMetrics(expectedEtcdMetrics, *etcdMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id"),
		pmetrictest.IgnoreMetricAttributeValue("server.address"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.port"),
		pmetrictest.IgnoreMetricAttributeValue("server_go_version", "etcd_server_go_version"),
		pmetrictest.IgnoreMetricAttributeValue("server_version", "etcd_server_version"),
		pmetrictest.IgnoreMetricAttributeValue("version", "go_info"),
		pmetrictest.IgnoreMetricAttributeValue("client_api_version", "etcd_server_client_requests_total"),
		pmetrictest.IgnoreResourceAttributeValue("server.address"),
		pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.name"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.port"),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	)
	assert.NoError(t, err)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedMetricsResult(t, expectedEtcdMetricsFile, etcdMetrics)
	}
}
