// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build splunk_integration

package test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	EVENT_SEARCH_QUERY_STRING  = "| search "
	METRIC_SEARCH_QUERY_STRING = "| mpreview "
)

func Test_Functions(t *testing.T) {
	t.Run("verify log ingestion by using annotations", testVerifyLogsIngestionUsingAnnotations)
	t.Run("custom metadata fields annotations", testVerifyCustomMetadataFieldsAnnotations)
	t.Run("metric namespace annotations", testVerifyMetricNamespaceAnnotations)
	t.Run("metric pod annotations", testVerifyMetricPodAnnotations)
}

func testVerifyLogsIngestionUsingAnnotations(t *testing.T) {
	tests := []struct {
		name               string
		label              string
		index              string
		expectedNoOfEvents int
	}{
		{"no annotations for namespace and pod", "pod-wo-index-wo-ns-index", "ci_events", 45},
		{"pod annotation only", "pod-w-index-wo-ns-index", "pod-anno", 100},
		{"namespace annotation only", "pod-wo-index-w-ns-index", "ns-anno", 15},
		{"pod and namespace annotation", "pod-w-index-w-ns-index", "pod-anno", 10},
		{"exclude namespace annotation", "pod-w-index-w-ns-exclude", "*", 0},
		{"exclude pod annotation", "pod-wo-index-w-exclude-w-ns-index", "*", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Printf("Test: %s - %s", tt.name, tt.label)
			searchQuery := EVENT_SEARCH_QUERY_STRING + "index=" + tt.index + " k8s.pod.labels.app::" + tt.label
			startTime := "-1h@h"
			events := CheckEventsFromSplunk(searchQuery, startTime)
			fmt.Println(" =========>  Events received: ", len(events))
			assert.Equal(t, len(events), tt.expectedNoOfEvents)
		})
	}
}

func testVerifyCustomMetadataFieldsAnnotations(t *testing.T) {
	tests := []struct {
		name               string
		label              string
		index              string
		value              string
		expectedNoOfEvents int
	}{
		{"custom metadata 1", "pod-w-index-wo-ns-index", "pod-anno", "pod-value-2", 100},
		{"custom metadata 2", "pod-w-index-w-ns-index", "pod-anno", "pod-value-1", 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Printf("Testing custom metadata annotation label=%s value=%s expected=%d event(s)", tt.label, tt.value, tt.expectedNoOfEvents)
			searchQuery := EVENT_SEARCH_QUERY_STRING + "index=" + tt.index + " k8s.pod.labels.app::" + tt.label + " customField::" + tt.value
			startTime := "-1h@h"
			events := CheckEventsFromSplunk(searchQuery, startTime)
			fmt.Println(" =========>  Events received: ", len(events))
			assert.Equal(t, len(events), tt.expectedNoOfEvents)
		})
	}
}

func testVerifyMetricIndexAndSourcetypeAnnotations(t *testing.T) {
	t.Run("metrics sent to metricIndex", func(t *testing.T) {
		fmt.Println("Test that metrics are being sent to 'test_metrics' index, as defined by splunk.com/metricsIndex annotation added during setup")
		index := "test_metrics"
		sourcetype := "sourcetype-anno"
		searchQuery := METRIC_SEARCH_QUERY_STRING + "index=" + index + " filter=\"sourcetype=" + sourcetype + "\""
		startTime := "-1h@h"
		events := CheckEventsFromSplunk(searchQuery, startTime)
		fmt.Println(" =========>  Events received: ", len(events))
		assert.Greater(t, len(events), 1)
	})
}

func testVerifyMetricNamespaceAnnotations(t *testing.T) {
	namespace := "namespace-metric-annotation"
	defaultIndex := "ci_metrics"
	defaultSourcetype := "httpevent"
	annotationIndex := "test_metrics"
	podAnnotationIndex := "test_metrics_annotation"
	annotationSourcetype := "annotation_sourcetype"
	podAnnotationSourcetype := "pod_annotation_sourcetype"
	podName := "pod-for-namespace-metric-annotation"

	client := createK8sClient(t)

	tests := []struct {
		name                      string
		annotationIndexValue      string
		annotationSourcetypeValue string
		usePodAnnotations         bool
	}{
		{"default index and default sourcetype", "", "", false},
		{"annotation index and default sourcetype", annotationIndex, "", false},
		{"default index and annotation sourcetype", "", annotationSourcetype, false},
		{"annotation index and annotation sourcetype", annotationIndex, annotationSourcetype, false},
		{"annotation index and annotation sourcetype in both namespace and pod", annotationIndex, annotationSourcetype, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addNamespaceAnnotation(t, client, namespace, tt.annotationIndexValue, tt.annotationSourcetypeValue, "")
			index := defaultIndex
			if tt.annotationIndexValue != "" {
				index = tt.annotationIndexValue
			}
			sourcetype := defaultSourcetype
			if tt.annotationSourcetypeValue != "" {
				sourcetype = tt.annotationSourcetypeValue
			}
			if tt.usePodAnnotations {
				addPodAnnotation(t, client, podName, namespace, podAnnotationIndex, podAnnotationSourcetype, "")
				index = podAnnotationIndex
				sourcetype = podAnnotationSourcetype
			}
			time.Sleep(20 * time.Second)
			searchQuery := METRIC_SEARCH_QUERY_STRING + "index=" + index + " filter=\"sourcetype=" + sourcetype + "\" | search \"k8s.namespace.name\"=" + namespace + " \"k8s.pod.name\"=" + podName
			fmt.Println("Search Query: ", searchQuery)
			startTime := "-15s@s"
			events := CheckEventsFromSplunk(searchQuery, startTime)
			fmt.Println(" =========>  Events received: ", len(events))
			assert.Greater(t, len(events), 1)

			removeAllNamespaceAnnotations(t, client, namespace)
			if tt.usePodAnnotations {
				removeAllPodsAnnotations(t, client, podName, namespace)
			}
		})
	}
}

func testVerifyMetricPodAnnotations(t *testing.T) {
	podName := "pod-for-metric-annotation"
	namespace := "namespace-pod-metric-annotation"
	defaultIndex := "ci_metrics"
	defaultSourcetype := "httpevent"
	annotationIndex := "test_metrics"
	annotationSourcetype := "annotation_sourcetype"
	annotationMetricSourcetype := "annotation_metric_sourcetype"

	client := createK8sClient(t)

	tests := []struct {
		name                            string
		annotationIndexValue            string
		annotationSourcetypeValue       string
		annotationMetricSourcetypeValue string
	}{
		{"default index and default sourcetype", "", "", ""},
		{"annotation index and default sourcetype", annotationIndex, "", ""},
		{"default index and annotation sourcetype", "", annotationSourcetype, ""},
		{"default index, default sourcetype and annotation metric sourcetype", "", "", annotationMetricSourcetype},
		{"default index, annotation sourcetype and annotation metric sourcetype", "", annotationSourcetype, annotationMetricSourcetype},
		{"annotation index and annotation sourcetype", annotationIndex, annotationSourcetype, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addPodAnnotation(t, client, podName, namespace, tt.annotationIndexValue, tt.annotationSourcetypeValue, tt.annotationMetricSourcetypeValue)
			time.Sleep(20 * time.Second)
			index := defaultIndex
			if tt.annotationIndexValue != "" {
				index = tt.annotationIndexValue
			}
			sourcetype := defaultSourcetype
			if tt.annotationSourcetypeValue != "" {
				sourcetype = tt.annotationSourcetypeValue
			}
			if tt.annotationMetricSourcetypeValue != "" {
				sourcetype = tt.annotationMetricSourcetypeValue
			}
			searchQuery := METRIC_SEARCH_QUERY_STRING + "index=" + index + " filter=\"sourcetype=" + sourcetype + "\" | search \"k8s.pod.name\"=" + podName
			fmt.Println("Search Query: ", searchQuery)
			startTime := "-15s@s"
			events := CheckEventsFromSplunk(searchQuery, startTime)
			fmt.Println(" =========>  Events received: ", len(events))
			assert.Greater(t, len(events), 1)

			removeAllPodsAnnotations(t, client, podName, namespace)
		})
	}
}

func createK8sClient(t *testing.T) *kubernetes.Clientset {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)
	return client
}

func removeAllNamespaceAnnotations(t *testing.T, clientset *kubernetes.Clientset, namespace_name string) {
	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace_name, metav1.GetOptions{})
	require.NoError(t, err)
	ns.Annotations = make(map[string]string)

	_, err = clientset.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
	require.NoError(t, err)
	fmt.Printf("All annotations removed from namespace_name %s\n", namespace_name)
}

func removeAllPodsAnnotations(t *testing.T, clientset *kubernetes.Clientset, pod_name string, namespace string) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod_name, metav1.GetOptions{})
	require.NoError(t, err)
	pod.Annotations = make(map[string]string)

	_, err = clientset.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
	require.NoError(t, err)
	fmt.Printf("All annotations removed from pod_name %s\n", pod_name)
}

func addNamespaceAnnotation(t *testing.T, clientset *kubernetes.Clientset, namespace_name string, annotationIndex string, annotationSourcetype string, annotationSourcetypeMetrics string) {
	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace_name, metav1.GetOptions{})
	require.NoError(t, err)
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	if annotationIndex != "" {
		ns.Annotations["splunk.com/metricsIndex"] = annotationIndex
	}
	if annotationSourcetype != "" {
		ns.Annotations["splunk.com/sourcetype"] = annotationSourcetype
	}
	if annotationSourcetypeMetrics != "" {
		ns.Annotations["splunk.com/metricsSourcetype"] = annotationSourcetypeMetrics
	}

	_, err = clientset.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
	require.NoError(t, err)
	fmt.Printf("Annotation added to namespace_name %s\n", namespace_name)
}

func addPodAnnotation(t *testing.T, clientset *kubernetes.Clientset, pod_name string, namespace string, annotationIndex string, annotationSourcetype string, annotationSourcetypeMetrics string) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod_name, metav1.GetOptions{})
	require.NoError(t, err)
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	if annotationIndex != "" {
		pod.Annotations["splunk.com/metricsIndex"] = annotationIndex
	}
	if annotationSourcetype != "" {
		pod.Annotations["splunk.com/sourcetype"] = annotationSourcetype
	}
	if annotationSourcetypeMetrics != "" {
		pod.Annotations["splunk.com/metricsSourcetype"] = annotationSourcetypeMetrics
	}
	_, err = clientset.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
	require.NoError(t, err)
	fmt.Printf("Annotation added to pod_name %s\n", pod_name)
}
