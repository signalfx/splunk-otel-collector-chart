// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build helm3

package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	helmvalues "helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/strvals"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type helmInstallAction struct {
	action *action.Install
	chart  *chart.Chart
}

func setExtraInstallOpts(action *action.Install, opts ChartOptions) {
	action.Wait = helmWaitEnabled(opts.WaitStrategy)
}

func mergeHelmValueFiles(valueFiles []string) (map[string]any, error) {
	vopts := helmvalues.Options{ValueFiles: valueFiles}
	return vopts.MergeValues(getter.All(cli.New()))
}

func newHelmInstallAction(t *testing.T, kubeConfig string, chartDir string) *helmInstallAction {
	t.Helper()
	return &helmInstallAction{
		action: action.NewInstall(initHelmActionConfig(t, kubeConfig)),
		chart:  loadChartFromDir(t, chartDir),
	}
}

func newHelmUpgradeAction(t *testing.T, kubeConfig string, chartDir string) *helmUpgradeAction {
	t.Helper()
	return &helmUpgradeAction{
		action: action.NewUpgrade(initHelmActionConfig(t, kubeConfig)),
		chart:  loadChartFromDir(t, chartDir),
	}
}

func setExtraUpgradeOpts(action *action.Upgrade, options ChartOptions) {
	action.Wait = helmWaitEnabled(options.WaitStrategy)
}

func newHelmListAction(t *testing.T, kubeConfig string) *action.List {
	t.Helper()
	return action.NewList(initHelmActionConfig(t, kubeConfig))
}

func setListStateMask(list *action.List) {
	list.StateMask = action.ListAll
}

func helmReleaseInfo(release *release.Release) (HelmRelease, error) {
	return HelmRelease{Name: release.Name, Namespace: release.Namespace}, nil
}

func newHelmUninstallAction(t *testing.T, kubeConfig string) *action.Uninstall {
	t.Helper()
	return action.NewUninstall(initHelmActionConfig(t, kubeConfig))
}

func setExtraUninstallOpts(uninstall *action.Uninstall) {
	uninstall.Wait = true
}

func parseHelmValueStringInto(setting string, values map[string]any) error {
	return strvals.ParseInto(setting, values)
}

type helmUpgradeAction struct {
	action *action.Upgrade
	chart  *chart.Chart
}

func initHelmActionConfig(t *testing.T, kubeConfig string) *action.Configuration {
	t.Helper()
	actionConfig := new(action.Configuration)
	cf := genericclioptions.NewConfigFlags(true)
	cf.Namespace = &DefaultNamespace
	cf.KubeConfig = &kubeConfig
	require.NoError(t, actionConfig.Init(cf, DefaultNamespace, os.Getenv("HELM_DRIVER"), t.Logf))
	return actionConfig
}

func loadChartFromDir(t *testing.T, dir string) *chart.Chart {
	t.Helper()
	chartPath := filepath.Join("..", "..", dir)
	c, err := loader.Load(chartPath)
	require.NoError(t, err)
	return c
}

func helmWaitEnabled(strategy HelmWaitStrategy) bool {
	return strategy != ""
}
