// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	testDir = "testdata"
)

// Env vars to control the test behavior:
// KUBECONFIG (required): the path to the kubeconfig file
// TEARDOWN_BEFORE_SETUP: if set to true, the test will run teardown before setup
// SKIP_TEARDOWN: if set to true, the test will skip teardown
// SKIP_SETUP: if set to true, the test will skip setup
func Test_GatewayOnly(t *testing.T) {
	testKubeConfig := getEnvVar(t, "KUBECONFIG")
	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		internal.ChartUninstall(t, testKubeConfig)
	}

	internal.SetupSignalFxAPIServer(t)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		internal.ChartUninstall(t, testKubeConfig)
	})

	tests := []struct {
		name       string
		valuesTmpl string
	}{
		{
			name:       "gateway_only_values",
			valuesTmpl: "gateway_only_values.tmpl",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricSink := internal.SetupSignalfxReceiver(t, internal.SignalFxReceiverPort)
			internal.BasicCollectorChartInstall(t, testKubeConfig, tt.valuesTmpl)
			t.Cleanup(func() {
				if os.Getenv("SKIP_TEARDOWN") == "true" {
					return
				}
				internal.ChartUninstall(t, testKubeConfig)
			})
			sendMetric(t)
			internal.WaitForMetrics(t, 1, metricSink)
			require.NotEmpty(t, metricSink.AllMetrics(), "expected at least one metric")
		})
	}
}

func sendMetric(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(testDir, "signalfx_gauge_metric.json"))
	require.NoError(t, err)

	// Send to kind cluster exposed port 9942, http forwarder sends to 9943
	req, err := http.NewRequest(http.MethodPost, "http://sock-splunk-otel-collector.default.svc.cluster.local:9942/v2/datapoint", bytes.NewBuffer(data))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-SF-Token", token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
}
