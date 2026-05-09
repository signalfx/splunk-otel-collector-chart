// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CheckComponentHealth checks that an OpenTelemetry component is not logging errors.
// tailLines controls how many log lines to fetch from each pod.
func CheckComponentHealth(t *testing.T, clientset *kubernetes.Clientset, namespace, labelSelector string, componentName string, tailLines int64) {
	t.Helper()

	pods, err := clientset.CoreV1().Pods(namespace).List(t.Context(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	require.NoError(t, err, "failed to list pods with selector %s", labelSelector)
	require.NotEmpty(t, pods.Items, "no pods found with selector %s", labelSelector)

	t.Logf("Checking component health for %d pod(s) with selector: %s", len(pods.Items), labelSelector)
	CheckComponentHealthForPods(t, clientset, namespace, pods.Items, componentName, tailLines)
}

// CheckComponentHealthForPods checks that a component is not logging errors in
// the provided pod list. Use this when you already have a filtered set of pods
// (e.g. control-plane agent pods).
func CheckComponentHealthForPods(t *testing.T, clientset *kubernetes.Clientset, namespace string, pods []v1.Pod, componentName string, tailLines int64) {
	t.Helper()
	require.NotEmpty(t, pods, "no pods provided to check for component %s", componentName)

	for _, pod := range pods {
		if pod.Status.Phase != v1.PodRunning {
			t.Logf("Skipping pod %s in phase %s", pod.Name, pod.Status.Phase)
			continue
		}

		found := false
		for _, container := range pod.Spec.Containers {
			if container.Name == CollectorContainerName {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Skipping pod %s - no container named %s", pod.Name, CollectorContainerName)
			continue
		}

		t.Logf("Checking logs for pod: %s, container: %s", pod.Name, CollectorContainerName)

		logs, err := GetPodLogs(t, clientset, namespace, pod.Name, CollectorContainerName, tailLines)
		require.NoError(t, err, "failed to get logs for pod: %s", pod.Name)

		totalLines := len(strings.Split(logs, "\n"))
		errorCount := strings.Count(strings.ToLower(logs), "\terror\t")
		componentMentions := strings.Count(strings.ToLower(logs), strings.ToLower(componentName))
		t.Logf("Log stats: %d total lines, %d error-level logs, %d mentions of %s",
			totalLines, errorCount, componentMentions, componentName)

		errorLines := findMatchingLogLines(logs, componentName)

		if len(errorLines) > 0 {
			displayLines := errorLines
			suffix := ""
			if len(errorLines) > 20 {
				displayLines = errorLines[:20]
				suffix = fmt.Sprintf("\n... (%d more unique error lines)", len(errorLines)-20)
			}
			t.Errorf("Found %s component errors in pod %s:\n%s%s",
				componentName, pod.Name, strings.Join(displayLines, "\n"), suffix)
		} else {
			t.Logf("No %s component errors found in pod %s", componentName, pod.Name)
		}
	}
}

func findMatchingLogLines(logs string, componentName string) []string {
	lines := strings.Split(logs, "\n")
	lowerComponentName := strings.ToLower(componentName)

	type logEntry struct {
		firstLine string
		count     int
	}
	seenMessages := make(map[string]*logEntry)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		lowerLine := strings.ToLower(line)

		isErrorOrWarn := strings.Contains(lowerLine, "\terror\t") || strings.Contains(lowerLine, "\twarn\t")
		if !isErrorOrWarn {
			continue
		}
		// Match if the component name appears anywhere in the line (covers both
		// otelcol.component.id and nested "name" fields like receiver_creator sub-receivers).
		if !strings.Contains(lowerLine, lowerComponentName) {
			continue
		}

		parts := strings.SplitN(line, "\t", 2)
		var messageKey string
		if len(parts) == 2 {
			messageKey = parts[1]
		} else {
			messageKey = line
		}

		if entry, exists := seenMessages[messageKey]; exists {
			entry.count++
		} else {
			seenMessages[messageKey] = &logEntry{
				firstLine: line,
				count:     1,
			}
		}
	}

	uniqueErrorLines := []string{}
	for _, entry := range seenMessages {
		if entry.count > 1 {
			uniqueErrorLines = append(uniqueErrorLines, fmt.Sprintf("%s (repeated %d times)", entry.firstLine, entry.count))
		} else {
			uniqueErrorLines = append(uniqueErrorLines, entry.firstLine)
		}
	}

	sort.Strings(uniqueErrorLines)

	return uniqueErrorLines
}

// GetRunningAgentPods returns all running agent pods (matching labelSelector) with an
// otel-collector container. The k8s_observer in the chart is scoped to node: ${K8S_NODE_NAME},
// so each agent only discovers pods on its own node -- the test doesn't need to map components
// to nodes; it just searches all agents and expects at least one to have discovered the receiver.
func GetRunningAgentPods(t *testing.T, clientset *kubernetes.Clientset, namespace, labelSelector string) []v1.Pod {
	t.Helper()
	pods, err := clientset.CoreV1().Pods(namespace).List(t.Context(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	require.NoError(t, err, "failed to list pods with selector %s", labelSelector)

	var result []v1.Pod
	for _, pod := range pods.Items {
		if pod.Status.Phase != v1.PodRunning {
			continue
		}
		for _, c := range pod.Spec.Containers {
			if c.Name == CollectorContainerName {
				result = append(result, pod)
				break
			}
		}
	}
	t.Logf("Found %d running agent pod(s) with selector %s", len(result), labelSelector)
	return result
}

// GetControlPlaneAgentPods returns agent pods that are running on control-plane
// nodes.
func GetControlPlaneAgentPods(t *testing.T, clientset *kubernetes.Clientset, namespace, labelSelector string) []v1.Pod {
	t.Helper()
	allAgents := GetRunningAgentPods(t, clientset, namespace, labelSelector)

	nodes, err := clientset.CoreV1().Nodes().List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err, "failed to list nodes")

	cpNodes := make(map[string]bool)
	for _, node := range nodes.Items {
		for label := range node.Labels {
			if label == "node-role.kubernetes.io/control-plane" || label == "node-role.kubernetes.io/master" {
				cpNodes[node.Name] = true
				break
			}
		}
	}

	var result []v1.Pod
	for _, pod := range allAgents {
		if cpNodes[pod.Spec.NodeName] {
			result = append(result, pod)
		}
	}
	t.Logf("Filtered to %d agent pod(s) on control-plane nodes (out of %d total)", len(result), len(allAgents))
	return result
}

// CheckReceiverStarted verifies that at least one of the given agent pods has a log line
// matching receiver_creator's "starting receiver" message for the named receiver.
//
// receiver_creator (observerhandler.go) logs at Info:
//
//	"starting receiver"  name=<receiverName>{endpoint="..."}  endpoint=<ip>  endpoint_id=<id>
func CheckReceiverStarted(t *testing.T, clientset *kubernetes.Clientset, namespace string, pods []v1.Pod, receiverName string) {
	t.Helper()
	require.NotEmpty(t, pods, "no agent pods provided to check for receiver %s", receiverName)

	found := false
	for _, pod := range pods {
		// Use a large tail to capture the startup "starting receiver" message which
		// may be far back if the collector has been running for several minutes.
		logs, err := GetPodLogs(t, clientset, namespace, pod.Name, CollectorContainerName, 10000)
		if err != nil {
			t.Logf("Failed to get logs from pod %s: %v", pod.Name, err)
			continue
		}
		if containsReceiverStartLog(logs, receiverName) {
			t.Logf("Receiver %s started on pod %s (node %s)", receiverName, pod.Name, pod.Spec.NodeName)
			found = true
			break
		}
	}
	assert.True(t, found, "receiver %s was not started by receiver_creator on any of the %d agent pod(s)", receiverName, len(pods))
}

func WaitForScrapeInterval(t *testing.T, d time.Duration) {
	t.Helper()
	t.Logf("Waiting %s for receiver discovery and scrape cycles...", d)
	time.Sleep(d)
}

func containsReceiverStartLog(logs, receiverName string) bool {
	lowerReceiver := strings.ToLower(receiverName)
	for _, line := range strings.Split(logs, "\n") {
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "starting receiver") && strings.Contains(lowerLine, lowerReceiver) {
			return true
		}
	}
	return false
}
