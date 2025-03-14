// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
)

const Namespace = "default"

func HostEndpoint(t *testing.T) string {
	if host, ok := os.LookupEnv("HOST_ENDPOINT"); ok {
		return host
	}
	if runtime.GOOS == "darwin" {
		return "host.docker.internal"
	}

	client, err := docker.NewClientWithOpts(docker.FromEnv)
	require.NoError(t, err)
	client.NegotiateAPIVersion(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	network, err := client.NetworkInspect(ctx, "kind", types.NetworkInspectOptions{})
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
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(tc.AllTraces()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d traces in %d minutes", entriesNum,
		len(tc.AllTraces()), timeoutMinutes)
}

func WaitForLogs(t *testing.T, entriesNum int, lc *consumertest.LogsSink) {
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(lc.AllLogs()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d logs in %d minutes", entriesNum,
		len(lc.AllLogs()), timeoutMinutes)
}

func WaitForMetrics(t *testing.T, entriesNum int, mc *consumertest.MetricsSink) {
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics in %d minutes", entriesNum,
		len(mc.AllMetrics()), timeoutMinutes)
}

func CheckNoEventsReceived(t *testing.T, lc *consumertest.LogsSink) {
	require.True(t, len(lc.AllLogs()) == 0,
		"received %d logs, expected 0 logs", len(lc.AllLogs()))
}

func CheckNoMetricsReceived(t *testing.T, lc *consumertest.MetricsSink) {
	require.True(t, len(lc.AllMetrics()) == 0,
		"received %d metrics, expected 0 metrics", len(lc.AllMetrics()))
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
	require.NoError(t, os.MkdirAll("results", 0755))
	require.NoError(t, golden.WriteTraces(t, filepath.Join("results", filepath.Base(file)), *trace))
}

func WriteNewExpectedMetricsResult(t *testing.T, file string, metric *pmetric.Metrics) {
	require.NoError(t, os.MkdirAll("results", 0755))
	require.NoError(t, golden.WriteMetrics(t, filepath.Join("results", filepath.Base(file)), *metric))
}

func WriteNewExpectedLogsResult(t *testing.T, file string, log *plog.Logs) {
	require.NoError(t, os.MkdirAll("results", 0755))
	require.NoError(t, golden.WriteLogs(t, filepath.Join("results", filepath.Base(file)), *log))
}

func CheckPodsReady(t *testing.T, clientset *kubernetes.Clientset, namespace, labelSelector string,
	timeout time.Duration) {
	require.Eventually(t, func() bool {
		pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
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
	_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	require.NoError(t, err, "failed to create namespace %s", name)

	require.Eventually(t, func() bool {
		_, err := clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
		return err == nil
	}, 1*time.Minute, 5*time.Second, "namespace %s is not available", name)
}

func LabelNamespace(t *testing.T, clientset *kubernetes.Clientset, name, key, value string) {
	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	require.NoError(t, err)
	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}
	ns.Labels[key] = value
	_, err = clientset.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
	require.NoError(t, err)
}

func AnnotateNamespace(t *testing.T, clientset *kubernetes.Clientset, name, key, value string) {
	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	require.NoError(t, err)
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	ns.Annotations[key] = value
	_, err = clientset.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
	require.NoError(t, err)
}

func WaitForAllDeploymentsToStart(t *testing.T, client *kubernetes.Clientset) {
	require.Eventually(t, func() bool {
		di, err := client.AppsV1().Deployments(Namespace).List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)
		for _, d := range di.Items {
			if d.Status.ReadyReplicas != d.Status.Replicas {
				var messages string
				for _, c := range d.Status.Conditions {
					messages += c.Message
					messages += "\n"
				}

				t.Logf("Deployment not ready: %s, %s", d.Name, messages)
				return false
			}
		}
		return true
	}, 10*time.Minute, 10*time.Second)
}
