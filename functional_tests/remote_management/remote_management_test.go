// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package remotemanagement

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/strvals"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	agentConfigMapName               = internal.DefaultChartReleaseName + "-splunk-otel-collector-otel-agent"
	remoteManagementConfigAnnotation = "splunk.com/remote-management-config"
	remoteManagedRelay               = `receivers:
  otlp:
    protocols:
      grpc:
exporters:
  debug:
service:
  pipelines:
    metrics:
      receivers:
        - otlp
      exporters:
        - debug
remote_management_functional_marker: preserved
`
)

type upgradeResult struct {
	helmRenderedRelay string
	upgradedRelay     string
}

func TestRemoteManagementConfigMapUpgrade(t *testing.T) {
	testKubeConfig, ok := os.LookupEnv("KUBECONFIG")
	require.True(t, ok, "the environment variable KUBECONFIG must be set")

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t, testKubeConfig)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	t.Run("preserves remote-managed relay on helm upgrade", func(t *testing.T) {
		result := installMutateAndUpgrade(t, testKubeConfig, "", false)
		require.Equal(t, remoteManagedRelay, result.upgradedRelay)
	})

	t.Run("resets remote-managed relay when resetFromHelm is set", func(t *testing.T) {
		result := installMutateAndUpgrade(t, testKubeConfig, "remoteManagement.collectorConfig.upgradeStrategy=resetFromHelm", true)
		require.Equal(t, result.helmRenderedRelay, result.upgradedRelay)
	})
}

func installMutateAndUpgrade(t *testing.T, testKubeConfig string, upgradeSet string, forceConflicts bool) upgradeResult {
	teardown(t, testKubeConfig)
	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		teardown(t, testKubeConfig)
	})

	clientset, err := internal.GetKubeClient(testKubeConfig)
	require.NoError(t, err)

	installValues := remoteManagementValues(t)
	installChart(t, testKubeConfig, installValues)

	installedConfigMap := getAgentConfigMap(t, clientset)
	require.Equal(t, "true", installedConfigMap.Annotations[remoteManagementConfigAnnotation])
	helmRenderedRelay := installedConfigMap.Data["relay"]
	require.NotEmpty(t, helmRenderedRelay)
	require.NotContains(t, helmRenderedRelay, "remote_management_functional_marker")

	applyAgentRelayAsOpAMPBridge(t, clientset, remoteManagedRelay)

	upgradeValues := remoteManagementValues(t)
	if upgradeSet != "" {
		require.NoError(t, strvals.ParseInto(upgradeSet, upgradeValues))
	}
	upgradeChart(t, testKubeConfig, upgradeValues, forceConflicts)

	upgradedRelay := getAgentConfigMap(t, clientset).Data["relay"]
	return upgradeResult{
		helmRenderedRelay: helmRenderedRelay,
		upgradedRelay:     upgradedRelay,
	}
}

func remoteManagementValues(t *testing.T) map[string]any {
	t.Helper()

	valuesYAML := `
clusterName: remote-management-functional

splunkObservability:
  realm: fake-realm
  accessToken: fake-token

remoteManagement:
  enabled: true
  opampBridge:
    nodeSelector:
      remote-management-functional-test: "disabled"

clusterReceiver:
  enabled: false

gateway:
  enabled: false

nodeSelector:
  remote-management-functional-test: "disabled"

agent:
  controlPlaneMetrics:
    apiserver:
      enabled: false
    controllerManager:
      enabled: false
    coredns:
      enabled: false
`

	var values map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(valuesYAML), &values))
	return values
}

func installChart(t *testing.T, testKubeConfig string, values map[string]any) {
	t.Helper()

	actionConfig := internal.InitHelmActionConfig(t, testKubeConfig)
	install := action.NewInstall(actionConfig)
	install.Namespace = internal.DefaultNamespace
	install.ReleaseName = internal.DefaultChartReleaseName
	install.WaitStrategy = kube.HookOnlyStrategy
	install.Timeout = internal.HelmActionTimeout

	t.Log("Running helm install")
	_, err := install.Run(loadChart(t), values)
	require.NoError(t, err)
}

func upgradeChart(t *testing.T, testKubeConfig string, values map[string]any, forceConflicts bool) {
	t.Helper()

	actionConfig := internal.InitHelmActionConfig(t, testKubeConfig)
	upgrade := action.NewUpgrade(actionConfig)
	upgrade.Namespace = internal.DefaultNamespace
	upgrade.WaitStrategy = kube.HookOnlyStrategy
	upgrade.Timeout = internal.HelmActionTimeout
	upgrade.ForceConflicts = forceConflicts

	t.Log("Running helm upgrade")
	_, err := upgrade.Run(internal.DefaultChartReleaseName, loadChart(t), values)
	require.NoError(t, err)
}

func getAgentConfigMap(t *testing.T, clientset *kubernetes.Clientset) *corev1.ConfigMap {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	configMap, err := clientset.CoreV1().ConfigMaps(internal.DefaultNamespace).Get(ctx, agentConfigMapName, metav1.GetOptions{})
	require.NoError(t, err)
	return configMap
}

func applyAgentRelayAsOpAMPBridge(t *testing.T, clientset *kubernetes.Clientset, relay string) {
	t.Helper()

	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()

		configMap := corev1apply.ConfigMap(agentConfigMapName, internal.DefaultNamespace).
			WithData(map[string]string{"relay": relay})
		_, err := clientset.CoreV1().ConfigMaps(internal.DefaultNamespace).Apply(ctx, configMap, metav1.ApplyOptions{
			FieldManager: "opampbridge",
			Force:        true,
		})
		if err != nil {
			t.Logf("failed to apply agent ConfigMap relay as OpAMP Bridge: %v", err)
			return false
		}
		return true
	}, time.Minute, time.Second)
}

func teardown(t *testing.T, testKubeConfig string) {
	t.Helper()

	actionConfig := internal.InitHelmActionConfig(t, testKubeConfig)
	uninstall := action.NewUninstall(actionConfig)
	uninstall.IgnoreNotFound = true
	uninstall.WaitStrategy = kube.StatusWatcherStrategy
	uninstall.Timeout = internal.HelmActionTimeout

	_, err := uninstall.Run(internal.DefaultChartReleaseName)
	if err != nil && !k8serrors.IsNotFound(err) {
		require.NoError(t, err)
	}
}

func loadChart(t *testing.T) chart.Charter {
	t.Helper()

	chartPath := filepath.Join("..", "..", "helm-charts", "splunk-otel-collector")
	c, err := loader.Load(chartPath)
	require.NoError(t, err)
	return c
}
