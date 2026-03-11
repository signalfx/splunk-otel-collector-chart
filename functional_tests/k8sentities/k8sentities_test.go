// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

// Package k8sentities contains a black-box functional test for the experimental
// featureGates.enableK8sEntities feature gate.
//
// When enabled, the helm chart adds a logs/k8s_entities pipeline to the cluster
// receiver that collects Kubernetes entity data via the k8s_cluster receiver and
// forwards it to the Splunk Observability v3/event endpoint using an otlphttp
// exporter. This test deploys the chart with the feature gate enabled, waits for
// data to arrive at a local OTLP HTTP sink that mimics the v3/event endpoint,
// and compares the collected logs against a golden file.
//
// Environment variables (all optional):
//
//	TEARDOWN_BEFORE_SETUP    – run teardown before setup when set to "true"
//	SKIP_SETUP               – skip chart installation when set to "true"
//	SKIP_TEARDOWN            – skip chart uninstall in cleanup when set to "true"
//	SKIP_TESTS               – skip assertions when set to "true"
//	UPDATE_EXPECTED_RESULTS  – overwrite golden file with actual results when set to "true"
//	KUBECONFIG               – path to a kubeconfig file (required for setup/teardown)
package k8sentities

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const otlpEntitiesPort = 4319

var entitiesLogsSink *consumertest.LogsSink

func Test_K8SEntities(t *testing.T) {
	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		t.Log("Running teardown before setup as TEARDOWN_BEFORE_SETUP is set to true")
		testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
		require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
		k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
		require.NoError(t, err)
		teardown(t, k8sClient)
	}

	internal.SetupSignalFxAPIServer(t)

	// Receive OTLP logs sent by the otlphttp/o11y_entities exporter to the /v3/event path.
	entitiesLogsSink = internal.SetupOTLPLogsSinkOnPort(t, otlpEntitiesPort, "/v3/event")

	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		deployCollector(t)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	internal.WaitForLogs(t, 1, entitiesLogsSink)

	t.Run("CheckK8SEntitiesLogs", func(t *testing.T) {
		allLogs := entitiesLogsSink.AllLogs()
		require.NotEmpty(t, allLogs, "expected at least one log batch from the k8s entities pipeline")

		// Merge all received log batches into a single plog.Logs for comparison.
		actualLogs := allLogs[0]
		for _, l := range allLogs[1:] {
			for i := 0; i < l.ResourceLogs().Len(); i++ {
				l.ResourceLogs().At(i).CopyTo(actualLogs.ResourceLogs().AppendEmpty())
			}
		}

		expectedFile := "testdata/expected_k8sentities.yaml"
		internal.MaybeUpdateExpectedLogsResults(t, expectedFile, &actualLogs)

		expectedLogs, err := golden.ReadLogs(expectedFile)
		require.NoError(t, err, "failed to read golden file %s", expectedFile)

		err = plogtest.CompareLogs(expectedLogs, actualLogs,
			plogtest.IgnoreTimestamp(),
			plogtest.IgnoreObservedTimestamp(),
			plogtest.IgnoreResourceAttributeValue("host.name"),
			plogtest.IgnoreResourceLogsOrder(),
			plogtest.IgnoreScopeLogsOrder(),
			plogtest.IgnoreLogRecordsOrder(),
		)
		require.NoError(t, err)
	})
}

func deployCollector(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	hostEp := internal.HostEndpoint(t)
	require.NotEmpty(t, hostEp, "host endpoint not found")

	valuesFile, err := filepath.Abs(filepath.Join("testdata", "k8sentities_values.yaml.tmpl"))
	require.NoError(t, err)

	replacements := map[string]any{
		"IngestURL": internal.HostPortHTTP(hostEp, otlpEntitiesPort),
		"ApiURL":    internal.HostPortHTTP(hostEp, internal.SignalFxAPIPort),
	}

	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 0, internal.GetDefaultChartOptions())

	config, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	internal.CheckPodsReady(t, clientset, internal.DefaultNamespace, "component=otel-k8s-cluster-receiver", 3*time.Minute, 0)
	// Give the cluster receiver time to emit entity data.
	time.Sleep(30 * time.Second)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		teardown(t, k8sClient)
	})
}

func teardown(t *testing.T, _ *k8stest.K8sClient) {
	testKubeConfig := os.Getenv("KUBECONFIG")
	internal.ChartUninstall(t, testKubeConfig)
}
