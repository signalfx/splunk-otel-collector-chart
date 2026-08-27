// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// MergeHelmValueFiles loads and merges Helm values files.
func MergeHelmValueFiles(t *testing.T, valueFiles []string) map[string]any {
	t.Helper()
	vals, err := mergeHelmValueFiles(valueFiles)
	require.NoError(t, err)
	return vals
}

// HelmInstall installs the chart under test with the selected Helm SDK.
func HelmInstall(t *testing.T, kubeConfig string, values map[string]any, options ChartOptions) error {
	t.Helper()
	return installChartFromDir(t, kubeConfig, defaultChartPath, values, options)
}

// HelmUpgrade upgrades the chart under test with the selected Helm SDK.
func HelmUpgrade(t *testing.T, kubeConfig string, values map[string]any, options ChartOptions) error {
	t.Helper()
	return upgradeChartFromDir(t, kubeConfig, defaultChartPath, values, options)
}

// HelmUpgradeInstall runs upgrade with Install enabled using the selected Helm SDK.
func HelmUpgradeInstall(t *testing.T, kubeConfig string, values map[string]any, options ChartOptions) error {
	t.Helper()
	return upgradeInstallChartFromDir(t, kubeConfig, defaultChartPath, values, options)
}

func parseHelmValueInto(setting string, values map[string]any) error {
	return parseHelmValueStringInto(setting, values)
}

func listChartReleases(t *testing.T, kubeConfig string, labelKey string, labelValue string) []HelmRelease {
	t.Helper()
	list := newHelmListAction(t, kubeConfig)
	annotateListOptions(list, labelKey, labelValue)
	releases, err := runHelmList(list)
	require.NoError(t, err)
	return releases
}

func uninstallChartRelease(t *testing.T, kubeConfig string, releaseName string) (string, error) {
	t.Helper()
	uninstall := newHelmUninstallAction(t, kubeConfig)
	annotateUninstallOptions(uninstall)
	return runHelmUninstall(uninstall, releaseName)
}

func installChartFromDir(t *testing.T, kubeConfig string, chartDir string, values map[string]any, options ChartOptions) error {
	t.Helper()
	install := newHelmInstallAction(t, kubeConfig, chartDir)
	annotateInstallOptions(install, options)
	return runHelmInstall(install, values)
}

func upgradeChartFromDir(t *testing.T, kubeConfig string, chartDir string, values map[string]any, options ChartOptions) error {
	t.Helper()
	upgrade := newHelmUpgradeAction(t, kubeConfig, chartDir)
	annotateUpgradeOptions(upgrade, options)
	return runHelmUpgrade(upgrade, options.ChartReleaseName, values)
}

func upgradeInstallChartFromDir(t *testing.T, kubeConfig string, chartDir string, values map[string]any, options ChartOptions) error {
	t.Helper()
	upgrade := newHelmUpgradeAction(t, kubeConfig, chartDir)
	setUpgradeInstall(upgrade, true)
	annotateUpgradeOptions(upgrade, options)
	return runHelmUpgrade(upgrade, options.ChartReleaseName, values)
}
