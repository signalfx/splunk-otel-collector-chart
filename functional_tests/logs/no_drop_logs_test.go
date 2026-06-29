// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	testFile                   = "test.log"
	testDir                    = "testdata"
	remoteTestFile             = "/tmp/temp-log-test/test.log"
	valuesTemplateFile         = "no_drop_logs_values.yaml.tmpl"
	dropLogsValuesTemplateFile = "drop_logs_values.yaml.tmpl"
	testLogLineCount           = 600
)

var podName string

// Env vars to control the test behavior
// TEARDOWN_BEFORE_SETUP: if set to true, the test will run teardown before setup
// SKIP_SETUP: if set to true, the test will skip setup
// SKIP_TEARDOWN: if set to true, the test will skip teardown
// SKIP_TESTS: if set to true, the test will skip the test
// UPDATE_EXPECTED_RESULTS: if set to true, the test will update the expected results
// KUBECONFIG: the path to the kubeconfig file
func Test_NoDropLogs(t *testing.T) {
	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		t.Log("Running teardown before setup as TEARDOWN_BEFORE_SETUP is set to true")
		teardown(t)
	}

	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	config, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	// NoDropLogs: with noDropLogsPipeline feature gate enabled, block_on_overflow=true applies backpressure
	// to the filelog receiver when the queue is full. No records are dropped — all 600 lines must arrive.
	t.Run("NoDropLogs", func(t *testing.T) {
		if os.Getenv("SKIP_SETUP") != "true" {
			deployChart(t, testKubeConfig, clientset, valuesTemplateFile)
			deployTestLogToPod(t, clientset, config)
		}
		// wait to ensure the queue fills and backpressure is applied before HEC starts
		time.Sleep(10 * time.Second)
		logsConsumer := internal.SetupHECLogsSink(t)

		// test log file contains 600 log lines, min_size=100 so expect 6 batches
		internal.WaitForLogs(t, 6, logsConsumer)
		podLogs, podLogsErr := internal.GetPodLogs(t, clientset, internal.DefaultNamespace, podName, internal.CollectorContainerName, 100)
		require.NoError(t, podLogsErr, "failed to get logs for pod: %s", podName)
		require.NotContains(t, podLogs, "Exporting failed. Rejecting data.", "drop log message not found — records shouldn't be dropped with noDropLogsPipeline feature gate")
		require.Equal(t, testLogLineCount, logsConsumer.LogRecordCount(), "expected number of log records does not match what received")
		if os.Getenv("SKIP_TEARDOWN") != "true" {
			teardown(t)
		}
	})

	// DropLogs: without noDropLogsPipeline feature gate, queue fills and records are dropped.
	// HEC is started after a delay so the queue fills while HEC is unavailable, triggering drops.
	t.Run("DropLogsWithoutFeatureGate", func(t *testing.T) {
		if os.Getenv("SKIP_SETUP") != "true" {
			teardown(t)
			deployChart(t, testKubeConfig, clientset, dropLogsValuesTemplateFile)
			deployTestLogToPod(t, clientset, config)
		}
		time.Sleep(10 * time.Second)

		logsConsumer := internal.SetupHECLogsSink(t)
		internal.WaitForLogs(t, 1, logsConsumer)

		podLogs, podLogsErr := internal.GetPodLogs(t, clientset, internal.DefaultNamespace, podName, internal.CollectorContainerName, 100)
		require.NoError(t, podLogsErr, "failed to get logs for pod: %s", podName)
		require.Contains(t, podLogs, "Exporting failed. Rejecting data.", "expected drop log message not found — records should be dropped without noDropLogsPipeline feature gate")
		if os.Getenv("SKIP_TEARDOWN") != "true" {
			teardown(t)
		}
	})
}

func deployChart(t *testing.T, testKubeConfig string, clientset *kubernetes.Clientset, templateFile string) {
	valuesFile, err := filepath.Abs(filepath.Join(testDir, templateFile))
	require.NoError(t, err)
	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "host endpoint not found")
	}
	replacements := map[string]any{
		"LogURL": internal.HostPortHTTP(hostEp, internal.HECLogsReceiverPort),
	}
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 0, internal.GetDefaultChartOptions())

	internal.CheckPodsReady(t, clientset, internal.DefaultNamespace, internal.AgentLabelSelector, 3*time.Minute, 5*time.Second)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		teardown(t)
	})
}

func deployTestLogToPod(t *testing.T, clientset *kubernetes.Clientset, config *rest.Config) {
	// log file is copied to only one randomly selected pod, and we need to remember the pod name for later log retrieval
	pods, err := internal.GetPods(t, clientset, internal.DefaultNamespace, internal.AgentLabelSelector)
	require.NoError(t, err)
	if len(pods.Items) == 0 {
		require.Failf(t, "no pods found for label %s", internal.AgentLabelSelector)
	}
	podName = pods.Items[0].Name
	testLogFile, err := filepath.Abs(filepath.Join(testDir, testFile))
	require.NoError(t, err)
	internal.CopyFileToPod(t, clientset, config, internal.DefaultNamespace, podName, internal.CollectorContainerName, testLogFile, remoteTestFile)
}

func teardown(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	internal.ChartUninstall(t, testKubeConfig)
}
