// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

var globalSinks *sinks

type sinks struct {
	logsConsumer                      *consumertest.LogsSink
	hecMetricsConsumer                *consumertest.MetricsSink
	logsObjectsConsumer               *consumertest.LogsSink
	agentMetricsConsumer              *consumertest.MetricsSink
	k8sclusterReceiverMetricsConsumer *consumertest.MetricsSink
	tracesConsumer                    *consumertest.TracesSink
}

func setupSinks(t *testing.T) {
	// create an API server
	internal.SetupSignalFxAPIServer(t)
	globalSinks = &sinks{
		logsConsumer:         internal.SetupHECLogsSink(t),
		hecMetricsConsumer:   internal.SetupHECMetricsSink(t),
		logsObjectsConsumer:  internal.SetupHECObjectsSink(t),
		agentMetricsConsumer: internal.SetupSignalfxReceiver(t, internal.SignalFxReceiverPort),
		k8sclusterReceiverMetricsConsumer: internal.SetupSignalfxReceiver(t,
			signalFxReceiverK8sClusterReceiverPort),
		tracesConsumer: internal.SetupOTLPTracesSink(t),
	}
}

func testLocalClusterComponentHealth(t *testing.T) {
	// Test component health - verify no RBAC or connection errors
	testKubeConfig := requireEnv(t, "KUBECONFIG")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	t.Run("component error logs checks", func(t *testing.T) {
		internal.CheckComponentHealth(t, client, internal.DefaultNamespace, internal.AgentLabelSelector, kubeletstatsReceiverName, 500)
		internal.CheckComponentHealth(t, client, internal.DefaultNamespace, clusterReceiverLabelSelector, k8sClusterReceiverName, 500)
	})

	t.Run("control plane receiver checks", func(t *testing.T) {
		runControlPlaneReceiverChecks(t, client, controlPlaneReceiversForEnv(kindTestKubeEnv))
	})
}

// runLocalClusterTests validates telemetry sent from kind to the runner's sinks.
func runLocalClusterTests(t *testing.T, includeK8sClusterReceiverTest bool) {
	t.Run("node.js traces captured", testNodeJSTraces)
	t.Run("java traces captured", testJavaTraces)
	t.Run(".NET traces captured", testDotNetTraces)
	t.Run("Python traces captured", testPythonTraces)
	t.Run("java metrics captured", testJavaMetrics)
	t.Run("node.js metrics captured", testNodeJSMetrics)
	t.Run(".NET metrics captured", testDotNetMetrics)
	t.Run("Python metrics captured", testPythonMetrics)
	t.Run("java profiling captured", testJavaProfiling)
	t.Run("node.js profiling captured", testNodeJSProfiling)
	t.Run(".NET profiling captured", testDotNetProfiling)
	t.Run("Python profiling captured", testPythonProfiling)
	if includeK8sClusterReceiverTest {
		t.Run("kubernetes cluster metrics", testK8sClusterReceiverMetrics)
	}
	t.Run("agent logs", testAgentLogs)
	t.Run("container log attributes validation", func(t *testing.T) {
		validateLogAttributes(t, globalSinks.logsConsumer)
	})
	t.Run("test HEC metrics", testHECMetrics)
	t.Run("test k8s objects", testK8sObjects)
	t.Run("test agent metrics", testAgentMetrics)
	t.Run("test target allocator", testTargetAllocator)
	t.Run("test prometheus metrics", testPrometheusAnnotationMetrics)
	t.Run("test component health", testLocalClusterComponentHealth)
}

func testK8sClusterReceiverMetrics(t *testing.T) {
	assertionFile := filepath.Join(testDir, expectedValuesDir, "expected_cluster_receiver_assertion.yaml")
	internal.AssertMetricsSnapshot(t, globalSinks.k8sclusterReceiverMetricsConsumer,
		"k8s.persistentvolume.status.phase", assertionFile, 3*time.Minute, 10*time.Second,
		internal.WithWaitForSnapshotMatch(),
		internal.WithVolatileAttributes("k8s.container.status.last_terminated_reason"),
		internal.WithRegexAttributes(internal.CommonK8sMetricAssertionRegexAttrs),
	)
}

func testAgentLogs(t *testing.T) {
	logsConsumer := globalSinks.logsConsumer
	internal.WaitForLogs(t, 5, logsConsumer)

	var helloWorldResource pcommon.Resource
	var helloWorldLogRecord *plog.LogRecord
	excludePods := true
	excludeNs := true
	var sourcetypes []string
	var indices []string
	var journalDsourceTypes []string

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		for i := 0; i < len(logsConsumer.AllLogs()); i++ {
			l := logsConsumer.AllLogs()[i]
			for j := 0; j < l.ResourceLogs().Len(); j++ {
				rl := l.ResourceLogs().At(j)
				if value, ok := rl.Resource().Attributes().Get("com.splunk.source"); ok && value.AsString() == "/run/log/journal" {
					if value, ok = rl.Resource().Attributes().Get("com.splunk.sourcetype"); ok {
						sourcetype := value.AsString()
						journalDsourceTypes = append(journalDsourceTypes, sourcetype)
					}
				}
				if value, ok := rl.Resource().Attributes().Get("com.splunk.sourcetype"); ok {
					sourcetype := value.AsString()
					sourcetypes = append(sourcetypes, sourcetype)
				}
				if value, ok := rl.Resource().Attributes().Get("com.splunk.index"); ok {
					index := value.AsString()
					indices = append(indices, index)
				}
				if value, ok := rl.Resource().Attributes().Get("k8s.container.name"); ok {
					if value.AsString() == "pod-w-index-w-ns-exclude" {
						excludePods = false
					}
					if value.AsString() == "pod-w-exclude-wo-ns-exclude" {
						excludeNs = false
					}
				}

				for k := 0; k < rl.ScopeLogs().Len(); k++ {
					sl := rl.ScopeLogs().At(k)
					for m := 0; m < sl.LogRecords().Len(); m++ {
						logRecord := sl.LogRecords().At(m)
						if logRecord.Body().AsString() == "Hello World" {
							helloWorldLogRecord = &logRecord
							helloWorldResource = rl.Resource()
						}
					}
				}
			}
		}
		assert.NotNil(tt, helloWorldLogRecord)
		assert.Contains(tt, sourcetypes, "kube:container:pod-w-index-wo-ns-index")
		assert.Contains(tt, sourcetypes, "kube:container:pod-wo-index-w-ns-index")
		assert.Contains(tt, sourcetypes, "kube:container:pod-wo-index-wo-ns-index")
		assert.Contains(tt, sourcetypes, "sourcetype-anno") // pod-wo-index-w-ns-index has a sourcetype annotation
	}, 3*time.Minute, 5*time.Second)

	if strings.HasPrefix(os.Getenv("K8S_VERSION"), "v1.30") {
		t.Log("Skipping test for journald sourcetypes for cluster version 1.30")
	} else {
		t.Run("test journald sourcetypes are set", func(t *testing.T) {
			assert.Contains(t, journalDsourceTypes, "kube:journald:containerd.service")
			assert.Contains(t, journalDsourceTypes, "kube:journald:kubelet.service")
		})
	}
	t.Run("test node.js log records", func(t *testing.T) {
		assert.NotNil(t, helloWorldLogRecord)
		sourceType, ok := helloWorldResource.Attributes().Get("com.splunk.sourcetype")
		assert.True(t, ok)
		assert.Equal(t, "kube:container:nodejs-test", sourceType.AsString())
		source, ok := helloWorldResource.Attributes().Get("com.splunk.source")
		assert.True(t, ok)
		assert.Regexp(t, "/var/log/pods/default_nodejs-test-.*/nodejs-test/0.log", source.AsString())
		index, ok := helloWorldResource.Attributes().Get("com.splunk.index")
		assert.True(t, ok)
		assert.Equal(t, "main", index.AsString())
		podName, ok := helloWorldLogRecord.Attributes().Get("k8s.pod.name")
		assert.True(t, ok)
		assert.Regexp(t, "nodejs-test-.*", podName.AsString())
	})
	t.Run("test index is set", func(t *testing.T) {
		assert.Contains(t, indices, "ns-anno")
		assert.Contains(t, indices, "pod-anno")
	})
	t.Run("test sourcetype is set", func(t *testing.T) {
		assert.Contains(t, sourcetypes, "kube:container:pod-w-index-wo-ns-index")
		assert.Contains(t, sourcetypes, "kube:container:pod-wo-index-w-ns-index")
		assert.Contains(t, sourcetypes, "kube:container:pod-wo-index-wo-ns-index")
		assert.Contains(t, sourcetypes, "sourcetype-anno") // pod-wo-index-w-ns-index has a sourcetype annotation
	})
	t.Run("excluded pods and namespaces", func(t *testing.T) {
		assert.True(t, excludePods, "excluded pods should be ignored")
		assert.True(t, excludeNs, "excluded namespaces should be ignored")
	})
	t.Run("check default metadata is attached to all the logs", func(t *testing.T) {
		_, ok := helloWorldLogRecord.Attributes().Get("k8s.pod.name")
		assert.True(t, ok)
		_, ok = helloWorldLogRecord.Attributes().Get("k8s.namespace.name")
		assert.True(t, ok)
		_, ok = helloWorldLogRecord.Attributes().Get("k8s.container.name")
		assert.True(t, ok)
		_, ok = helloWorldLogRecord.Attributes().Get("k8s.pod.uid")
		assert.True(t, ok)
	})
}

func testK8sObjects(t *testing.T) {
	logsObjectsConsumer := globalSinks.logsObjectsConsumer
	internal.WaitForLogs(t, 5, logsObjectsConsumer)

	var kinds []string
	var sourceTypes []string
	foundCustomField1 := false
	foundCustomField2 := false

	for i := 0; i < len(logsObjectsConsumer.AllLogs()); i++ {
		l := logsObjectsConsumer.AllLogs()[i]
		for j := 0; j < l.ResourceLogs().Len(); j++ {
			rl := l.ResourceLogs().At(j)
			for k := 0; k < rl.ScopeLogs().Len(); k++ {
				sl := rl.ScopeLogs().At(k)
				for m := 0; m < sl.LogRecords().Len(); m++ {
					logRecord := sl.LogRecords().At(m)
					if logRecord.Body().Type() == pcommon.ValueTypeMap {
						if kind, ok := logRecord.Body().Map().Get("kind"); ok {
							kinds = append(kinds, kind.AsString())
						}
					}
					if value, ok := logRecord.Attributes().Get("customfield1"); ok && value.AsString() == "customvalue1" {
						foundCustomField1 = true
					}
					if value, ok := logRecord.Attributes().Get("customfield2"); ok && value.AsString() == "customvalue2" {
						foundCustomField2 = true
					}
				}
			}
			if value, ok := rl.Resource().Attributes().Get("com.splunk.sourcetype"); ok {
				sourceTypes = append(sourceTypes, value.AsString())
			}
		}
	}

	assert.Contains(t, kinds, "Pod")
	assert.Contains(t, kinds, "Namespace")
	assert.Contains(t, kinds, "Node")

	assert.Contains(t, sourceTypes, "kube:object:pods")
	assert.Contains(t, sourceTypes, "kube:object:namespaces")
	assert.Contains(t, sourceTypes, "kube:object:nodes")

	assert.True(t, foundCustomField1)
	assert.True(t, foundCustomField2)
}

// Internal telemetry metrics are only sent when an event occurs. Due to cluster
// setup and potentially different events occurring on the cluster before the
// test runs, different internal telemetry metrics will be sent. This method
// is used to ensure some events occur that would only take place occasionally
// before. This led to test flakiness as sometimes metrics are expected but the
// event hasn't taken place, or vice versa.
func generateRequiredTelemetry(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	assert.True(t, setKubeConfig)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	testNamespaceName := "test-namespace"
	internal.CreateNamespace(t, client, testNamespaceName)
	internal.WaitForDefaultServiceAccount(t, client, testNamespaceName)

	testPodConfig := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "main",
					Image:   "python:3.11",
					Command: []string{"python"},
					Args:    []string{"-c", "print('hello world')"},
				},
			},
		},
	}
	internal.CreatePod(t, client, testPodConfig.Name, testNamespaceName, testPodConfig)
	internal.DeletePod(t, client, testPodConfig.Name, testNamespaceName)
	internal.LabelNamespace(t, client, testNamespaceName, "testLabel", "true")
	internal.DeleteNamespace(t, client, testNamespaceName)
}

func testAgentMetrics(t *testing.T) {
	generateRequiredTelemetry(t)

	t.Run("internal metrics", func(t *testing.T) {
		testAgentMetricsAssertion(t, "expected_internal_metrics_assertion.yaml",
			"otelcol_processor_memory_limiter_accepted_log_records", "", nil)
	})
	t.Run("kubelet_stats metrics", func(t *testing.T) {
		// Kubelet CPU stats are not emitted consistently across supported Kubernetes versions.
		testAgentMetricsAssertion(t, "expected_kubelet_stats_metrics_assertion.yaml",
			"container.memory.usage", kindAgentPodNamePrefix, []string{"container.cpu.usage"})
	})
	t.Run("host_metrics", func(t *testing.T) {
		testAgentMetricsAssertion(t, "expected_host_metrics_assertion.yaml",
			"system.memory.usage", "", nil)
	})
}

// These values were ignored by the previous metric comparison. Their presence
// remains required by the assertion snapshots.
var agentMetricVolatileAttrs = []string{
	"container.id", "k8s.daemonset.uid", "k8s.deployment.uid", "k8s.pod.uid",
	"k8s.pod.name", "k8s.container.name", "pod_identifier", "otelcol_signal",
	"k8s.replicaset.uid", "k8s.replicaset.name", "k8s.namespace.name",
	"k8s.namespace.uid", "k8s.node.name", "container.image.name",
	"container.image.tag", "k8s.node.uid", "net.host.name", "processor",
	"service.instance.id", "service.version", "receiver", "transport",
	"exporter", "server.address", "com.splunk.sourcetype", "device",
}

func testAgentMetricsAssertion(t *testing.T, assertionFileName, targetMetric, podNamePrefix string, flakyMetricNames []string) {
	assertionFile := filepath.Join(testDir, expectedValuesDir, assertionFileName)
	options := []internal.MetricsAssertionOption{
		internal.WithWaitForSnapshotMatch(),
		internal.WithVolatileAttributes(agentMetricVolatileAttrs...),
		internal.WithScopeVersionRegex(".*"),
	}
	if podNamePrefix != "" || len(flakyMetricNames) > 0 {
		options = append(options, internal.WithMetricsFilter(func(metrics *pmetric.Metrics) {
			internal.RemoveFlakyMetrics(metrics, flakyMetricNames)
			if podNamePrefix != "" {
				internal.RetainNumberMetricDatapointsForPod(metrics, podNamePrefix)
			}
		}))
	}
	internal.AssertMetricsSnapshot(t, globalSinks.agentMetricsConsumer,
		targetMetric, assertionFile, 3*time.Minute, 10*time.Second, options...)
}

func testHECMetrics(t *testing.T) {
	internal.AssertMetricNames(t, globalSinks.hecMetricsConsumer,
		filepath.Join(testDir, "expected_hec_metric_names_assertion.yaml"), 3*time.Minute, 10*time.Second)
}
