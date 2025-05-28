// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package configurationswitching

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	testDir   = "testdata"
	valuesDir = "values"
)

var globalSinks *sinks

type sinks struct {
	logsConsumer        *consumertest.LogsSink
	hecMetricsConsumer  *consumertest.MetricsSink
	logsObjectsConsumer *consumertest.LogsSink
}

func deployChartsAndApps(t *testing.T, valuesFileName string, repl map[string]any) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}
	replacements := map[string]any{
		"LogHecEndpoint":    fmt.Sprintf("http://%s:%d", hostEp, internal.HECLogsReceiverPort),
		"MetricHecEndpoint": fmt.Sprintf("http://%s:%d/services/collector", hostEp, internal.HECMetricsReceiverPort),
	}
	for k, v := range repl {
		replacements[k] = v
	}

	valuesFile, err := filepath.Abs(filepath.Join(testDir, valuesDir, valuesFileName))
	require.NoError(t, err)
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 0)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		t.Log("Cleaning up cluster")
		teardown(t)
	})
}

func teardown(t *testing.T) {
	t.Log("Running teardown")
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	internal.ChartUninstall(t, testKubeConfig)
}

func Test_Functions(t *testing.T) {
	globalSinks = &sinks{
		logsConsumer:        internal.SetupHECLogsSink(t),
		hecMetricsConsumer:  internal.SetupHECMetricsSink(t),
		logsObjectsConsumer: internal.SetupHECObjectsSink(t),
	}
	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	t.Run("agent logs and metrics enabled or disabled", testAgentLogsAndMetrics)
	t.Run("logs and metrics index switch", testIndexSwitch)
	t.Run("cluster receiver enabled or disabled", testClusterReceiverEnabledOrDisabled)
	t.Run("logs and metrics attributes verification", testVerifyLogsAndMetricsAttributes)
}

func testAgentLogsAndMetrics(t *testing.T) {
	valuesFileName := "values_logs_and_metrics_switching.yaml.tmpl"
	hecMetricsConsumer := globalSinks.hecMetricsConsumer
	agentLogsConsumer := globalSinks.logsConsumer

	t.Run("check logs and metrics received when both are enabled", func(t *testing.T) {
		internal.ResetLogsSink(t, agentLogsConsumer)
		internal.ResetMetricsSink(t, hecMetricsConsumer)

		internal.CheckNoMetricsReceived(t, hecMetricsConsumer)
		internal.CheckNoEventsReceived(t, agentLogsConsumer)

		replacements := map[string]any{
			"MetricsEnabled": true,
			"LogsEnabled":    true,
		}
		deployChartsAndApps(t, valuesFileName, replacements)

		internal.WaitForMetrics(t, 5, hecMetricsConsumer)
		internal.WaitForLogs(t, 5, agentLogsConsumer)
	})

	t.Run("check metrics only enabled", func(t *testing.T) {
		internal.ResetLogsSink(t, agentLogsConsumer)
		internal.ResetMetricsSink(t, hecMetricsConsumer)

		internal.CheckNoMetricsReceived(t, hecMetricsConsumer)
		internal.CheckNoEventsReceived(t, agentLogsConsumer)

		replacements := map[string]any{
			"MetricsEnabled": true,
			"LogsEnabled":    false,
		}
		deployChartsAndApps(t, valuesFileName, replacements)

		internal.WaitForMetrics(t, 5, hecMetricsConsumer)
		internal.CheckNoEventsReceived(t, agentLogsConsumer)
	})

	t.Run("check logs only enabled", func(t *testing.T) {
		internal.ResetLogsSink(t, agentLogsConsumer)
		internal.ResetMetricsSink(t, hecMetricsConsumer)

		replacements := map[string]any{
			"MetricsEnabled": false,
			"LogsEnabled":    true,
		}
		deployChartsAndApps(t, valuesFileName, replacements)

		internal.WaitForLogs(t, 5, agentLogsConsumer)
	})
}

func testIndexSwitch(t *testing.T) {
	metricsIndex := "metricsIndex"
	newMetricsIndex := "newMetricsIndex"
	logsIndex := "main"
	newLogsIndex := "newLogsIndex"
	nonDefaultSourcetype := "my-sourcetype"

	valuesFileName := "values_indexes_switching.yaml.tmpl"
	hecMetricsConsumer := globalSinks.hecMetricsConsumer
	internal.ResetMetricsSink(t, hecMetricsConsumer)
	internal.CheckNoMetricsReceived(t, hecMetricsConsumer)
	agentLogsConsumer := globalSinks.logsConsumer
	internal.ResetLogsSink(t, agentLogsConsumer)
	internal.CheckNoEventsReceived(t, agentLogsConsumer)

	t.Run("default_source_type", func(t *testing.T) {
		replacements := map[string]any{
			"MetricsIndex": metricsIndex,
			"LogsIndex":    logsIndex,
		}
		deployChartsAndApps(t, valuesFileName, replacements)

		internal.WaitForMetrics(t, 3, hecMetricsConsumer)
		internal.WaitForLogs(t, 3, agentLogsConsumer)

		var sourcetypes []string
		var indices []string
		logs := agentLogsConsumer.AllLogs()
		sourcetypes, indices = getLogsIndexAndSourceType(logs)
		assert.Greater(t, len(sourcetypes), 1) // we are receiving logs from different containers
		// check sourcetypes have same prefix
		prefix := "kube:container:"
		for _, element := range sourcetypes {
			if !strings.HasPrefix(element, prefix) {
				t.Errorf("Element does not start with the prefix %q: %s", prefix, element)
			}
		}
		assert.NotContains(t, sourcetypes, nonDefaultSourcetype)
		assert.Len(t, indices, 1)
		assert.Equal(t, logsIndex, indices[0])

		mIndices := getMetricsIndex(hecMetricsConsumer.AllMetrics())
		assert.Len(t, mIndices, 1)
		assert.Equal(t, metricsIndex, mIndices[0])
	})

	t.Run("non_default_source_type", func(t *testing.T) {
		replacements := map[string]any{
			"MetricsIndex":         newMetricsIndex,
			"LogsIndex":            newLogsIndex,
			"NonDefaultSourcetype": true,
			"Sourcetype":           nonDefaultSourcetype,
		}
		deployChartsAndApps(t, valuesFileName, replacements)
		internal.ResetMetricsSink(t, hecMetricsConsumer)
		internal.ResetLogsSink(t, agentLogsConsumer)

		internal.WaitForLogs(t, 3, agentLogsConsumer)
		logs := agentLogsConsumer.AllLogs()
		sourcetypes, indices := getLogsIndexAndSourceType(logs)
		t.Logf("Indices: %v", indices)
		assert.Contains(t, indices, newLogsIndex)
		assert.Contains(t, sourcetypes, nonDefaultSourcetype)

		internal.WaitForMetrics(t, 3, hecMetricsConsumer)
		mIndices := getMetricsIndex(hecMetricsConsumer.AllMetrics())
		assert.Contains(t, mIndices, newMetricsIndex)
	})
}

func testClusterReceiverEnabledOrDisabled(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	config, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	valuesFileName := "values_cluster_receiver_switching.yaml.tmpl"
	logsObjectsConsumer := globalSinks.logsObjectsConsumer
	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}
	logsObjectsHecEndpoint := fmt.Sprintf("http://%s:%d/services/collector", hostEp, internal.HECObjectsReceiverPort)

	t.Run("check cluster receiver disabled", func(t *testing.T) {
		replacements := map[string]any{
			"ClusterReceiverEnabled": false,
			"LogObjectsHecEndpoint":  logsObjectsHecEndpoint,
		}
		deployChartsAndApps(t, valuesFileName, replacements)
		internal.WaitForTerminatingPods(t, clientset, internal.Namespace)
		internal.ResetLogsSink(t, logsObjectsConsumer)
		pods := listPodsInNamespace(t, internal.Namespace)
		assert.Len(t, pods.Items, 1)
		assert.True(t, strings.HasPrefix(pods.Items[0].Name, "sock-splunk-otel-collector-agent"))
		internal.CheckNoEventsReceived(t, logsObjectsConsumer)
	})

	t.Run("check cluster receiver enabled", func(t *testing.T) {
		replacements := map[string]any{
			"ClusterReceiverEnabled": true,
			"LogObjectsHecEndpoint":  logsObjectsHecEndpoint,
		}
		deployChartsAndApps(t, valuesFileName, replacements)
		internal.WaitForTerminatingPods(t, clientset, internal.Namespace)
		internal.ResetLogsSink(t, logsObjectsConsumer)
		pods := listPodsInNamespace(t, internal.Namespace)
		assert.Len(t, pods.Items, 2)
		assert.True(t, checkPodExists(pods, "sock-splunk-otel-collector-agent"))
		assert.True(t, checkPodExists(pods, "sock-splunk-otel-collector-k8s-cluster-receiver"))
		internal.WaitForLogs(t, 5, logsObjectsConsumer)
	})
}

func testVerifyLogsAndMetricsAttributes(t *testing.T) {
	attributesList := [4]string{"k8s.node.name", "k8s.pod.name", "k8s.pod.uid", "k8s.namespace.name"}

	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}

	t.Run("verify cluster receiver attributes", func(t *testing.T) {
		valuesFileName := "values_cluster_receiver_only.yaml.tmpl"
		logsObjectsConsumer := globalSinks.logsObjectsConsumer
		logsObjectsHecEndpoint := fmt.Sprintf("http://%s:%d/services/collector", hostEp, internal.HECObjectsReceiverPort)

		replacements := map[string]any{
			"ClusterReceiverEnabled": true,
			"LogObjectsHecEndpoint":  logsObjectsHecEndpoint,
		}
		deployChartsAndApps(t, valuesFileName, replacements)
		internal.ResetLogsSink(t, logsObjectsConsumer)
		internal.WaitForLogs(t, 5, logsObjectsConsumer)
		t.Logf("===> >>>> Logs: %v", len(logsObjectsConsumer.AllLogs()))

		for _, attr := range attributesList {
			t.Log("Checking attribute: ", attr)
			attrValues, notFoundCounter := getLogsAttributes(logsObjectsConsumer.AllLogs(), attr)
			assert.GreaterOrEqual(t, len(attrValues), 1)
			assert.Equal(t, 0, notFoundCounter)
			t.Logf("Attributes: %v", attrValues)
		}
	})

	t.Run("verify cluster receiver metrics attributes", func(t *testing.T) {
		valuesFileName := "values_cluster_receiver_only.yaml.tmpl"
		hecMetricsConsumer := globalSinks.hecMetricsConsumer
		logsObjectsHecEndpoint := fmt.Sprintf("http://%s:%d/services/collector", hostEp, internal.HECObjectsReceiverPort)

		replacements := map[string]any{
			"ClusterReceiverEnabled": true,
			"LogObjectsHecEndpoint":  logsObjectsHecEndpoint,
		}
		deployChartsAndApps(t, valuesFileName, replacements)
		internal.ResetMetricsSink(t, hecMetricsConsumer)
		t.Logf("===> >>>> Metrics: %d", len(hecMetricsConsumer.AllMetrics()))

		internal.WaitForMetrics(t, 5, hecMetricsConsumer)
		for _, attr := range attributesList {
			t.Log("Checking attributes: ", attr)
			attrValues, notFoundCounter := getMetricsAttributes(hecMetricsConsumer.AllMetrics(), attr)
			assert.GreaterOrEqual(t, len(attrValues), 1)
			assert.Equal(t, 0, notFoundCounter)
			t.Logf("Resource Attributes for %s: %v", attr, attrValues)
		}
	})

	t.Run("verify agent logs attributes", func(t *testing.T) {
		valuesFileName := "values_logs_and_metrics_switching.yaml.tmpl"
		agentLogsConsumer := globalSinks.logsConsumer

		replacements := map[string]any{
			"MetricsEnabled": true,
			"LogsEnabled":    true,
		}
		deployChartsAndApps(t, valuesFileName, replacements)
		internal.ResetLogsSink(t, agentLogsConsumer)

		internal.WaitForLogs(t, 5, agentLogsConsumer)
		for _, attr := range attributesList {
			t.Log("Checking attribute: ", attr)
			attrValues, notFoundCounter := getLogsAttributes(agentLogsConsumer.AllLogs(), attr)
			assert.GreaterOrEqual(t, len(attrValues), 1)
			assert.Equal(t, 0, notFoundCounter)
			t.Logf("Attributes: %v", attrValues)
		}
	})

	t.Run("verify metrics attributes", func(t *testing.T) {
		valuesFileName := "values_logs_and_metrics_switching.yaml.tmpl"
		hecMetricsConsumer := globalSinks.hecMetricsConsumer

		replacements := map[string]any{
			"MetricsEnabled": true,
			"LogsEnabled":    true,
		}
		deployChartsAndApps(t, valuesFileName, replacements)
		internal.ResetMetricsSink(t, hecMetricsConsumer)

		internal.WaitForMetrics(t, 5, hecMetricsConsumer)
		for _, attr := range attributesList {
			t.Log("Checking attribute: ", attr)
			attrValues, notFoundCounter := getMetricsAttributes(hecMetricsConsumer.AllMetrics(), attr)
			assert.GreaterOrEqual(t, len(attrValues), 1)
			assert.Equal(t, 0, notFoundCounter)
			t.Logf("Attributes for %s: %v", attr, attrValues)
		}
	})
}

func checkPodExists(pods *corev1.PodList, podNamePrefix string) bool {
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, podNamePrefix) {
			return true
		}
	}
	return false
}

func listPodsInNamespace(t *testing.T, namespace string) *corev1.PodList {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	// Get the list of pods in the specified namespace
	pods, err := client.CoreV1().Pods(namespace).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	t.Logf("There are %d pods in the namespace %q\n", len(pods.Items), namespace)
	return pods
}

func getLogsIndexAndSourceType(logs []plog.Logs) ([]string, []string) {
	var sourcetypes []string
	var indices []string

	for i := 0; i < len(logs); i++ {
		l := logs[i]
		for j := 0; j < l.ResourceLogs().Len(); j++ {
			rl := l.ResourceLogs().At(j)
			if value, ok := rl.Resource().Attributes().Get("com.splunk.sourcetype"); ok {
				sourcetype := value.AsString()
				// check if sourcetype is already in the list
				if !contains(sourcetypes, sourcetype) {
					sourcetypes = append(sourcetypes, sourcetype)
				}
			}
			if value, ok := rl.Resource().Attributes().Get("com.splunk.index"); ok {
				index := value.AsString()
				// check if index is already in the list
				if !contains(indices, index) {
					indices = append(indices, index)
				}
			}
		}
	}
	return sourcetypes, indices
}

// get metrics index from metrics
func getMetricsIndex(metrics []pmetric.Metrics) []string {
	var indices []string
	for i := 0; i < len(metrics); i++ {
		m := metrics[i]
		if value, ok := m.ResourceMetrics().At(0).Resource().Attributes().Get("com.splunk.index"); ok {
			index := value.AsString()
			if !contains(indices, index) {
				indices = append(indices, index)
			}
		}
	}
	return indices
}

func contains(list []string, newValue string) bool {
	for _, v := range list {
		if v == newValue {
			return true
		}
	}
	return false
}

func getLogsAttributes(logs []plog.Logs, attributeName string) ([]string, int) {
	var attributes []string
	notFoundCounter := 0
	foundCounter := 0

	for i := 0; i < len(logs); i++ {
		l := logs[i]
		for j := 0; j < l.ResourceLogs().Len(); j++ {
			rl := l.ResourceLogs().At(j)
			for k := 0; k < rl.ScopeLogs().Len(); k++ {
				sl := rl.ScopeLogs().At(k)
				for m := 0; m < sl.LogRecords().Len(); m++ {
					tmpAttribute, ok := sl.LogRecords().At(m).Attributes().Get(attributeName)
					if ok {
						if !contains(attributes, tmpAttribute.AsString()) {
							attributes = append(attributes, tmpAttribute.AsString())
						}
						foundCounter++
					} else {
						fmt.Println("=== Attribute not found: ", attributeName)
						fmt.Printf("Log Record Body: %v\n", sl.LogRecords().At(m).Body().AsRaw())
						notFoundCounter++
					}
				}
			}
		}
	}
	fmt.Printf("Counters: Found: %d | Not Found: %d\n", foundCounter, notFoundCounter)
	return attributes, notFoundCounter
}

func getMetricsAttributes(metrics []pmetric.Metrics, attributeName string) ([]string, int) {
	var resourceAttributes []string
	notFoundCounter := 0
	foundCounter := 0
	skippedCounter := 0
	prefixesForMetricsToSkip := []string{
		// agent metrics
		"system.", "k8s.node.",
		// cluster receiver metrics
		"k8s.deployment.", "k8s.namespace.", "k8s.replicaset.", "k8s.daemonset.",
	}

	for i := 0; i < len(metrics); i++ {
		m := metrics[i]
		for j := 0; j < m.ResourceMetrics().Len(); j++ {
			rm := m.ResourceMetrics().At(j)
			for k := 0; k < rm.ScopeMetrics().Len(); k++ {
				sm := rm.ScopeMetrics().At(k)
				for l := 0; l < sm.Metrics().Len(); l++ {
					skip := false
					for _, prefix := range prefixesForMetricsToSkip {
						if strings.HasPrefix(sm.Metrics().At(l).Name(), prefix) {
							skip = true
							break
						}
					}
					if skip {
						skippedCounter++
						continue
					}
					for m := 0; m < sm.Metrics().At(l).Gauge().DataPoints().Len(); m++ {
						tmpAttribute, ok := sm.Metrics().At(l).Gauge().DataPoints().At(m).Attributes().Get(attributeName)

						if ok {
							if !contains(resourceAttributes, tmpAttribute.AsString()) {
								resourceAttributes = append(resourceAttributes, tmpAttribute.AsString())
							}
							foundCounter++
						} else {
							fmt.Printf("Resource Attribute %s not found for metric: %v \n", attributeName, sm.Metrics().At(l).Name())
							notFoundCounter++
						}
					}
				}
			}
		}
	}
	fmt.Printf("Counters: Found: %d | Skipped: %d | Not Found: %d\n", foundCounter, skippedCounter, notFoundCounter)
	return resourceAttributes, notFoundCounter
}
