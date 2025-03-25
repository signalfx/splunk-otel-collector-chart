// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build splunk_integration

package test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

const EVENT_SEARCH_QUERY_STRING = "| search "
const METRIC_SEARCH_QUERY_STRING = "| mpreview "

func Test_Functions(t *testing.T) {

	t.Run("verify log ingestion by using annotations", testVerifyLogsIngestionUsingAnnotations)
	t.Run("custom metadata fields annotations", testVerifyCustomMetadataFieldsAnnotations)
	t.Run("metric index annotations", testVerifyMetricIndexAndSourcetypeAnnotations)

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
