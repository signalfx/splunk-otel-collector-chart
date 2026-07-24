// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	kubectlApply    = "apply"
	kubectlCommand  = "kubectl"
	kubectlDelete   = "delete"
	kubectlFileFlag = "-f"
	testDir         = "testdata"
	testPodManifest = "test-pod.yaml"
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

	podManifestPath := filepath.Join(testDir, testPodManifest)

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
			runKubectlFileCommand(t, testKubeConfig, kubectlApply, podManifestPath)
			t.Cleanup(func() {
				if os.Getenv("SKIP_TEARDOWN") == "true" {
					return
				}
				runKubectlFileCommand(t, testKubeConfig, kubectlDelete, podManifestPath)
				internal.ChartUninstall(t, testKubeConfig)
			})

			require.Eventually(t, func() bool {
				foundExpectedMetric := false
				for _, m := range metricSink.AllMetrics() {
					for i := 0; i < m.ResourceMetrics().Len(); i++ {
						sm := m.ResourceMetrics().At(i).ScopeMetrics().At(0)
						for j := 0; j < sm.Metrics().Len(); j++ {
							if sm.Metrics().At(j).Name() == "cpu.num_processors" {
								foundExpectedMetric = true
							}
						}
					}
				}
				return foundExpectedMetric
			}, 1*time.Minute, 1*time.Second, "expected to see metric")
		})
	}
}

func runKubectlFileCommand(t *testing.T, kubeConfig, action, manifestPath string) {
	t.Helper()
	cmd := exec.Command(kubectlCommand, action, kubectlFileFlag, manifestPath)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeConfig)
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "failed to run kubectl %s for manifest %s: %s", action, manifestPath, string(output))
}
