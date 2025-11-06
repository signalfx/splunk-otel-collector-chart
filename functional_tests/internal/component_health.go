// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CheckComponentHealth checks that an OpenTelemetry component is not logging errors
func CheckComponentHealth(t *testing.T, clientset *kubernetes.Clientset, namespace, labelSelector string, componentName string) {
	t.Helper()

	pods, err := clientset.CoreV1().Pods(namespace).List(t.Context(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	require.NoError(t, err, "failed to list pods with selector %s", labelSelector)
	require.NotEmpty(t, pods.Items, "no pods found with selector %s", labelSelector)

	t.Logf("Checking component health for %d pod(s) with selector: %s", len(pods.Items), labelSelector)

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

		// Debug: count total error lines and lines mentioning the component
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

		// Match error-level logs containing the component name
		if strings.Contains(lowerLine, "\terror\t") && strings.Contains(lowerLine, lowerComponentName) {
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

	sort.Strings(uniqueErrorLines)

	return uniqueErrorLines
}
