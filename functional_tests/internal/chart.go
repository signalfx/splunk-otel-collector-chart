// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	HelmActionTimeout       = 5 * time.Minute
	DefaultChartReleaseName = "sock"
	chartLabelKey           = "helm.sh/chart-name"
	defaultChartPath        = "helm-charts/splunk-otel-collector"
)

type ChartOptions struct {
	ChartNamespace   string
	ChartReleaseName string
	ChartWait        bool
	ChartTimeout     time.Duration
}

func GetDefaultChartOptions() ChartOptions {
	return ChartOptions{
		ChartNamespace:   DefaultNamespace,
		ChartReleaseName: DefaultChartReleaseName,
		ChartWait:        true,
		ChartTimeout:     HelmActionTimeout,
	}
}

func ChartInstallOrUpgrade(t *testing.T, testKubeConfig string, valuesFile string, replacements map[string]any, minReadyTime time.Duration, options ChartOptions) {
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
	install.Namespace = options.ChartNamespace
	install.ReleaseName = options.ChartReleaseName
	install.Wait = options.ChartWait
	install.Timeout = options.ChartTimeout
	install.Labels = map[string]string{chartLabelKey: DefaultChartReleaseName}

	// If UPGRADE_FROM_VALUES env var is set, we install the helm chart using the values. Otherwise, run helm install.
	// UPGRADE_FROM_CHART_DIR is an optional env var that provides an alternative path for the initial helm chart.
	upgradeFromValues := os.Getenv("UPGRADE_FROM_VALUES")
	if upgradeFromValues != "" {
		oldChartDir := os.Getenv("UPGRADE_FROM_CHART_DIR")
		oldChartPath := filepath.Join("..", "..", oldChartDir)
		newChartPath := filepath.Join("..", "..", defaultChartPath)

		valuesDir := filepath.Dir(valuesFile)
		initValuesBytes, rfErr := os.ReadFile(filepath.Join(valuesDir, upgradeFromValues))
		require.NoError(t, rfErr)
		initChart := loadChartFromDir(t, oldChartDir)
		var initValues map[string]any
		require.NoError(t, yaml.Unmarshal(initValuesBytes, &initValues))
		t.Log("Running helm install of the base release")
		_, err2 = install.Run(initChart, initValues)

		cmd := exec.Command("kubectl", "get", "pods", "--all-namespaces")
		cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", testKubeConfig))
		output, _ := cmd.CombinedOutput()
		t.Logf("kubectl get pods --all-namespaces: %s", string(output))

		re := regexp.MustCompile("sock-splunk-otel-collector-agent-\\S*")
		agentPodName := re.FindString(string(output))
		t.Logf("agentPodName: %s", agentPodName)

		cmd = exec.Command("kubectl", "describe", "pods", "-l", "app=splunk-otel-collector")
		cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", testKubeConfig))
		output, _ = cmd.CombinedOutput()
		t.Logf("kubectl describe pods -l app=splunk-otel-collector: %s", string(output))

		cmd = exec.Command("kubectl", "logs", string(agentPodName))
		cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", testKubeConfig))
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Logf("Err: %v", err)
		}
		t.Logf("kubectl logs: %s", string(output))

		require.NoError(t, err2)

		// Helm upgrade does not install or update CRDs, so apply them
		// from the new chart if the CRD version changed.
		if crdsInstallEnabled(values) {
			UpdateOperatorCRDs(t, oldChartPath, newChartPath, testKubeConfig)
		}

		upgrade := action.NewUpgrade(actionConfig)
		upgrade.Namespace = options.ChartNamespace
		upgrade.Wait = options.ChartWait
		upgrade.Timeout = options.ChartTimeout
		t.Log("Running helm upgrade")
		_, err = upgrade.Run(options.ChartReleaseName, loadChart(t), values)
	} else {
		t.Log("Running helm install")
		_, err = install.Run(loadChart(t), values)
	}
	require.NoError(t, err)

	// Wait for pods to be ready for at least minReadyTime
	clientset, err := getKubeClient(testKubeConfig)
	require.NoError(t, err)
	labelSelector := "release=" + options.ChartReleaseName
	CheckPodsReady(t, clientset, options.ChartNamespace, labelSelector, options.ChartTimeout, minReadyTime)
}

func getKubeClient(kubeConfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func deleteCertSecret(t *testing.T, clientset *kubernetes.Clientset, releaseName, namespace string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) //nolint:usetesting // called from t.Cleanup where t.Context is canceled
	defer cancel()
	secretName := releaseName + "-operator-controller-manager-service-cert"
	t.Logf("Attempting to delete secret: %s in namespace: %s", secretName, namespace)
	_, getErr := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, v1.GetOptions{})
	if getErr == nil {
		deleteErr := clientset.CoreV1().Secrets(namespace).Delete(ctx, secretName, v1.DeleteOptions{})
		require.NoError(t, deleteErr)
		t.Logf("Deleted webhook secret: %s (namespace: %s)", secretName, namespace)
	} else {
		t.Logf("Secret %s not found in namespace: %s, nothing to delete", secretName, namespace)
	}
}

func ChartUninstall(t *testing.T, testKubeConfig string) {
	actionConfig := InitHelmActionConfig(t, testKubeConfig)
	client := action.NewList(actionConfig)
	client.AllNamespaces = true
	client.Selector = fmt.Sprintf("%s==%s", chartLabelKey, DefaultChartReleaseName)
	client.StateMask = action.ListAll // Include releases in all states
	releases, err := client.Run()
	require.NoError(t, err)
	clientset, err := getKubeClient(testKubeConfig)
	require.NoError(t, err)

	if len(releases) == 0 {
		t.Log("No Helm releases found for uninstall.")
		deleteCertSecret(t, clientset, DefaultChartReleaseName, DefaultNamespace)
		deleteOperatorCRDs(t, testKubeConfig)
		return
	}

	uninstall := action.NewUninstall(actionConfig)
	uninstall.IgnoreNotFound = true
	uninstall.Wait = true
	uninstall.Timeout = HelmActionTimeout
	for _, release := range releases {
		t.Logf("Uninstalling release: %s (namespace: %s)", release.Name, release.Namespace)
		_, _ = uninstall.Run(release.Name)
		deleteCertSecret(t, clientset, release.Name, release.Namespace)
	}

	deleteOperatorCRDs(t, testKubeConfig)
}

func deleteOperatorCRDs(t *testing.T, testKubeConfig string) {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	if err != nil {
		t.Logf("Failed to build kube config for CRD cleanup: %v", err)
		return
	}
	crdClient, err := apiextensionsclient.NewForConfig(kubeConfig)
	if err != nil {
		t.Logf("Failed to create apiextensions client for CRD cleanup: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute) //nolint:usetesting // called from t.Cleanup where t.Context is canceled
	defer cancel()
	crdList, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, v1.ListOptions{})
	if err != nil {
		t.Logf("Failed to list CRDs: %v", err)
		return
	}

	var deleted []string
	for _, crd := range crdList.Items {
		if crd.Spec.Group != "opentelemetry.io" {
			continue
		}
		delErr := crdClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, crd.Name, v1.DeleteOptions{})
		if k8serrors.IsNotFound(delErr) {
			t.Logf("CRD %s already absent, skipping", crd.Name)
			continue
		}
		if delErr != nil {
			t.Logf("CRD %s not deleted: %v", crd.Name, delErr)
			continue
		}
		t.Logf("Deleted CRD: %s, waiting for removal...", crd.Name)
		deleted = append(deleted, crd.Name)
	}

	if len(deleted) == 0 {
		t.Log("No opentelemetry.io CRDs found to delete")
		return
	}

	crdAPI := crdClient.ApiextensionsV1().CustomResourceDefinitions()
	for _, name := range deleted {
		require.Eventually(t, func() bool {
			_, getErr := crdAPI.Get(ctx, name, v1.GetOptions{})
			return k8serrors.IsNotFound(getErr)
		}, 3*time.Minute, 3*time.Second, "CRD %s was not removed in time", name)
		t.Logf("CRD %s fully removed", name)
	}
}

func InitHelmActionConfig(t *testing.T, kubeConfig string) *action.Configuration {
	actionConfig := new(action.Configuration)
	cf := genericclioptions.NewConfigFlags(true)
	cf.Namespace = &DefaultNamespace
	cf.KubeConfig = &kubeConfig
	require.NoError(t, actionConfig.Init(cf, DefaultNamespace, os.Getenv("HELM_DRIVER"), t.Logf))
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

	crdsDir := filepath.Join(newChartPath, "charts", "opentelemetry-operator-crds", "crds")
	cmd := exec.Command("kubectl", "apply", "-f", crdsDir)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", testKubeConfig))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to apply CRDs: %s", string(output))
	t.Logf("Successfully applied CRDs from %s", crdsDir)
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
