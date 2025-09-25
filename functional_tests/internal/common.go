// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

var DefaultNamespace = "default"

const waitTimeout = 3 * time.Minute

func HostEndpoint(t *testing.T) string {
	if host, ok := os.LookupEnv("HOST_ENDPOINT"); ok {
		return host
	}
	if runtime.GOOS == "darwin" {
		return "host.docker.internal"
	}

	client, err := docker.NewClientWithOpts(docker.FromEnv)
	require.NoError(t, err)
	client.NegotiateAPIVersion(t.Context())
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	network, err := client.NetworkInspect(ctx, "kind", network.InspectOptions{})
	require.NoError(t, err)
	for _, ipam := range network.IPAM.Config {
		if ipam.Gateway != "" {
			return ipam.Gateway
		}
	}
	require.Fail(t, "failed to find host endpoint")
	return ""
}

func WaitForTraces(t *testing.T, entriesNum int, tc *consumertest.TracesSink) {
	require.Eventuallyf(t, func() bool {
		return len(tc.AllTraces()) >= entriesNum
	}, waitTimeout, 1*time.Second,
		"failed to receive %d entries,  received %d traces in %f minutes", entriesNum,
		len(tc.AllTraces()), waitTimeout.Minutes())
}

func WaitForLogs(t *testing.T, entriesNum int, lc *consumertest.LogsSink) {
	require.Eventuallyf(t, func() bool {
		return len(lc.AllLogs()) >= entriesNum
	}, waitTimeout, 1*time.Second,
		"failed to receive %d entries,  received %d logs in %f minutes", entriesNum,
		len(lc.AllLogs()), waitTimeout.Minutes())
}

func WaitForMetrics(t *testing.T, entriesNum int, mc *consumertest.MetricsSink) {
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) >= entriesNum
	}, waitTimeout, 1*time.Second,
		"failed to receive %d entries,  received %d metrics in %f minutes", entriesNum,
		len(mc.AllMetrics()), waitTimeout.Minutes())
}

func CheckNoEventsReceived(t *testing.T, lc *consumertest.LogsSink) {
	require.Empty(t, lc.AllLogs(), "received %d logs, expected 0 logs", len(lc.AllLogs()))
}

func CheckNoMetricsReceived(t *testing.T, lc *consumertest.MetricsSink) {
	require.Empty(t, lc.AllMetrics(), "received %d metrics, expected 0 metrics", len(lc.AllMetrics()))
}

func ResetMetricsSink(t *testing.T, mc *consumertest.MetricsSink) {
	mc.Reset()
	t.Logf("Metrics sink reset, current metrics: %d", len(mc.AllMetrics()))
}

func ResetLogsSink(t *testing.T, lc *consumertest.LogsSink) {
	lc.Reset()
	t.Logf("Logs sink reset, current logs: %d", len(lc.AllLogs()))
}

var shouldUpdateExpectedResults = func() bool {
	return os.Getenv("UPDATE_EXPECTED_RESULTS") == "true"
}

func MaybeWriteUpdateExpectedTracesResults(t *testing.T, file string, traces *ptrace.Traces) {
	if shouldUpdateExpectedResults() {
		require.NoError(t, golden.WriteTraces(t, filepath.Base(file), *traces))
		t.Logf("Wrote updated expected trace results to %s", filepath.Base(file))
	}
}

func MaybeUpdateExpectedMetricsResults(t *testing.T, file string, metrics *pmetric.Metrics) {
	if shouldUpdateExpectedResults() {
		require.NoError(t, golden.WriteMetrics(t, filepath.Base(file), *metrics))
		t.Logf("Wrote updated expected metric results to %s", filepath.Base(file))
	}
}

func MaybeUpdateExpectedLogsResults(t *testing.T, file string, logs *plog.Logs) {
	if shouldUpdateExpectedResults() {
		require.NoError(t, golden.WriteLogs(t, filepath.Base(file), *logs))
		t.Logf("Wrote updated expected log results to %s", filepath.Base(file))
	}
}

func CheckPodsReady(t *testing.T, clientset *kubernetes.Clientset, namespace, labelSelector string,
	timeout time.Duration, minReadyTime time.Duration,
) {
	var readySince time.Time
	require.Eventually(t, func() bool {
		pods, err := clientset.CoreV1().Pods(namespace).List(t.Context(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		require.NoError(t, err)
		if len(pods.Items) == 0 {
			readySince = time.Time{} // reset
			t.Logf("[CheckPodsReady] No pods found for selector '%s' in namespace '%s'", labelSelector, namespace)
			return false
		}
		allReady := true
		for _, pod := range pods.Items {
			ready := false
			for _, condition := range pod.Status.Conditions {
				if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
					ready = true
					break
				}
			}
			if pod.Status.Phase != v1.PodRunning || !ready {
				allReady = false
			}
			t.Logf("[CheckPodsReady] Pod: %s | Phase: %s | Ready: %v", pod.Name, pod.Status.Phase, ready)
		}
		if !allReady {
			readySince = time.Time{}
			return false
		}
		if readySince.IsZero() {
			readySince = time.Now()
			t.Logf("[CheckPodsReady] All pods ready at: %s", readySince.Format(time.RFC3339))
		}
		t.Logf("[CheckPodsReady] All pods have been ready for: %s (minReadyTime: %s)", time.Since(readySince), minReadyTime)
		return time.Since(readySince) >= minReadyTime
	}, timeout, 5*time.Second, "Pods in namespace %s with label %s are not ready for minReadyTime=%s", namespace, labelSelector, minReadyTime)
}

func CreatePod(t *testing.T, clientset *kubernetes.Clientset, name string, namespace string, podConfig *v1.Pod) {
	_, err := clientset.CoreV1().Pods(namespace).Create(t.Context(), podConfig, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, err = clientset.CoreV1().Pods(namespace).Get(t.Context(), name, metav1.GetOptions{})
		return err == nil
	}, 1*time.Minute, 5*time.Second, "pod %s is not available", name)
}

func DeletePod(t *testing.T, clientset *kubernetes.Clientset, name string, namespace string) {
	err := clientset.CoreV1().Pods(namespace).Delete(t.Context(), name, metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, err = clientset.CoreV1().Pods(namespace).Get(t.Context(), name, metav1.GetOptions{})
		return err != nil
	}, 1*time.Minute, 5*time.Second, "pod %s is still available", name)
}

func CreateNamespace(t *testing.T, clientset *kubernetes.Clientset, name string) {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := clientset.CoreV1().Namespaces().Create(t.Context(), ns, metav1.CreateOptions{})
	require.NoError(t, err, "failed to create namespace %s", name)

	require.Eventually(t, func() bool {
		_, err = clientset.CoreV1().Namespaces().Get(t.Context(), name, metav1.GetOptions{})
		return err == nil
	}, 1*time.Minute, 5*time.Second, "namespace %s is not available", name)
}

func DeleteNamespace(t *testing.T, clientset *kubernetes.Clientset, name string) {
	err := clientset.CoreV1().Namespaces().Delete(t.Context(), name, metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, err = clientset.CoreV1().Namespaces().Get(t.Context(), name, metav1.GetOptions{})
		return err != nil
	}, 1*time.Minute, 5*time.Second, "namespace %s is still available", name)
}

func LabelNamespace(t *testing.T, clientset *kubernetes.Clientset, name, key, value string) {
	ns, err := clientset.CoreV1().Namespaces().Get(t.Context(), name, metav1.GetOptions{})
	require.NoError(t, err)
	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}
	ns.Labels[key] = value
	_, err = clientset.CoreV1().Namespaces().Update(t.Context(), ns, metav1.UpdateOptions{})
	require.NoError(t, err)
}

func AnnotateNamespace(t *testing.T, clientset *kubernetes.Clientset, name, key, value string) {
	ns, err := clientset.CoreV1().Namespaces().Get(t.Context(), name, metav1.GetOptions{})
	require.NoError(t, err)
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	ns.Annotations[key] = value
	_, err = clientset.CoreV1().Namespaces().Update(t.Context(), ns, metav1.UpdateOptions{})
	require.NoError(t, err)
}

func WaitForTerminatingPods(t *testing.T, clientset *kubernetes.Clientset, namespace string) {
	require.Eventually(t, func() bool {
		pods, err := clientset.CoreV1().Pods(namespace).List(t.Context(), metav1.ListOptions{})
		require.NoError(t, err)

		terminatingPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning || pod.Status.Phase == v1.PodPending {
				continue
			}
			// Check if the pod is terminating
			if pod.DeletionTimestamp != nil {
				terminatingPods++
			}
		}

		return terminatingPods == 0
	}, 2*time.Minute, 5*time.Second, "there are still terminating pods after 2 minutes")
}

func DeleteObject(t *testing.T, k8sClient *k8stest.K8sClient, objYAML string) {
	obj := &unstructured.Unstructured{}
	require.NoError(t, yaml.Unmarshal([]byte(objYAML), obj))

	if err := k8stest.DeleteObject(k8sClient, obj); err != nil {
		// If an object that's being deleted is not found, it's considered successful deletion.
		// Some tests delete all resources on setup to ensure a clean running environment,
		// so it's a valid case to attempt to delete an object that doesn't exist.
		require.True(t, meta.IsNoMatchError(err) || strings.Contains(err.Error(), "not found"), "failed to delete object, err: %w", err)
	}
}

// SelectMetricSetWithTimeout finds a metrics payload containing a target metric with Eventually timeout
func SelectMetricSetWithTimeout(t *testing.T, expected pmetric.Metrics, targetMetric string, metricSink *consumertest.MetricsSink, ignoreLen bool, timeout time.Duration, interval time.Duration) *pmetric.Metrics {
	var selectedMetrics *pmetric.Metrics

	require.Eventuallyf(t, func() bool {
		for h := len(metricSink.AllMetrics()) - 1; h >= 0; h-- {
			m := metricSink.AllMetrics()[h]
			foundTargetMetric := false

		OUTER:
			for i := 0; i < m.ResourceMetrics().Len(); i++ {
				for j := 0; j < m.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
					for k := 0; k < m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
						metric := m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
						if metric.Name() == targetMetric {
							foundTargetMetric = true
							break OUTER
						}
					}
				}
			}

			if !foundTargetMetric {
				continue
			}

			if ignoreLen || (expected.ResourceMetrics().Len() > 0 && m.ResourceMetrics().Len() == expected.ResourceMetrics().Len() && m.MetricCount() == expected.MetricCount()) {
				selectedMetrics = &m
				t.Logf("Found target metric '%s' in payload with %d total metrics", targetMetric, m.MetricCount())
				return true
			}
		}
		t.Logf("Waiting for target metric '%s'...", targetMetric)
		return false
	}, timeout, interval, "Failed to find target metric %s within timeout period of %v", targetMetric, timeout)

	return selectedMetrics
}
