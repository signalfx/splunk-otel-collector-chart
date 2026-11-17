// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

func runTalosTests(t *testing.T, client *kubernetes.Clientset, config *rest.Config) {
	for _, test := range []struct {
		name, selector, assertionFile string
	}{
		{"agent metrics", internal.AgentLabelSelector, "expected_agent_metric_names_assertion.yaml"},
		{"cluster metrics", clusterReceiverLabelSelector, "expected_cluster_metric_names_assertion.yaml"},
	} {
		t.Run(test.name, func(t *testing.T) {
			pods, err := internal.GetPods(t, client, internal.DefaultNamespace, test.selector)
			require.NoError(t, err)
			require.NotEmpty(t, pods.Items)
			for _, pod := range pods.Items {
				sink := new(consumertest.MetricsSink)
				readPodTelemetry(t, client, config, pod.Name, "metrics.json", func(data []byte) {
					metrics, unmarshalErr := (&pmetric.JSONUnmarshaler{}).UnmarshalMetrics(data)
					require.NoError(t, unmarshalErr)
					require.NoError(t, sink.ConsumeMetrics(t.Context(), metrics))
				})
				internal.AssertMetricNames(t, sink, filepath.Join(testDir, "expected_talos_values", test.assertionFile), time.Second, 10*time.Millisecond)
			}
		})
	}

	if os.Getenv("UPGRADE_FROM_VALUES") != "" {
		return
	}
	t.Run("container logs", func(t *testing.T) {
		pods, err := internal.GetPods(t, client, internal.DefaultNamespace, internal.AgentLabelSelector)
		require.NoError(t, err)
		require.NotEmpty(t, pods.Items)
		found := false
		for _, pod := range pods.Items {
			readPodTelemetry(t, client, config, pod.Name, "logs.json", func(data []byte) {
				logs, unmarshalErr := (&plog.JSONUnmarshaler{}).UnmarshalLogs(data)
				require.NoError(t, unmarshalErr)
				for _, resourceLogs := range logs.ResourceLogs().All() {
					if !hasAttrMatch(resourceLogs.Resource().Attributes(), "k8s.container.name", "log-attr-test") {
						continue
					}
					for _, scopeLogs := range resourceLogs.ScopeLogs().All() {
						for _, record := range scopeLogs.LogRecords().All() {
							found = found || record.Body().AsString() == "LOG_ATTR_VALIDATION_MARKER"
						}
					}
				}
			})
		}
		require.True(t, found, "container log with Kubernetes metadata not collected from log-attr-test")
	})
}

// File exporters append newline-delimited batches. Read complete batches only,
// since the last write may still be in progress when the pod file is copied.
func readPodTelemetry(t *testing.T, client *kubernetes.Clientset, config *rest.Config, pod, fileName string, consume func([]byte)) {
	t.Helper()
	localPath := filepath.Join(t.TempDir(), fileName)
	internal.CopyFileFromPod(t, client, config, internal.DefaultNamespace, pod, internal.CollectorContainerName,
		"/splunk-telemetry/"+fileName, localPath)
	file, err := os.Open(localPath)
	require.NoError(t, err)
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		data, readErr := reader.ReadBytes('\n')
		if errors.Is(readErr, io.EOF) {
			return
		}
		require.NoError(t, readErr)
		consume(data)
	}
}
