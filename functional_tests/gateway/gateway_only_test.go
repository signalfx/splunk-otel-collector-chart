// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"fmt"
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
	kubectlDelete   = "delete"
	testDir         = "testdata"
	testMetricName  = "cpu.num_processors"
	testPodManifest = "standalone-collector-pod.yaml"
)

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
		name        string
		valuesTmpl  string
		accessToken string
	}{
		{
			name:        "gateway_only_passthrough_enabled",
			valuesTmpl:  "gateway_only_passthrough_enabled_values.tmpl",
			accessToken: "standalonePodToken",
		},
		{
			name:        "gateway_only_passthrough_disabled",
			valuesTmpl:  "gateway_only_passthrough_disabled_values.tmpl",
			accessToken: "gatewayToken",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricSink := internal.SetupSignalFxReceiverWithToken(t, internal.SignalFxReceiverPort, tt.accessToken)
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
						rm := m.ResourceMetrics().At(i)
						for j := 0; j < rm.ScopeMetrics().Len(); j++ {
							sm := rm.ScopeMetrics().At(j)
							for k := 0; k < sm.Metrics().Len(); k++ {
								if sm.Metrics().At(k).Name() == testMetricName {
									foundExpectedMetric = true
								}
							}
						}
					}
				}
				return foundExpectedMetric
			}, 1*time.Minute, 1*time.Second, "failed to find expected metric %s", testMetricName)
		})
	}
}

func runKubectlFileCommand(t *testing.T, kubeConfig, action, manifestPath string) {
	t.Helper()
	cmd := exec.Command("kubectl", action, "-f", manifestPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeConfig))
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "failed to run kubectl %s for manifest %s: %s", action, manifestPath, string(output))
}
