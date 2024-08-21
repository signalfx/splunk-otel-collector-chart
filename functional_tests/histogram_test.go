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

	expectedEtcdMetrics, err := golden.ReadMetrics(filepath.Join(testDir, "etcd_metrics.yaml"))
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
						if metricToConsider.Name() == "coredns_dns_request_duration_seconds" { //&& m.MetricCount() == expectedCoreDNSMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedCoreDNSMetrics.ResourceMetrics().Len() {
							corednsMetrics = &m
							break OUTER
						} else if metricToConsider.Name() == "kubeproxy_sync_proxy_rules_iptables_total" { //&& m.MetricCount() == expectedKubeProxyMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedKubeProxyMetrics.ResourceMetrics().Len() {
							kubeProxyMetrics = &m
							break OUTER
						} else if metricToConsider.Name() == "scheduler_scheduling_attempt_duration_seconds" { //&& m.MetricCount() == expectedKubeSchedulerMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedKubeSchedulerMetrics.ResourceMetrics().Len() {
							schedulerMetrics = &m
							break OUTER
						} else if metricToConsider.Name() == "apiserver_audit_event_total" { //&& m.MetricCount() == expectedApiMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedApiMetrics.ResourceMetrics().Len() {
							apiMetrics = &m
							break OUTER
						} else if metricToConsider.Name() == "workqueue_queue_duration_seconds" { //&& m.MetricCount() == expectedControllerManagerMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedControllerManagerMetrics.ResourceMetrics().Len() {
							controllerManagerMetrics = &m
							break OUTER
						} else if metricToConsider.Name() == "etcd_cluster_version" { //&& m.MetricCount() == expectedEtcdMetrics.MetricCount() && m.ResourceMetrics().Len() == expectedEtcdMetrics.ResourceMetrics().Len() {
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

	}, 3*time.Minute, 5*time.Second)

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
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricAttributeValue("to", "coredns_forward_request_duration_seconds"),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreResourceAttributeValue("server.address"),
		pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.name"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.port"),
	)
	assert.NoError(t, err)
	if err != nil {
		require.NoError(t, os.MkdirAll("results", 0755))
		require.NoError(t, golden.WriteMetrics(t, filepath.Join("results", "coredns_metrics.yaml"), *corednsMetrics))
	}

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
	if err != nil {
		require.NoError(t, os.MkdirAll("results", 0755))
		require.NoError(t, golden.WriteMetrics(t, filepath.Join("results", "scheduler_metrics.yaml"), *schedulerMetrics))
	}

	metricNames := []string{"aggregator_discovery_aggregation_count_total",
		"apiserver_audit_event_total",
		"apiserver_audit_requests_rejected_total",
		"apiserver_envelope_encryption_dek_cache_fill_percent",
		"apiserver_storage_data_key_generation_failures_total",
		"apiserver_storage_envelope_transformation_cache_misses_total",
		"apiserver_webhooks_x509_insecure_sha1_total",
		"apiserver_webhooks_x509_missing_san_total",
		"disabled_metrics_total",
		"etcd_cluster_version",
		"etcd_debugging_auth_revision",
		"etcd_debugging_lease_granted_total",
		"etcd_debugging_lease_renewed_total",
		"etcd_debugging_lease_revoked_total",
		"etcd_debugging_mvcc_compact_revision",
		"etcd_debugging_mvcc_current_revision",
		"etcd_debugging_mvcc_db_compaction_keys_total",
		"etcd_debugging_mvcc_db_compaction_last",
		"etcd_debugging_mvcc_events_total",
		"etcd_debugging_mvcc_keys_total",
		"etcd_debugging_mvcc_pending_events_total",
		"etcd_debugging_mvcc_range_total",
		"etcd_debugging_mvcc_slow_watcher_total",
		"etcd_debugging_mvcc_total_put_size_in_bytes",
		"etcd_debugging_mvcc_watch_stream_total",
		"etcd_debugging_mvcc_watcher_total",
		"etcd_debugging_server_lease_expired_total",
		"etcd_debugging_store_expires_total",
		"etcd_debugging_store_reads_total",
		"etcd_debugging_store_watch_requests_total",
		"etcd_debugging_store_watchers",
		"etcd_debugging_store_writes_total",
		"etcd_disk_defrag_inflight",
		"etcd_disk_wal_write_bytes_total",
		"etcd_grpc_proxy_cache_hits_total",
		"etcd_grpc_proxy_cache_keys_total",
		"etcd_grpc_proxy_cache_misses_total",
		"etcd_grpc_proxy_cache_misses_total",
		"etcd_grpc_proxy_events_coalescing_total",
		"etcd_grpc_proxy_watchers_coalescing_total",
		"etcd_mvcc_db_open_read_transactions",
		"etcd_mvcc_db_total_size_in_bytes",
		"etcd_mvcc_db_total_size_in_use_in_bytes",
		"etcd_mvcc_delete_total",
		"etcd_mvcc_put_total",
		"etcd_mvcc_range_total",
		"etcd_mvcc_txn_total",
		"etcd_network_client_grpc_received_bytes_total",
		"etcd_network_client_grpc_sent_bytes_total",
		"etcd_server_client_requests_total",
		"etcd_server_go_version",
		"etcd_server_has_leader",
		"etcd_server_health_failures",
		"etcd_server_health_success",
		"etcd_server_heartbeat_send_failures_total",
		"etcd_server_id",
		"etcd_server_is_learner",
		"etcd_server_is_leader",
		"etcd_server_leader_changes_seen_total",
		"etcd_server_learner_promote_successes",
		"etcd_server_learner_promote_successes",
		"etcd_server_proposals_applied_total",
		"etcd_server_proposals_committed_total",
		"etcd_server_proposals_failed_total",
		"etcd_server_proposals_pending",
		"etcd_server_quota_backend_bytes",
		"etcd_server_read_indexes_failed_total",
		"etcd_server_slow_apply_total",
		"etcd_server_slow_read_indexes_total",
		"etcd_server_snapshot_apply_in_progress_total",
		"etcd_server_version",
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
		"go_memstats_gc_cpu_fraction",
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
		"grpc_server_handled_total",
		"grpc_server_msg_received_total",
		"grpc_server_msg_sent_total",
		"grpc_server_started_total",
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
		"promhttp_metric_handler_requests_in_flight",
		"promhttp_metric_handler_requests_total",
		"os_fd_used",
		"os_fd_limit",
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
		pmetrictest.IgnoreMetricAttributeValue("version", "go_info"),
		pmetrictest.IgnoreMetricAttributeValue("build_date", "kubernetes_build_info"),
		pmetrictest.IgnoreMetricAttributeValue("git_commit", "kubernetes_build_info"),
		pmetrictest.IgnoreMetricAttributeValue("git_version", "kubernetes_build_info"),
		pmetrictest.IgnoreMetricAttributeValue("go_version", "kubernetes_build_info"),
		pmetrictest.IgnoreMetricAttributeValue("minor", "kubernetes_build_info"),
		pmetrictest.IgnoreMetricAttributeValue("platform", "kubernetes_build_info"),
		pmetrictest.IgnoreMetricAttributeValue("server_go_version", "etcd_server_go_version"),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	)
	assert.NoError(t, err)
	if err != nil {
		require.NoError(t, os.MkdirAll("results", 0755))
		require.NoError(t, golden.WriteMetrics(t, filepath.Join("results", "proxy_metrics.yaml"), *kubeProxyMetrics))
	}

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
	if err != nil {
		require.NoError(t, os.MkdirAll("results", 0755))
		require.NoError(t, golden.WriteMetrics(t, filepath.Join("results", "api_metrics.yaml"), *apiMetrics))
	}

	err = pmetrictest.CompareMetrics(expectedControllerManagerMetrics, *controllerManagerMetrics,
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
		pmetrictest.IgnoreResourceAttributeValue("server.address"),
		pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.name"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.port"),
		pmetrictest.IgnoreResourceAttributeValue("service.name"),
		pmetrictest.IgnoreResourceAttributeValue("server.port"),
	)
	assert.NoError(t, err)
	if err != nil {
		require.NoError(t, os.MkdirAll("results", 0755))
		require.NoError(t, golden.WriteMetrics(t, filepath.Join("results", "controller_metrics.yaml"), *controllerManagerMetrics))
	}

	err = pmetrictest.CompareMetrics(expectedEtcdMetrics, *etcdMetrics,
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
		pmetrictest.IgnoreMetricAttributeValue("server_go_version", "etcd_server_go_version"),
		pmetrictest.IgnoreMetricAttributeValue("server_version", "etcd_server_version"),
		pmetrictest.IgnoreMetricAttributeValue("version", "go_info"),
		pmetrictest.IgnoreMetricAttributeValue("client_api_version", "etcd_server_client_requests_total"),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreResourceAttributeValue("server.address"),
		pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.name"),
		pmetrictest.IgnoreResourceAttributeValue("net.host.port"),
	)
	assert.NoError(t, err)
	if err != nil {
		require.NoError(t, os.MkdirAll("results", 0755))
		require.NoError(t, golden.WriteMetrics(t, filepath.Join("results", "etcd_metrics.yaml"), *etcdMetrics))
	}
}
