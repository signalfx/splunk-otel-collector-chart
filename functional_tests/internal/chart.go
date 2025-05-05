// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	HelmActionTimeout = 10 * time.Minute
	chartReleaseName  = "sock"
	defaultChartPath  = "helm-charts/splunk-otel-collector"
)

func ChartInstallOrUpgrade(t *testing.T, testKubeConfig string, valuesFile string, replacements map[string]any) {
	valuesBytes, err := os.ReadFile(valuesFile)
	require.NoError(t, err)
	tmpl, err := template.New("").Parse(string(valuesBytes))
	require.NoError(t, err)
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, replacements)
	require.NoError(t, err)
	var values map[string]any
	err = yaml.Unmarshal(buf.Bytes(), &values)
	require.NoError(t, err)

	actionConfig := InitHelmActionConfig(t, testKubeConfig)
	install := action.NewInstall(actionConfig)
	install.Namespace = Namespace
	install.ReleaseName = chartReleaseName
	install.Wait = true
	install.Timeout = HelmActionTimeout

	// If UPGRADE_FROM_VALUES env var is set, we install the helm chart using the values. Otherwise, run helm install.
	// UPGRADE_FROM_CHART_DIR is an optional env var that provides an alternative path for the initial helm chart.
	upgradeFromValues := os.Getenv("UPGRADE_FROM_VALUES")
	if upgradeFromValues != "" {
		// install the base chart
		valuesDir := filepath.Dir(valuesFile)
		initValuesBytes, rfErr := os.ReadFile(filepath.Join(valuesDir, upgradeFromValues))
		require.NoError(t, rfErr)
		initChart := loadChartFromDir(t, os.Getenv("UPGRADE_FROM_CHART_DIR"))
		var initValues map[string]any
		require.NoError(t, yaml.Unmarshal(initValuesBytes, &initValues))
		t.Log("Running helm install of the base release")
		_, err = install.Run(initChart, initValues)
		require.NoError(t, err)
		// if install crds option is enabled, try to update the crds
		t.Logf("Checking if CRD installation is enabled in %s", valuesFile)
		if update := crdsInstallEnabled(values); update {
			t.Log("Updating CRDs")
			UpdateOperatorCRDs(t, filepath.Join("..", "..", os.Getenv("UPGRADE_FROM_CHART_DIR")), filepath.Join("..", "..", defaultChartPath), testKubeConfig)
		} else {
			t.Log("Skipping CRDs update")
		}
		// test the upgrade
		upgrade := action.NewUpgrade(actionConfig)
		upgrade.Namespace = Namespace
		upgrade.Wait = true
		upgrade.Timeout = HelmActionTimeout
		t.Log("Running helm upgrade")
		_, err = upgrade.Run(chartReleaseName, loadChart(t), values)
	} else {
		t.Log("Running helm install")
		_, err = install.Run(loadChart(t), values)
	}
	require.NoError(t, err)
}

func ChartUninstall(t *testing.T, testKubeConfig string) {
	uninstall := action.NewUninstall(InitHelmActionConfig(t, testKubeConfig))
	uninstall.IgnoreNotFound = true
	uninstall.Wait = true
	uninstall.Timeout = HelmActionTimeout
	_, _ = uninstall.Run(chartReleaseName)
}

func InitHelmActionConfig(t *testing.T, kubeConfig string) *action.Configuration {
	actionConfig := new(action.Configuration)
	cf := genericclioptions.NewConfigFlags(true)
	cf.Namespace = &Namespace
	cf.KubeConfig = &kubeConfig
	require.NoError(t, actionConfig.Init(cf, Namespace, os.Getenv("HELM_DRIVER"), t.Logf))
	return actionConfig
}

func UpdateOperatorCRDs(t *testing.T, oldChartPath string, newChartPath string, testKubeConfig string) {
	oldCrdsVer := getDependencyVersion(t, "opentelemetry-operator-crds", oldChartPath)
	newCrdsVer := getDependencyVersion(t, "opentelemetry-operator-crds", newChartPath)

	if oldCrdsVer == newCrdsVer {
		t.Logf("CRDs are already up to date: %s", oldCrdsVer)
		return
	}
	t.Logf("Updating CRDs from %s to %s", oldCrdsVer, newCrdsVer)

	cmd := exec.Command("kubectl", "apply", "-f", filepath.Join(newChartPath, "charts", "opentelemetry-operator-crds", "crds")) //nolint:gosec
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", testKubeConfig))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to apply CRDs: %s", string(output))

	t.Logf("Successfully applied CRDs from %s", filepath.Join(newChartPath, "charts", "opentelemetry-operator-crds", "crds"))
}

func loadChart(t *testing.T) *chart.Chart {
	return loadChartFromDir(t, defaultChartPath)
}

func loadChartFromDir(t *testing.T, dir string) *chart.Chart {
	chartPath := filepath.Join("..", "..", dir)
	c, err := loader.Load(chartPath)
	require.NoError(t, err)
	return c
}

func getDependencyVersion(t *testing.T, dependency string, chartPath string) string {
	chartFilePath := filepath.Join(chartPath, "Chart.yaml")
	chartFileContent, err := os.ReadFile(chartFilePath)
	require.NoError(t, err, "Failed to read %s", chartFilePath)

	var chartData map[string]any
	err = yaml.Unmarshal(chartFileContent, &chartData)
	require.NoError(t, err, "Failed to parse %s", chartFilePath)

	dependencies, ok := chartData["dependencies"].([]any)
	require.True(t, ok, "No dependencies found in %s", chartFilePath)

	for _, dep := range dependencies {
		depMap, _ := dep.(map[string]any)
		if depMap["name"] == dependency {
			var version string
			version, ok = depMap["version"].(string)
			require.True(t, ok, "Dependency version not found or invalid")
			return version
		}
	}

	t.Fatalf("Dependency %s not found in %s", dependency, chartFilePath)
	return ""
}

func crdsInstallEnabled(values map[string]any) bool {
	operatorcrds, ok := values["operatorcrds"].(map[string]any)
	if !ok {
		return false
	}

	install, ok := operatorcrds["install"].(bool)
	if !ok {
		return false
	}

	return install
}
