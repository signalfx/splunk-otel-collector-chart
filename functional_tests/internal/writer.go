// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"gopkg.in/yaml.v3"
)

// WriteMetrics writes a pmetric.Metrics to the specified file in YAML format.
func WriteMetrics(t *testing.T, filePath string, metrics pmetric.Metrics) error {
	if err := writeMetrics(filePath, metrics); err != nil {
		return err
	}
	t.Logf("Golden file successfully written to %s.", filePath)
	t.Log("NOTE: The WriteMetrics call must be removed in order to pass the test.")
	t.Fail()
	return nil
}

// marshalMetricsYAML marshals a pmetric.Metrics to YAML format.
func marshalMetricsYAML(metrics pmetric.Metrics) ([]byte, error) {
	unmarshaler := &pmetric.JSONMarshaler{}
	fileBytes, err := unmarshaler.MarshalMetrics(metrics)
	if err != nil {
		return nil, err
	}
	var jsonVal map[string]interface{}
	if err = json.Unmarshal(fileBytes, &jsonVal); err != nil {
		return nil, err
	}
	b := &bytes.Buffer{}
	enc := yaml.NewEncoder(b)
	enc.SetIndent(2)
	if err := enc.Encode(jsonVal); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// writeMetrics writes a pmetric.Metrics to the specified file in YAML format.
func writeMetrics(filePath string, metrics pmetric.Metrics) error {
	sortMetrics(metrics)
	normalizeTimestamps(metrics)
	b, err := marshalMetricsYAML(metrics)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, b, 0600)
}

// WriteLogs writes a plog.Logs to the specified file in YAML format.
func WriteLogs(t *testing.T, filePath string, logs plog.Logs) error {
	if err := writeLogs(filePath, logs); err != nil {
		return err
	}
	t.Logf("Golden file successfully written to %s.", filePath)
	t.Log("NOTE: The WriteLogs call must be removed in order to pass the test.")
	t.Fail()
	return nil
}

func marshalLogs(logs plog.Logs) ([]byte, error) {
	unmarshaler := &plog.JSONMarshaler{}
	fileBytes, err := unmarshaler.MarshalLogs(logs)
	if err != nil {
		return nil, err
	}
	var jsonVal map[string]interface{}
	if err = json.Unmarshal(fileBytes, &jsonVal); err != nil {
		return nil, err
	}
	b := &bytes.Buffer{}
	enc := yaml.NewEncoder(b)
	enc.SetIndent(2)
	if err := enc.Encode(jsonVal); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func writeLogs(filePath string, logs plog.Logs) error {
	b, err := marshalLogs(logs)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, b, 0600)
}

// WriteTraces writes a ptrace.Traces to the specified file in YAML format.
func WriteTraces(t *testing.T, filePath string, traces ptrace.Traces) error {
	if err := writeTraces(filePath, traces); err != nil {
		return err
	}
	t.Logf("Golden file successfully written to %s.", filePath)
	t.Log("NOTE: The WriteLogs call must be removed in order to pass the test.")
	t.Fail()
	return nil
}

func marshalTraces(traces ptrace.Traces) ([]byte, error) {
	marshaler := &ptrace.JSONMarshaler{}
	fileBytes, err := marshaler.MarshalTraces(traces)
	if err != nil {
		return nil, err
	}
	var jsonVal map[string]interface{}
	if err = json.Unmarshal(fileBytes, &jsonVal); err != nil {
		return nil, err
	}
	b := &bytes.Buffer{}
	enc := yaml.NewEncoder(b)
	enc.SetIndent(2)
	if err := enc.Encode(jsonVal); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func writeTraces(filePath string, traces ptrace.Traces) error {
	b, err := marshalTraces(traces)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, b, 0600)
}
