// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build histogram

package functional_tests

import (
	"bytes"
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
)

const (
	otlpReceiverPort = 4317

	testValuesFile = "test_values.yaml.tmpl"
	testDir        = "testdata_histogram"
	valuesDir      = "values"
)

var setupRun = sync.Once{}

var histogramMetricsConsumer *consumertest.MetricsSink

func setupOnce(t *testing.T) *consumertest.MetricsSink {
	setupRun.Do(func() {
		histogramMetricsConsumer = setupOtlpReceiver(t, otlpReceiverPort)

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

func setupOtlpReceiver(t *testing.T, port int) *consumertest.MetricsSink {
	mc := new(consumertest.MetricsSink)
	f := signalfxreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*signalfxreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", port)

	rcvr, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, mc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating metrics receiver")
	t.Cleanup(func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	})

	return mc
}

func deployChartsAndApps(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	chartPath := filepath.Join("..", "helm-charts", "splunk-otel-collector")
	chart, err := loader.Load(chartPath)
	require.NoError(t, err)

	valuesBytes, err := os.ReadFile(filepath.Join(testDir, valuesDir, "test_values.yaml.tmpl"))
	require.NoError(t, err)

	hostEp := hostEndpoint(t)
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
	otlpMetricsSink := setupOnce(t)
	waitForMetrics(t, 5, otlpMetricsSink)

	expectedKubeSchedulerMetrics, err := golden.ReadMetrics(filepath.Join(testDir, "scheduler_metrics.yaml"))
	require.NoError(t, err)

	expectedKubeProxyMetrics, err := golden.ReadMetrics(filepath.Join(testDir, "proxy_metrics.yaml"))
	require.NoError(t, err)

	expectedApiMetrics, err := golden.ReadMetrics(filepath.Join(testDir, "api_metrics.yaml"))
	require.NoError(t, err)

	expectedControllerManagerMetrics, err := golden.ReadMetrics(filepath.Join(testDir, "controller_manager_metrics.yaml"))
	require.NoError(t, err)

	expectedCoreDNSMetrics, err := golden.ReadMetrics(filepath.Join(testDir, "coredns_metrics.yaml"))
	require.NoError(t, err)

	var corednsMetrics *pmetric.Metrics
	var schedulerMetrics *pmetric.Metrics
	var kubeProxyMetrics *pmetric.Metrics
	var apiMetrics *pmetric.Metrics
	var controllerManagerMetrics *pmetric.Metrics

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
						} else if metricToConsider.Name() == "scheduler_scheduling_attempt_duration_seconds" && m.MetricCount() == expectedKubeSchedulerMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedKubeSchedulerMetrics.ResourceMetrics().Len() {
							schedulerMetrics = &m
							break OUTER
						} else if metricToConsider.Name() == "apiserver_audit_event_total" && m.MetricCount() == expectedApiMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedApiMetrics.ResourceMetrics().Len() {
							apiMetrics = &m
							break OUTER
						} else if metricToConsider.Name() == "workqueue_queue_duration_seconds" && m.MetricCount() == expectedControllerManagerMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedControllerManagerMetrics.ResourceMetrics().Len() {
							controllerManagerMetrics = &m
							break OUTER
						} else {
							fmt.Println(metricToConsider.Name())
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

	}, 3*time.Minute, 5*time.Second)

	require.NotNil(t, corednsMetrics)
	require.NotNil(t, schedulerMetrics)
	require.NotNil(t, kubeProxyMetrics)
	require.NotNil(t, apiMetrics)
	require.NotNil(t, controllerManagerMetrics)

	err = pmetrictest.CompareMetrics(expectedCoreDNSMetrics, *corednsMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreResourceAttributeValue("server.address"),
		pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.name"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.port"),
	)
	assert.NoError(t, err)

	err = pmetrictest.CompareMetrics(expectedKubeSchedulerMetrics, *schedulerMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreSubsequentDataPoints("scheduler_plugin_execution_duration_seconds"),
		pmetrictest.IgnoreResourceAttributeValue("server.address"),
		pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.name"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.port"),
	)
	assert.NoError(t, err)

	metricNames := []string{"aggregator_discovery_aggregation_count_total",
		"apiserver_audit_event_total",
		"apiserver_audit_requests_rejected_total",
		"apiserver_envelope_encryption_dek_cache_fill_percent",
		"apiserver_storage_data_key_generation_failures_total",
		"apiserver_storage_envelope_transformation_cache_misses_total",
		"apiserver_webhooks_x509_insecure_sha1_total",
		"apiserver_webhooks_x509_missing_san_total",
		"disabled_metrics_total",
		"go_cgo_go_to_c_calls_calls_total",
		"go_cpu_classes_gc_mark_assist_cpu_seconds_total",
		"go_cpu_classes_gc_mark_dedicated_cpu_seconds_total",
		"go_cpu_classes_gc_mark_idle_cpu_seconds_total",
		"go_cpu_classes_gc_pause_cpu_seconds_total",
		"go_cpu_classes_gc_total_cpu_seconds_total",
		"go_cpu_classes_idle_cpu_seconds_total",
		"go_cpu_classes_scavenge_assist_cpu_seconds_total",
		"go_cpu_classes_scavenge_background_cpu_seconds_total",
		"go_cpu_classes_scavenge_total_cpu_seconds_total",
		"go_cpu_classes_total_cpu_seconds_total",
		"go_cpu_classes_user_cpu_seconds_total",
		"go_gc_cycles_automatic_gc_cycles_total",
		"go_gc_cycles_forced_gc_cycles_total",
		"go_gc_cycles_total_gc_cycles_total",
		"go_gc_duration_seconds_count",
		"go_gc_duration_seconds_quantile",
		"go_gc_duration_seconds_sum",
		"go_gc_heap_allocs_bytes_total",
		"go_gc_heap_allocs_objects_total",
		"go_gc_heap_frees_bytes_total",
		"go_gc_heap_frees_objects_total",
		"go_gc_heap_goal_bytes",
		"go_gc_heap_objects_objects",
		"go_gc_heap_tiny_allocs_objects_total",
		"go_gc_limiter_last_enabled_gc_cycle",
		"go_gc_stack_starting_size_bytes",
		"go_goroutines",
		"go_info",
		"go_memory_classes_heap_free_bytes",
		"go_memory_classes_heap_objects_bytes",
		"go_memory_classes_heap_released_bytes",
		"go_memory_classes_heap_stacks_bytes",
		"go_memory_classes_heap_unused_bytes",
		"go_memory_classes_metadata_mcache_free_bytes",
		"go_memory_classes_metadata_mcache_inuse_bytes",
		"go_memory_classes_metadata_mspan_free_bytes",
		"go_memory_classes_metadata_mspan_inuse_bytes",
		"go_memory_classes_metadata_other_bytes",
		"go_memory_classes_os_stacks_bytes",
		"go_memory_classes_other_bytes",
		"go_memory_classes_profiling_buckets_bytes",
		"go_memory_classes_total_bytes",
		"go_memstats_alloc_bytes",
		"go_memstats_alloc_bytes_total",
		"go_memstats_buck_hash_sys_bytes",
		"go_memstats_frees_total",
		"go_memstats_gc_sys_bytes",
		"go_memstats_heap_alloc_bytes",
		"go_memstats_heap_idle_bytes",
		"go_memstats_heap_inuse_bytes",
		"go_memstats_heap_objects",
		"go_memstats_heap_released_bytes",
		"go_memstats_heap_sys_bytes",
		"go_memstats_last_gc_time_seconds",
		"go_memstats_lookups_total",
		"go_memstats_mallocs_total",
		"go_memstats_mcache_inuse_bytes",
		"go_memstats_mcache_sys_bytes",
		"go_memstats_mspan_inuse_bytes",
		"go_memstats_mspan_sys_bytes",
		"go_memstats_next_gc_bytes",
		"go_memstats_other_sys_bytes",
		"go_memstats_stack_inuse_bytes",
		"go_memstats_stack_sys_bytes",
		"go_memstats_sys_bytes",
		"go_sched_gomaxprocs_threads",
		"go_sched_goroutines_goroutines",
		"go_sync_mutex_wait_total_seconds_total",
		"go_threads",
		"hidden_metrics_total",
		"kubeproxy_sync_proxy_rules_endpoint_changes_pending",
		"kubeproxy_sync_proxy_rules_endpoint_changes_total",
		"kubeproxy_sync_proxy_rules_iptables_last",
		"kubeproxy_sync_proxy_rules_iptables_partial_restore_failures_total",
		"kubeproxy_sync_proxy_rules_iptables_restore_failures_total",
		"kubeproxy_sync_proxy_rules_iptables_total",
		"kubeproxy_sync_proxy_rules_last_queued_timestamp_seconds",
		"kubeproxy_sync_proxy_rules_last_timestamp_seconds",
		"kubeproxy_sync_proxy_rules_no_local_endpoints_total",
		"kubeproxy_sync_proxy_rules_service_changes_pending",
		"kubeproxy_sync_proxy_rules_service_changes_total",
		"kubernetes_build_info",
		"kubernetes_feature_enabled",
		"process_cpu_seconds_total",
		"process_max_fds",
		"process_open_fds",
		"process_resident_memory_bytes",
		"process_start_time_seconds",
		"process_virtual_memory_bytes",
		"process_virtual_memory_max_bytes",
		"registered_metrics_total",
		"rest_client_exec_plugin_ttl_seconds",
		"rest_client_requests_total",
		"rest_client_transport_cache_entries",
		"rest_client_transport_create_calls_total",
		"scrape_duration_seconds",
		"scrape_samples_post_metric_relabeling",
		"scrape_samples_scraped",
		"scrape_series_added",
		"up",
	}

	err = pmetrictest.CompareMetrics(expectedKubeProxyMetrics, *kubeProxyMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("server.address", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("net.host.port", metricNames...),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	)
	assert.NoError(t, err)

	err = pmetrictest.CompareMetrics(expectedApiMetrics, *apiMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("server.address", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("net.host.port", metricNames...),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	)
	assert.NoError(t, err)

	err = pmetrictest.CompareMetrics(expectedControllerManagerMetrics, *controllerManagerMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("server.address", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("net.host.port", metricNames...),
	)
	assert.NoError(t, err)
}
