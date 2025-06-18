// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package passthrough

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	token   = "tokenPassthrough"
	testDir = "testdata"
)

// Env vars to control the test behavior:
// KUBECONFIG (required): the path to the kubeconfig file
// TEARDOWN_BEFORE_SETUP: if set to true, the test will run teardown before setup
// SKIP_TEARDOWN: if set to true, the test will skip teardown
// SKIP_SETUP: if set to true, the test will skip setup
func Test_TokenPassthrough(t *testing.T) {
	testKubeConfig := getEnvVar(t, "KUBECONFIG")
	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t, testKubeConfig)
	}

	internal.SetupSignalFxAPIServer(t)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		teardown(t, testKubeConfig)
	})

	tests := []struct {
		name       string
		valuesTmpl string
	}{
		{
			name:       "agent_with_gateway",
			valuesTmpl: "agent_with_gateway_values.tmpl",
		},
		{
			name:       "agent_only",
			valuesTmpl: "agent_only_values.tmpl",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			internal.SetupSignalfxReceiver(t, internal.SignalFxReceiverPort)
			traceSink := internal.SetupOTLPTracesSinkWithToken(t, token)
			installCollectorChart(t, testKubeConfig, tt.valuesTmpl)
			t.Cleanup(func() {
				if os.Getenv("SKIP_TEARDOWN") == "true" {
					return
				}
				internal.ChartUninstall(t, testKubeConfig)
			})
			sendTrace(t)
			internal.WaitForTraces(t, 1, traceSink)
			require.NotEmpty(t, traceSink.AllTraces(), "expected at least one trace")
		})
	}
}

func installCollectorChart(t *testing.T, kubeConfig, valuesTmpl string) {
	t.Helper()
	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping collector chart installation as SKIP_SETUP is set to true")
		return
	}

	hostEp := internal.HostEndpoint(t)
	valuesFile, err := filepath.Abs(filepath.Join("testdata", valuesTmpl))
	require.NoError(t, err)
	internal.ChartInstallOrUpgrade(t, kubeConfig, valuesFile, map[string]any{
		"ApiURL":    fmt.Sprintf("http://%s:%d", hostEp, internal.SignalFxAPIPort),
		"IngestURL": fmt.Sprintf("http://%s:%d", hostEp, internal.SignalFxReceiverPort),
		"OTLPSink":  fmt.Sprintf("http://%s:%d", hostEp, internal.OTLPHTTPReceiverPort),
	}, 0)
}

func teardown(t *testing.T, kubeConfig string) {
	t.Helper()
	internal.ChartUninstall(t, kubeConfig)
}

func getEnvVar(t *testing.T, key string) string {
	value, ok := os.LookupEnv(key)
	require.True(t, ok, "the environment variable %s must be set", key)
	return value
}

func sendTrace(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(testDir, "trace.json"))
	require.NoError(t, err)

	// Send to kind cluster exposed port 43180 which is mapped to the container port 4318
	req, err := http.NewRequest(http.MethodPost, "http://localhost:43180/v1/traces", bytes.NewBuffer(data))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-SF-Token", token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
}
