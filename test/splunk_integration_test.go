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

const EVENT_SEARCH_QUERY_STRING = "| search "
const METRIC_SEARCH_QUERY_STRING = "| mpreview "

func Test_Functions(t *testing.T) {

	t.Run("verify log ingestion by using annotations", testVerifyLogsIngestionUsingAnnotations)
	t.Run("custom metadata fields annotations", testVerifyCustomMetadataFieldsAnnotations)
	t.Run("metric namespace annotations", testVerifyMetricNamespaceAnnotations)

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
	namespace := "default"
	defaultIndex := "ci_metrics"
	defaultSourcetype := "httpevent"
	annotationIndex := "test_metrics"
	annotationSourcetype := "annotation_sourcetype"

	client := createK8sClient(t)

	tests := []struct {
		name                      string
		annotationIndexValue      string
		annotationSourcetypeValue string
	}{
		{"default index and default sourcetype", "", ""},
		{"annotation index and default sourcetype", annotationIndex, ""},
		{"default index and annotation sourcetype", "", annotationSourcetype},
		{"annotation index and annotation sourcetype", annotationIndex, annotationSourcetype},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addNamespaceAnnotation(t, client, namespace, tt.annotationIndexValue, tt.annotationSourcetypeValue)
			time.Sleep(20 * time.Second)

			index := defaultIndex
			if tt.annotationIndexValue != "" {
				index = tt.annotationIndexValue
			}
			sourcetype := defaultSourcetype
			if tt.annotationSourcetypeValue != "" {
				sourcetype = tt.annotationSourcetypeValue
			}
			searchQuery := METRIC_SEARCH_QUERY_STRING + "index=" + index + " filter=\"sourcetype=" + sourcetype + "\" | search \"k8s.namespace.name\"=" + namespace
			fmt.Println("Search Query: ", searchQuery)
			startTime := "-15s@s"
			events := CheckEventsFromSplunk(searchQuery, startTime)
			fmt.Println(" =========>  Events received: ", len(events))
			assert.Greater(t, len(events), 1)

			removeAllNamespaceAnnotations(t, client, namespace)
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

func addNamespaceAnnotation(t *testing.T, clientset *kubernetes.Clientset, namespace_name string, annotationIndex string, annotationSourcetype string) {
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

	_, err = clientset.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
	require.NoError(t, err)
	fmt.Printf("Annotation added to namespace_name %s\n", namespace_name)
}
