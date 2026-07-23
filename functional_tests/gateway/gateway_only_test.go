// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	configMapFile          = "signalfx-metric-file-configmap.yaml"
	kubeConfigEnvKey       = "KUBECONFIG"
	kubeConfigEnvSeparator = "="
	kubectlApply           = "apply"
	kubectlCommand         = "kubectl"
	kubectlDelete          = "delete"
	kubectlFileFlag        = "-f"
	podFile                = "test-pod.yaml"
	testDir                = "testdata"
	testPodName            = "file-uploader-pod"
)

// Env vars to control the test behavior:
// KUBECONFIG (required): the path to the kubeconfig file
// TEARDOWN_BEFORE_SETUP: if set to true, the test will run teardown before setup
// SKIP_TEARDOWN: if set to true, the test will skip teardown
// SKIP_SETUP: if set to true, the test will skip setup
func Test_GatewayOnly(t *testing.T) {
	testKubeConfig := getEnvVar(t, kubeConfigEnvKey)
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
			manifestFiles := gatewayOnlyManifestFiles()
			metricSink := internal.SetupSignalfxReceiver(t, internal.SignalFxReceiverPort)
			internal.BasicCollectorChartInstall(t, testKubeConfig, tt.valuesTmpl)
			applyKubectlFiles(t, testKubeConfig, manifestFiles)
			t.Cleanup(func() {
				if os.Getenv("SKIP_TEARDOWN") == "true" {
					return
				}
				deleteKubectlFiles(t, testKubeConfig, manifestFiles)
				internal.ChartUninstall(t, testKubeConfig)
			})
			internal.WaitForMetrics(t, 1, metricSink)

			require.NotEmpty(t, metricSink.AllMetrics(), "expected at least one metric")
			for _, m := range metricSink.AllMetrics() {
				for i := 0; i < m.ResourceMetrics().Len(); i++ {
					sm := m.ResourceMetrics().At(i).ScopeMetrics().At(0)
					for j := 0; j < sm.Metrics().Len(); j++ {
						t.Logf("Metric sink received metric name: %s, type: %s", sm.Metrics().At(j).Name(), sm.Metrics().At(j).Type().String())
					}
				}
			}
			internal.LogPodContainerLogs(t, testKubeConfig, internal.DefaultNamespace, testPodName)
		})
	}
}

func applyKubectlFiles(t *testing.T, kubeConfig string, manifestFiles []string) {
	t.Helper()
	for _, manifestFile := range manifestFiles {
		runKubectlFileCommand(t, kubeConfig, kubectlApply, manifestFile)
	}
}

func deleteKubectlFiles(t *testing.T, kubeConfig string, manifestFiles []string) {
	t.Helper()
	for _, manifestFile := range manifestFiles {
		runKubectlFileCommand(t, kubeConfig, kubectlDelete, manifestFile)
	}
}

func gatewayOnlyManifestFiles() []string {
	return []string{
		configMapFile,
		podFile,
	}
}

func runKubectlFileCommand(t *testing.T, kubeConfig, action, manifestFile string) {
	t.Helper()
	manifestPath := filepath.Join(testDir, manifestFile)
	cmd := exec.Command(kubectlCommand, action, kubectlFileFlag, manifestPath)
	cmd.Env = append(os.Environ(), kubeConfigEnvKey+kubeConfigEnvSeparator+kubeConfig)
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "failed to run kubectl %s for manifest %s: %s", action, manifestPath, string(output))
}
