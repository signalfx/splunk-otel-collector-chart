// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package k8sevents

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	redisReleaseName = "test-redis"
	redisChartRepo   = "https://charts.bitnami.com/bitnami"
	redisChart       = "redis"
)

// Env vars to control the test behavior:
// KUBECONFIG (required): the path to the kubeconfig file
// TEARDOWN_BEFORE_SETUP: if set to true, the test will run teardown before setup
// SKIP_TEARDOWN: if set to true, the test will skip teardown
// SKIP_SETUP: if set to true, the test will skip setup
func Test_Discovery(t *testing.T) {
	testKubeConfig, ok := os.LookupEnv("KUBECONFIG")
	require.True(t, ok, "the environment variable KUBECONFIG must be set")
	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t, testKubeConfig)
	}
	installRedisChart(t, testKubeConfig)
	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		teardown(t, testKubeConfig)
	})

	internal.SetupSignalFxAPIServer(t)

	tests := []struct {
		name       string
		valuesTmpl string
	}{
		{
			name:       "agent_only",
			valuesTmpl: "agent_only_values.tmpl",
		},
		{
			name:       "agent_with_gateway",
			valuesTmpl: "agent_with_gateway_values.tmpl",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricsSink := internal.SetupSignalfxReceiver(t, internal.SignalFxReceiverPort)
			eventsSink := internal.SetupOTLPLogsSink(t)
			installCollectorChart(t, testKubeConfig, tt.valuesTmpl)
			t.Cleanup(func() {
				if os.Getenv("SKIP_TEARDOWN") == "true" {
					return
				}
				internal.ChartUninstall(t, testKubeConfig)
			})
			assertRedisEntities(t, eventsSink)
			assertRedisMetrics(t, metricsSink)
		})
	}
}

func assertRedisEntities(t *testing.T, sink *consumertest.LogsSink) {
	internal.WaitForLogs(t, 1, sink)
	rl := sink.AllLogs()[len(sink.AllLogs())-1].ResourceLogs().At(0)
	assertAttr(t, rl.Resource().Attributes(), "k8s.cluster.name", "test-cluster")
	assert.Equal(t, 1, rl.ScopeLogs().Len())
	sl := rl.ScopeLogs().At(0)
	assertAttr(t, sl.Scope().Attributes(), "otel.entity.event_as_log", true)
	assert.Equal(t, 1, sl.LogRecords().Len())
	lrAttrs := sl.LogRecords().At(0).Attributes()
	assertAttr(t, lrAttrs, "otel.entity.event.type", "entity_state")
	assertAttr(t, lrAttrs, "otel.entity.type", "service")
	idAttrsVal, ok := lrAttrs.Get("otel.entity.id")
	assert.True(t, ok)
	idAttrs := idAttrsVal.Map()
	assertAttr(t, idAttrs, "service.type", "redis")
	assertAttr(t, idAttrs, "service.name", "redis")
	entityAttrsVal, ok := lrAttrs.Get("otel.entity.attributes")
	assert.True(t, ok)
	entityAttrs := entityAttrsVal.Map()
	assertAttr(t, entityAttrs, "k8s.namespace.name", internal.DefaultNamespace)
	assertAttr(t, entityAttrs, "k8s.pod.name", "test-redis-master-0")
	assertAttr(t, entityAttrs, "discovery.status", "successful")
}

func assertAttr(t *testing.T, attrs pcommon.Map, name string, val any) {
	entityType, ok := attrs.Get(name)
	assert.True(t, ok)
	if ok {
		assert.Equal(t, val, entityType.AsRaw())
	}
}

func assertRedisMetrics(t *testing.T, sink *consumertest.MetricsSink) {
	internal.WaitForMetrics(t, 5, sink)
	foundMetrics := make(map[string]bool)
	for _, m := range sink.AllMetrics() {
		for i := 0; i < m.ResourceMetrics().Len(); i++ {
			sm := m.ResourceMetrics().At(i).ScopeMetrics().At(0)
			for j := 0; j < sm.Metrics().Len(); j++ {
				foundMetrics[sm.Metrics().At(j).Name()] = true
			}
		}
	}
	expectedRedisMetrics := []string{
		"redis.clients.blocked",
		"redis.clients.connected",
		"redis.clients.max_input_buffer",
		"redis.clients.max_output_buffer",
		"redis.commands",
		"redis.commands.processed",
		"redis.connections.received",
		"redis.connections.rejected",
		"redis.cpu.time",
		"redis.keys.evicted",
		"redis.keys.expired",
		"redis.keyspace.hits",
		"redis.keyspace.misses",
		"redis.latest_fork",
		"redis.memory.fragmentation_ratio",
		"redis.memory.lua",
		"redis.memory.peak",
		"redis.memory.rss",
		"redis.memory.used",
		"redis.net.input",
		"redis.net.output",
		"redis.rdb.changes_since_last_save",
		"redis.replication.backlog_first_byte_offset",
		"redis.replication.offset",
		"redis.slaves.connected",
		"redis.uptime",
	}
	for _, rm := range expectedRedisMetrics {
		assert.Contains(t, foundMetrics, rm)
	}
}

func installCollectorChart(t *testing.T, kubeConfig, valuesTmpl string) {
	t.Helper()
	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping collector chart installation as SKIP_SETUP is set to true")
		return
	}

	hostEp := internal.HostEndpoint(t)
	valuesFile, err := filepath.Abs(filepath.Join("testdata", valuesTmpl))
	require.NoError(t, err)
	internal.ChartInstallOrUpgrade(t, kubeConfig, valuesFile, map[string]any{
		"ApiURL":    fmt.Sprintf("http://%s:%d", hostEp, internal.SignalFxAPIPort),
		"IngestURL": fmt.Sprintf("http://%s:%d", hostEp, internal.SignalFxReceiverPort),
		"EventsURL": fmt.Sprintf("http://%s:%d", hostEp, internal.OTLPHTTPReceiverPort),
	}, 0, internal.GetDefaultChartOptions())
}

// installRedisChart deploys a simple Redis server with official helm chart.
func installRedisChart(t *testing.T, kubeConfig string) {
	t.Helper()
	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping redis chart installation as SKIP_SETUP is set to true")
		return
	}

	actionConfig := internal.InitHelmActionConfig(t, kubeConfig)
	rc, err := registry.NewClient()
	require.NoError(t, err)
	actionConfig.RegistryClient = rc
	install := action.NewInstall(actionConfig)
	install.Namespace = internal.DefaultNamespace
	install.ReleaseName = redisReleaseName
	install.RepoURL = redisChartRepo
	install.Wait = true
	install.Timeout = internal.HelmActionTimeout
	hCli := cli.New()
	hCli.KubeConfig = kubeConfig
	chartPath, err := install.LocateChart(redisChart, hCli)
	require.NoError(t, err)
	var ch *chart.Chart
	ch, err = loader.Load(chartPath)
	require.NoError(t, err)

	// Install the redis chart with no replicas and no auth
	var release *release.Release
	release, err = install.Run(ch, map[string]any{
		"auth": map[string]any{
			"enabled": false,
		},
		"replica": map[string]any{
			"replicaCount": 0,
		},
	})
	require.NoError(t, err)
	t.Logf("Helm chart installed. Release name: %s", release.Name)
}

func uninstallRedisChart(t *testing.T, kubeConfig string) {
	t.Helper()
	uninstallAction := action.NewUninstall(internal.InitHelmActionConfig(t, kubeConfig))
	uninstallAction.Wait = true
	uninstallAction.Timeout = internal.HelmActionTimeout
	uninstallAction.IgnoreNotFound = true
	_, err := uninstallAction.Run(redisReleaseName)
	require.NoError(t, err)
	t.Logf("Helm release %q uninstalled", redisReleaseName)
}

func teardown(t *testing.T, kubeConfig string) {
	t.Helper()
	uninstallRedisChart(t, kubeConfig)
	internal.ChartUninstall(t, kubeConfig)
}
