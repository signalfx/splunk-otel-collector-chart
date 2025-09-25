// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package nodroplogs

import (
	"fmt"
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
	testFile           = "test.log"
	testDir            = "testdata"
	remoteTestFile     = "/tmp/temp-log-test/test.log"
	valuesTemplateFile = "no_drop_logs_values.yaml.tmpl"
	containerName      = "otel-collector"
	podLabelSelector   = "component=otel-collector-agent"
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

	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		deployChart(t, testKubeConfig, clientset)
		deployTestLogToPod(t, clientset, config)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	// 1. Log sink is unavailable, queue becomes full, export fails, and backpressure is applied to the receiver.
	// This can be verified with the log message "Exporting failed. Rejecting data."
	// 2. Log sink is available, and all logs are exported successfully.
	// This can be verified by matching the number of log records to the test log file line count
	t.Run("NoDropLogs", func(t *testing.T) {
		time.Sleep(10 * time.Second)
		podLogs := internal.GetPodLogs(t, clientset, internal.DefaultNamespace, podName, containerName, 100)
		require.Contains(t, podLogs, "Exporting failed. Rejecting data.", "expected log message not found in pod logs")

		logsConsumer := internal.SetupHECLogsSink(t)

		// entriesNum calculation logic:
		// - test log file contains 600 log lines
		// - filelog receiver batches 100 lines/request
		// - for this test, I set the sender queue size to 3 requests; ie: 3 * 100 log messages, before the queue is full.
		// entriesNum = 600 / (3 * 100) = 2
		internal.WaitForLogs(t, 2, logsConsumer)
		require.Equal(t, 600, logsConsumer.LogRecordCount(), "expected number of log records does not match what received")
	})
}

func deployChart(t *testing.T, testKubeConfig string, clientset *kubernetes.Clientset) {
	valuesFile, err := filepath.Abs(filepath.Join(testDir, valuesTemplateFile))
	require.NoError(t, err)
	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "host endpoint not found")
	}
	replacements := map[string]any{
		"LogURL": fmt.Sprintf("http://%s:%d", hostEp, internal.HECLogsReceiverPort),
	}
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 0, internal.GetDefaultChartOptions())

	internal.CheckPodsReady(t, clientset, internal.DefaultNamespace, podLabelSelector, 3*time.Minute, 5*time.Second)

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
	pods := internal.GetPods(t, clientset, internal.DefaultNamespace, podLabelSelector)
	if len(pods.Items) == 0 {
		require.Failf(t, "no pods found for label %s", podLabelSelector)
	}
	podName = pods.Items[0].Name
	testLogFile, err := filepath.Abs(filepath.Join(testDir, testFile))
	require.NoError(t, err)
	internal.CopyFileToPod(t, clientset, config, internal.DefaultNamespace, podName, containerName, testLogFile, remoteTestFile)
}

func teardown(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	internal.ChartUninstall(t, testKubeConfig)
}
