// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build !helm3

package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli"
	helmvalues "helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/kube"
	releasei "helm.sh/helm/v4/pkg/release"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/strvals"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type helmInstallAction struct {
	action *action.Install
	chart  chart.Charter
}

func setExtraInstallOpts(action *action.Install, opts ChartOptions) {
	action.WaitStrategy = helmWaitStrategy(opts.WaitStrategy)
	action.ForceConflicts = opts.ForceConflicts
}

type helmUpgradeAction struct {
	action *action.Upgrade
	chart  chart.Charter
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
	action.WaitStrategy = helmWaitStrategy(options.WaitStrategy)
	action.ForceConflicts = options.ForceConflicts
}

func newHelmListAction(t *testing.T, kubeConfig string) *action.List {
	t.Helper()
	return action.NewList(initHelmActionConfig(t, kubeConfig))
}

func setListStateMask(list *action.List) {
	list.StateMask = action.ListAll
}

func helmReleaseInfo(release releasei.Releaser) (HelmRelease, error) {
	r, ok := release.(*releasev1.Release)
	if !ok {
		return HelmRelease{}, fmt.Errorf("expected *releasev1.Release, got %T", release)
	}
	return HelmRelease{Name: r.Name, Namespace: r.Namespace}, nil
}

func newHelmUninstallAction(t *testing.T, kubeConfig string) *action.Uninstall {
	t.Helper()
	return action.NewUninstall(initHelmActionConfig(t, kubeConfig))
}

func setExtraUninstallOpts(uninstall *action.Uninstall) {
	uninstall.WaitStrategy = kube.StatusWatcherStrategy
}

func parseHelmValueStringInto(setting string, values map[string]any) error {
	return strvals.ParseInto(setting, values)
}

func initHelmActionConfig(t *testing.T, kubeConfig string) *action.Configuration {
	t.Helper()
	actionConfig := new(action.Configuration)
	cf := genericclioptions.NewConfigFlags(true)
	cf.Namespace = &DefaultNamespace
	cf.KubeConfig = &kubeConfig
	require.NoError(t, actionConfig.Init(cf, DefaultNamespace, os.Getenv("HELM_DRIVER")))
	return actionConfig
}

func loadChartFromDir(t *testing.T, dir string) chart.Charter {
	t.Helper()
	chartPath := filepath.Join("..", "..", dir)
	c, err := loader.Load(chartPath)
	require.NoError(t, err)
	return c
}

func helmWaitStrategy(strategy HelmWaitStrategy) kube.WaitStrategy {
	if strategy == "" {
		return kube.StatusWatcherStrategy
	}
	return kube.WaitStrategy(strategy)
}
