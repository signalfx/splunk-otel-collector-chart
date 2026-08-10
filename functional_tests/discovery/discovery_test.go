// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package k8sevents

import (
	"os"
	"testing"
	"time"

	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	redisDeploymentName = "test-redis"
	redisManifestDir    = "testdata/valkey"
	redisLabelSelector  = "app.kubernetes.io/instance=" + redisDeploymentName
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
	installRedisDeployment(t, testKubeConfig)
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
			internal.BasicCollectorChartInstall(t, testKubeConfig, tt.valuesTmpl)
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
	assertAttr(t, idAttrs, "service.name", "test-redis")
	entityAttrsVal, ok := lrAttrs.Get("otel.entity.attributes")
	assert.True(t, ok)
	entityAttrs := entityAttrsVal.Map()
	assertAttr(t, entityAttrs, "k8s.namespace.name", internal.DefaultNamespace)
	podName, ok := entityAttrs.Get("k8s.pod.name")
	assert.True(t, ok)
	if ok {
		assert.Regexp(t, `^test-redis-[a-z0-9]+-[a-z0-9]+$`, podName.AsString())
	}
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
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		foundMetrics := make(map[string]bool)
		for _, m := range sink.AllMetrics() {
			for i := 0; i < m.ResourceMetrics().Len(); i++ {
				rm := m.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						foundMetrics[sm.Metrics().At(k).Name()] = true
					}
				}
			}
		}

		for _, rm := range expectedRedisMetrics {
			assert.Contains(tt, foundMetrics, rm)
		}
	}, 5*time.Minute, 3*time.Second, "Missing expected redis metrics")
}

func installRedisDeployment(t *testing.T, kubeConfig string) {
	t.Helper()
	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping Redis-compatible deployment as SKIP_SETUP is set to true")
		return
	}

	k8sClient, err := k8stest.NewK8sClient(kubeConfig)
	require.NoError(t, err)
	createdObjects, err := k8stest.CreateObjects(k8sClient, redisManifestDir)
	require.NoError(t, err)
	require.NotEmpty(t, createdObjects)

	clientset, err := internal.GetKubeClient(kubeConfig)
	require.NoError(t, err)
	internal.CheckPodsReady(
		t,
		clientset,
		internal.DefaultNamespace,
		redisLabelSelector,
		2*time.Minute,
		0,
	)
}

func uninstallRedisDeployment(t *testing.T, kubeConfig string) {
	t.Helper()
	k8sClient, err := k8stest.NewK8sClient(kubeConfig)
	require.NoError(t, err)
	internal.DeleteObject(t, k8sClient, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-redis
  namespace: default
`)
}

func teardown(t *testing.T, kubeConfig string) {
	t.Helper()
	uninstallRedisDeployment(t, kubeConfig)
	internal.ChartUninstall(t, kubeConfig)
}
