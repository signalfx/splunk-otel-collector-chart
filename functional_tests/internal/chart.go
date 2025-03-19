// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
)

const (
	helmActionTimeout = 5 * time.Minute
	chartReleaseName  = "sock"
)

func ChartInstallOrUpgrade(t *testing.T, testKubeConfig string, valuesFile string, replacements map[string]any) {
	valuesBytes, err := os.ReadFile(valuesFile)
	require.NoError(t, err)
	tmpl, err := template.New("").Parse(string(valuesBytes))
	require.NoError(t, err)
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, replacements)
	require.NoError(t, err)
	var values map[string]interface{}
	err = yaml.Unmarshal(buf.Bytes(), &values)
	require.NoError(t, err)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", Namespace), Namespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format+"\n", v...)
	}); err != nil {
		require.NoError(t, err)
	}

	install := action.NewInstall(actionConfig)
	install.Namespace = Namespace
	install.ReleaseName = chartReleaseName
	install.Wait = true
	install.Timeout = helmActionTimeout

	// If UPGRADE_FROM_VALUES env var is set, we install the helm chart using the values. Otherwise, run helm install.
	// UPGRADE_FROM_CHART_DIR is an optional env var that provides an alternative path for the initial helm chart.
	upgradeFromValues := os.Getenv("UPGRADE_FROM_VALUES")
	if upgradeFromValues != "" {
		// install the base chart
		valuesDir := filepath.Dir(valuesFile)
		initValuesBytes, rfErr := os.ReadFile(filepath.Join(valuesDir, upgradeFromValues))
		require.NoError(t, rfErr)
		initChart := loadChartFromDir(t, os.Getenv("UPGRADE_FROM_CHART_DIR"))
		var initValues map[string]interface{}
		require.NoError(t, yaml.Unmarshal(initValuesBytes, &initValues))
		t.Log("Running helm install of the base release")
		_, err = install.Run(initChart, initValues)
		require.NoError(t, err)

		// test the upgrade
		upgrade := action.NewUpgrade(actionConfig)
		upgrade.Namespace = Namespace
		upgrade.Wait = true
		upgrade.Timeout = helmActionTimeout
		t.Log("Running helm upgrade")
		_, err = upgrade.Run(chartReleaseName, loadChart(t), values)
	} else {
		t.Log("Running helm install")
		_, err = install.Run(loadChart(t), values)
	}
	require.NoError(t, err)
}

func ChartUninstall(t *testing.T, testKubeConfig string) {
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", Namespace), Namespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format+"\n", v)
	}); err != nil {
		require.NoError(t, err)
	}
	uninstall := action.NewUninstall(actionConfig)
	uninstall.IgnoreNotFound = true
	uninstall.Wait = true
	uninstall.Timeout = helmActionTimeout
	_, _ = uninstall.Run(chartReleaseName)
}

func loadChart(t *testing.T) *chart.Chart {
	return loadChartFromDir(t, "helm-charts/splunk-otel-collector")
}

func loadChartFromDir(t *testing.T, dir string) *chart.Chart {
	chartPath := filepath.Join("..", "..", dir)
	c, err := loader.Load(chartPath)
	require.NoError(t, err)
	return c
}
