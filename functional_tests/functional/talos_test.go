// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const talosLogFixtureName = "talos-log-test"

// Install the fixture before the chart so the normal Helm readiness wait also
// gives file_log time to discover and export its container logs.
func deployTalosLogFixture(t *testing.T, client *kubernetes.Clientset) {
	t.Helper()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      talosLogFixtureName,
			Namespace: internal.DefaultNamespace,
			Labels:    map[string]string{"app": talosLogFixtureName},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    talosLogFixtureName,
				Image:   "busybox:1.38.0",
				Command: []string{"sh", "-c", "while true; do echo TALOS_CONTAINER_LOG_MARKER; sleep 1; done"},
			}},
			AutomountServiceAccountToken: new(false),
		},
	}
	_, err := client.CoreV1().Pods(internal.DefaultNamespace).Create(t.Context(), pod, metav1.CreateOptions{})
	require.True(t, err == nil || k8serrors.IsAlreadyExists(err), "failed to create Talos log fixture: %v", err)
	internal.CheckPodsReady(t, client, internal.DefaultNamespace, "app="+talosLogFixtureName, 3*time.Minute, 0)
	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		deleteErr := client.CoreV1().Pods(internal.DefaultNamespace).Delete(ctx, talosLogFixtureName, metav1.DeleteOptions{})
		if !k8serrors.IsNotFound(deleteErr) {
			require.NoError(t, deleteErr)
		}
	})
}

func runTalosTests(t *testing.T, client *kubernetes.Clientset, config *rest.Config) {
	t.Run("Talos node metrics and container logs", func(t *testing.T) {
		agents, err := internal.GetPods(t, client, internal.DefaultNamespace, internal.AgentLabelSelector)
		require.NoError(t, err)
		require.NotEmpty(t, agents.Items)
		foundFixtureLog := false
		for _, pod := range agents.Items {
			metricsPath := copyTalosTelemetry(t, client, config, pod.Name, "metrics")
			names := talosMetricNames(t, metricsPath)
			require.Contains(t, names, "system.cpu.time", "hostmetrics did not emit host CPU data on %s", pod.Spec.NodeName)
			require.Contains(t, names, "k8s.node.memory.working_set", "kubeletstats did not emit node memory data on %s", pod.Spec.NodeName)

			logsPath := copyTalosTelemetry(t, client, config, pod.Name, "logs")
			for _, record := range talosTelemetryRecords(t, logsPath) {
				logs, unmarshalErr := (&plog.JSONUnmarshaler{}).UnmarshalLogs(record)
				require.NoError(t, unmarshalErr)
				for i := 0; i < logs.ResourceLogs().Len(); i++ {
					resourceLogs := logs.ResourceLogs().At(i)
					if !hasAttrMatch(resourceLogs.Resource().Attributes(), "k8s.pod.name", talosLogFixtureName) {
						continue
					}
					for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
						records := resourceLogs.ScopeLogs().At(j).LogRecords()
						for k := 0; k < records.Len(); k++ {
							if records.At(k).Body().AsString() == "TALOS_CONTAINER_LOG_MARKER" {
								foundFixtureLog = true
							}
						}
					}
				}
			}
		}
		require.True(t, foundFixtureLog, "file_log did not collect the Talos container log fixture with Kubernetes pod metadata")
	})

	t.Run("Talos cluster metrics", func(t *testing.T) {
		pods, err := internal.GetPods(t, client, internal.DefaultNamespace, clusterReceiverLabelSelector)
		require.NoError(t, err)
		require.NotEmpty(t, pods.Items)
		names := talosMetricNames(t, copyTalosTelemetry(t, client, config, pods.Items[0].Name, "metrics"))
		require.Contains(t, names, "k8s.node.condition_ready", "k8s_cluster did not emit node readiness metrics")
	})
}

func copyTalosTelemetry(t *testing.T, client *kubernetes.Clientset, config *rest.Config, pod, signal string) string {
	t.Helper()
	localPath := filepath.Join(t.TempDir(), signal+".json")
	internal.CopyFileFromPod(t, client, config, internal.DefaultNamespace, pod, internal.CollectorContainerName,
		fmt.Sprintf("/splunk-telemetry/%s.json", signal), localPath)
	return localPath
}

func talosMetricNames(t *testing.T, filePath string) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for _, record := range talosTelemetryRecords(t, filePath) {
		metrics, err := (&pmetric.JSONUnmarshaler{}).UnmarshalMetrics(record)
		require.NoError(t, err)
		for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
			scopes := metrics.ResourceMetrics().At(i).ScopeMetrics()
			for j := 0; j < scopes.Len(); j++ {
				for k := 0; k < scopes.At(j).Metrics().Len(); k++ {
					metric := scopes.At(j).Metrics().At(k)
					if anyDataPointMatches(metric, func(_ pcommon.Map) bool { return true }) {
						names[metric.Name()] = true
					}
				}
			}
		}
	}
	return names
}

// The file exporter appends one OTLP JSON object per batch. Ignore only an
// unfinished final object if a batch was being written during the copy.
func talosTelemetryRecords(t *testing.T, filePath string) []json.RawMessage {
	t.Helper()
	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()
	decoder := json.NewDecoder(file)
	var records []json.RawMessage
	for {
		var record json.RawMessage
		decodeErr := decoder.Decode(&record)
		if errors.Is(decodeErr, io.EOF) || errors.Is(decodeErr, io.ErrUnexpectedEOF) {
			break
		}
		require.NoError(t, decodeErr)
		records = append(records, record)
	}
	require.NotEmpty(t, records, "no complete telemetry batches in %s", filePath)
	return records
}
