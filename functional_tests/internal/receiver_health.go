// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	kubeletstatsReceiverName = "kubeletstatsreceiver"
	k8sClusterReceiverName   = "k8sclusterreceiver"
)

// CheckReceiverHealth checks that receivers are working without RBAC or connection errors
func CheckReceiverHealth(t *testing.T, clientset *kubernetes.Clientset, namespace, labelSelector string, receiverComponentName string) {
	t.Helper()

	pods, err := clientset.CoreV1().Pods(namespace).List(t.Context(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	require.NoError(t, err, "failed to list pods with selector %s", labelSelector)
	require.NotEmpty(t, pods.Items, "no pods found with selector %s", labelSelector)

	t.Logf("Checking receiver health for %d pod(s) with selector: %s", len(pods.Items), labelSelector)

	for _, pod := range pods.Items {
		if pod.Status.Phase != "Running" {
			t.Logf("Skipping pod %s in phase %s", pod.Name, pod.Status.Phase)
			continue
		}

		containerName := "otel-collector"
		found := false
		for _, container := range pod.Spec.Containers {
			if container.Name == containerName {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Skipping pod %s - no container named %s", pod.Name, containerName)
			continue
		}

		t.Logf("Checking logs for pod: %s, container: %s", pod.Name, containerName)

		logs := GetPodLogs(t, clientset, namespace, pod.Name, containerName, 500)

		// Debug: count total error lines and lines mentioning the receiver
		totalLines := len(strings.Split(logs, "\n"))
		errorCount := strings.Count(strings.ToLower(logs), "\terror\t")
		receiverMentions := strings.Count(strings.ToLower(logs), strings.ToLower(receiverComponentName))
		t.Logf("Log stats: %d total lines, %d error-level logs, %d mentions of %s",
			totalLines, errorCount, receiverMentions, receiverComponentName)

		errorLines := findMatchingLogLines(logs, receiverComponentName)

		if len(errorLines) > 0 {
			displayLines := errorLines
			suffix := ""
			if len(errorLines) > 20 {
				displayLines = errorLines[:20]
				suffix = fmt.Sprintf("\n... (%d more unique error lines)", len(errorLines)-20)
			}
			t.Errorf("Found %s receiver errors in pod %s:\n%s%s",
				receiverComponentName, pod.Name, strings.Join(displayLines, "\n"), suffix)
		} else {
			t.Logf("No %s receiver errors found in pod %s", receiverComponentName, pod.Name)
		}
	}
}

func CheckKubeletstatsReceiverHealth(t *testing.T, clientset *kubernetes.Clientset, namespace, agentLabelSelector string) {
	t.Helper()
	CheckReceiverHealth(t, clientset, namespace, agentLabelSelector, kubeletstatsReceiverName)
}

func CheckK8sClusterReceiverHealth(t *testing.T, clientset *kubernetes.Clientset, namespace, clusterReceiverLabelSelector string) {
	t.Helper()
	CheckReceiverHealth(t, clientset, namespace, clusterReceiverLabelSelector, k8sClusterReceiverName)
}

func findMatchingLogLines(logs string, receiverComponentName string) []string {
	lines := strings.Split(logs, "\n")

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

		// Match error-level logs containing the receiver name
		if strings.Contains(lowerLine, "\terror\t") && strings.Contains(lowerLine, strings.ToLower(receiverComponentName)) {
			// Strip timestamp to deduplicate
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
	}

	uniqueErrorLines := []string{}
	for _, entry := range seenMessages {
		if entry.count > 1 {
			uniqueErrorLines = append(uniqueErrorLines, fmt.Sprintf("%s (repeated %d times)", entry.firstLine, entry.count))
		} else {
			uniqueErrorLines = append(uniqueErrorLines, entry.firstLine)
		}
	}

	return uniqueErrorLines
}

func AssertNoReceiverErrors(t *testing.T, clientset *kubernetes.Clientset, namespace, agentLabelSelector, clusterReceiverLabelSelector string) {
	t.Helper()

	t.Run("kubeletstats receiver health", func(t *testing.T) {
		CheckKubeletstatsReceiverHealth(t, clientset, namespace, agentLabelSelector)
	})

	t.Run("k8s_cluster receiver health", func(t *testing.T) {
		CheckK8sClusterReceiverHealth(t, clientset, namespace, clusterReceiverLabelSelector)
	})
}
