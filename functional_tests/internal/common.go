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

	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

var Namespace = "default"

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

func WriteNewExpectedTracesResult(t *testing.T, file string, trace *ptrace.Traces) {
	require.NoError(t, os.MkdirAll("results", 0o755))
	require.NoError(t, golden.WriteTraces(t, filepath.Join("results", filepath.Base(file)), *trace))
}

func WriteNewExpectedMetricsResult(t *testing.T, file string, metric *pmetric.Metrics) {
	require.NoError(t, os.MkdirAll("results", 0o755))
	require.NoError(t, golden.WriteMetrics(t, filepath.Join("results", filepath.Base(file)), *metric))
}

func WriteNewExpectedLogsResult(t *testing.T, file string, log *plog.Logs) {
	require.NoError(t, os.MkdirAll("results", 0o755))
	require.NoError(t, golden.WriteLogs(t, filepath.Join("results", filepath.Base(file)), *log))
}

func CheckPodsReady(t *testing.T, clientset *kubernetes.Clientset, namespace, labelSelector string,
	timeout time.Duration,
) {
	require.Eventually(t, func() bool {
		pods, err := clientset.CoreV1().Pods(namespace).List(t.Context(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		require.NoError(t, err)
		if len(pods.Items) == 0 {
			return false
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase != v1.PodRunning {
				return false
			}
			ready := false
			for _, condition := range pod.Status.Conditions {
				if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
					ready = true
					break
				}
			}
			if !ready {
				return false
			}
		}
		return true
	}, timeout, 5*time.Second, "Pods in namespace %s with label %s are not ready", namespace, labelSelector)
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
