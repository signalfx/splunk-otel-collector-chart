// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

func deployCharts(
	t *testing.T,
	testKubeConfig string,
	client *kubernetes.Clientset,
	extensionsClient *clientset.Clientset,
) {
	kubeTestEnv, setKubeTestEnv := os.LookupEnv("KUBE_TEST_ENV")
	require.True(t, setKubeTestEnv, "the environment variable KUBE_TEST_ENV must be set")

	if requiresPrometheusResources(kubeTestEnv) {
		deployPrometheusCRDs(t, extensionsClient)
	}
	if kubeTestEnv == aksTestKubeEnv {
		createAKSWindowsValidationSecret(t, client)
	}

	chartInfo := map[string]internal.ChartOptions{}
	addChartInfo := func(fileName string, chartOption internal.ChartOptions) {
		valuesFile, errAbs := filepath.Abs(filepath.Join(testDir, valuesDir, fileName))
		assert.NoError(t, errAbs)
		chartInfo[valuesFile] = chartOption
	}
	switch kubeTestEnv {
	case autopilotTestKubeEnv:
		addChartInfo("autopilot_test_values.yaml.tmpl", internal.GetDefaultChartOptions())
	case aksTestKubeEnv:
		aksWinOpts := internal.ChartOptions{
			ChartNamespace:   internal.DefaultNamespace,
			ChartReleaseName: "aks-win",
			WaitStrategy:     kube.StatusWatcherStrategy,
			ChartTimeout:     internal.HelmActionTimeout,
		}
		aksLinuxOpts := internal.ChartOptions{
			ChartNamespace:   internal.DefaultNamespace,
			ChartReleaseName: "aks-linux",
			WaitStrategy:     kube.StatusWatcherStrategy,
			ChartTimeout:     internal.HelmActionTimeout,
		}
		if upgradeChartDir := os.Getenv("UPGRADE_FROM_CHART_DIR"); upgradeChartDir != "" {
			aksWinOpts.UpgradeFromValues = "aks_win_upgrade_from_previous_release_values.yaml"
			aksWinOpts.UpgradeFromChartDir = upgradeChartDir
			aksLinuxOpts.UpgradeFromValues = "aks_linux_upgrade_from_previous_release_values.yaml"
			aksLinuxOpts.UpgradeFromChartDir = upgradeChartDir
		}
		addChartInfo("aks_test_win_values.yaml.tmpl", aksWinOpts)
		addChartInfo("aks_test_linux_values.yaml.tmpl", aksLinuxOpts)
	case eksTestKubeEnv:
		addChartInfo("eks_test_values.yaml.tmpl", internal.GetDefaultChartOptions())
	case eksAutoModeTestKubeEnv:
		addChartInfo("eks_auto_mode_test_values.yaml.tmpl", internal.GetDefaultChartOptions())
	case eksFargateTestKubeEnv:
		addChartInfo("eks_fargate_test_values.yaml.tmpl", internal.GetDefaultChartOptions())
	case gkeTestKubeEnv:
		addChartInfo("gke_test_values.yaml.tmpl", internal.GetDefaultChartOptions())
	case rosaTestKubeEnv:
		addChartInfo("rosa_test_values.yaml.tmpl", internal.GetDefaultChartOptions())
	case gceTestKubeEnv:
		addChartInfo("gce_test_values.yaml.tmpl", internal.GetDefaultChartOptions())
	default:
		addChartInfo("test_values.yaml.tmpl", internal.GetDefaultChartOptions())
	}

	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}

	replacements := map[string]any{
		"K8sClusterEndpoint":    internal.HostPortHTTP(hostEp, signalFxReceiverK8sClusterReceiverPort),
		"AgentEndpoint":         internal.HostPortHTTP(hostEp, internal.SignalFxReceiverPort),
		"LogHecEndpoint":        internal.HostPortHTTP(hostEp, internal.HECLogsReceiverPort),
		"MetricHecEndpoint":     internal.HostPortHTTP(hostEp, internal.HECMetricsReceiverPort) + "/services/collector",
		"OtlpEndpoint":          internal.HostPort(hostEp, internal.OTLPGRPCReceiverPort),
		"OtlpHttpEndpoint":      internal.HostPort(hostEp, internal.OTLPHTTPReceiverPort),
		"ApiURLEndpoint":        internal.HostPortHTTP(hostEp, internal.SignalFxAPIPort),
		"LogObjectsHecEndpoint": internal.HostPortHTTP(hostEp, internal.HECObjectsReceiverPort) + "/services/collector",
		"KubeTestEnv":           kubeTestEnv,
	}

	for valuesFile, chartOption := range chartInfo {
		internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 1*time.Minute, chartOption)
	}
}

// createAKSWindowsValidationSecret ensures the mixed-node AKS test exercises the
// Linux-only secret validation hook instead of using a chart-created Secret.
func createAKSWindowsValidationSecret(t *testing.T, client kubernetes.Interface) {
	secrets := client.CoreV1().Secrets(internal.DefaultNamespace)
	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: aksWindowsValidationResourceName,
		},
		Data: map[string][]byte{
			"splunk_observability_access_token": []byte("test"),
			"splunk_platform_hec_token":         []byte("test"),
		},
	}

	existing, err := secrets.Get(t.Context(), desired.Name, metav1.GetOptions{})
	switch {
	case k8serrors.IsNotFound(err):
		_, err = secrets.Create(t.Context(), desired, metav1.CreateOptions{})
	case err != nil:
		require.NoError(t, err)
	default:
		desired.ResourceVersion = existing.ResourceVersion
		_, err = secrets.Update(t.Context(), desired, metav1.UpdateOptions{})
	}
	require.NoError(t, err)
}
