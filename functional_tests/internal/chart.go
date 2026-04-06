// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/kube"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	HelmActionTimeout       = 15 * time.Minute
	DefaultChartReleaseName = "sock"
	chartLabelKey           = "helm.sh/chart-name"
	defaultChartPath        = "helm-charts/splunk-otel-collector"
)

type ChartOptions struct {
	ChartNamespace      string
	ChartReleaseName    string
	WaitStrategy        kube.WaitStrategy
	ChartTimeout        time.Duration
	ForceConflicts      bool
	UpgradeFromValues   string
	UpgradeFromChartDir string
}

func GetDefaultChartOptions() ChartOptions {
	return ChartOptions{
		ChartNamespace:   DefaultNamespace,
		ChartReleaseName: DefaultChartReleaseName,
		WaitStrategy:     kube.StatusWatcherStrategy,
		ChartTimeout:     HelmActionTimeout,
		ForceConflicts:   false,
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
	install.WaitStrategy = options.WaitStrategy
	install.Timeout = options.ChartTimeout
	install.ForceConflicts = options.ForceConflicts
	install.Labels = map[string]string{chartLabelKey: DefaultChartReleaseName}

	// Determine upgrade-from values: prefer ChartOptions fields, fall back to env vars.
	upgradeFromValues := options.UpgradeFromValues
	if upgradeFromValues == "" {
		upgradeFromValues = os.Getenv("UPGRADE_FROM_VALUES")
	}
	if upgradeFromValues != "" {
		oldChartDir := options.UpgradeFromChartDir
		if oldChartDir == "" {
			oldChartDir = os.Getenv("UPGRADE_FROM_CHART_DIR")
		}
		oldChartPath := filepath.Join("..", "..", oldChartDir)
		newChartPath := filepath.Join("..", "..", defaultChartPath)

		valuesDir := filepath.Dir(valuesFile)
		initValuesBytes, rfErr := os.ReadFile(filepath.Join(valuesDir, upgradeFromValues))
		require.NoError(t, rfErr)
		initChart := loadChartFromDir(t, oldChartDir)
		var initValues map[string]any
		require.NoError(t, yaml.Unmarshal(initValuesBytes, &initValues))
		t.Log("Running helm install of the base release")
		_, err = install.Run(initChart, initValues)
		require.NoError(t, err)

		// Helm upgrade does not install or update CRDs, so apply them
		// from the new chart if the CRD version changed.
		if crdsInstallEnabled(values) {
			UpdateOperatorCRDs(t, oldChartPath, newChartPath, testKubeConfig)
		}

		upgrade := action.NewUpgrade(actionConfig)
		upgrade.Namespace = options.ChartNamespace
		upgrade.WaitStrategy = options.WaitStrategy
		upgrade.Timeout = options.ChartTimeout
		upgrade.ForceConflicts = options.ForceConflicts
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
	err := clientset.CoreV1().Secrets(namespace).Delete(ctx, secretName, v1.DeleteOptions{})
	switch {
	case k8serrors.IsNotFound(err):
		t.Logf("Secret %s not found in namespace: %s, nothing to delete", secretName, namespace)
	case err != nil:
		require.NoError(t, err)
	default:
		t.Logf("Deleted webhook secret: %s (namespace: %s)", secretName, namespace)
	}
}

func ChartUninstall(t *testing.T, testKubeConfig string) {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)
	crdClient, err := apiextensionsclient.NewForConfig(kubeConfig)
	require.NoError(t, err)
	dynClient, err := dynamic.NewForConfig(kubeConfig)
	require.NoError(t, err)

	actionConfig := InitHelmActionConfig(t, testKubeConfig)
	client := action.NewList(actionConfig)
	client.AllNamespaces = true
	client.Selector = fmt.Sprintf("%s==%s", chartLabelKey, DefaultChartReleaseName)
	client.StateMask = action.ListAll // Include releases in all states
	releases, err := client.Run()
	require.NoError(t, err)

	// Delete CRs before helm uninstall. It will strip finalizers if the CR is not deleted in 1min.
	deleteOperatorCRs(t, crdClient, dynClient)

	if len(releases) == 0 {
		t.Log("No Helm releases found for uninstall.")
		deleteCertSecret(t, clientset, DefaultChartReleaseName, DefaultNamespace)
		deleteOperatorCRDs(t, crdClient)
		return
	}

	uninstall := action.NewUninstall(actionConfig)
	uninstall.IgnoreNotFound = true
	uninstall.WaitStrategy = kube.StatusWatcherStrategy
	uninstall.Timeout = HelmActionTimeout
	for _, rel := range releases {
		r, ok := rel.(*releasev1.Release)
		require.Truef(t, ok, "expected *releasev1.Release, got %T", rel)
		t.Logf("Uninstalling release: %s (namespace: %s)", r.Name, r.Namespace)
		_, _ = uninstall.Run(r.Name)
		deleteCertSecret(t, clientset, r.Name, r.Namespace)
	}

	deleteOperatorCRDs(t, crdClient)
}

// otelGVRs returns the GVR for each opentelemetry.io CRD (first served version).
func otelGVRs(ctx context.Context, crdClient apiextensionsclient.Interface) ([]schema.GroupVersionResource, error) {
	crdList, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var gvrs []schema.GroupVersionResource
	for _, crd := range crdList.Items {
		if crd.Spec.Group != "opentelemetry.io" {
			continue
		}
		for _, ver := range crd.Spec.Versions {
			if ver.Served {
				gvrs = append(gvrs, schema.GroupVersionResource{
					Group:    crd.Spec.Group,
					Version:  ver.Name,
					Resource: crd.Spec.Names.Plural,
				})
				break
			}
		}
	}
	return gvrs, nil
}

// stripFinalizers patches all CRs for the given GVR to remove finalizers.
func stripFinalizers(ctx context.Context, t *testing.T, dynClient dynamic.Interface, gvr schema.GroupVersionResource) {
	patch, _ := json.Marshal(map[string]any{"metadata": map[string]any{"finalizers": nil}})
	list, err := dynClient.Resource(gvr).List(ctx, v1.ListOptions{})
	if err != nil || len(list.Items) == 0 {
		return
	}
	for i := range list.Items {
		cr := &list.Items[i]
		if len(cr.GetFinalizers()) == 0 {
			continue
		}
		_, patchErr := dynClient.Resource(gvr).Namespace(cr.GetNamespace()).Patch(
			ctx, cr.GetName(), types.MergePatchType, patch, v1.PatchOptions{})
		if patchErr != nil {
			t.Logf("Failed to strip finalizers from %s/%s (ns: %s): %v", gvr.Resource, cr.GetName(), cr.GetNamespace(), patchErr)
		} else {
			t.Logf("Stripped finalizers from %s/%s (ns: %s)", gvr.Resource, cr.GetName(), cr.GetNamespace())
		}
	}
}

// deleteOperatorCRs deletes all opentelemetry.io CRs across all namespaces.
// It first requests deletion and waits for the operator to process finalizers.
// If CRs are still stuck after 1 minute, it strips finalizers as a fallback.
func deleteOperatorCRs(t *testing.T, crdClient apiextensionsclient.Interface, dynClient dynamic.Interface) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute) //nolint:usetesting // called from t.Cleanup where t.Context is canceled
	defer cancel()

	gvrs, err := otelGVRs(ctx, crdClient)
	if err != nil {
		t.Logf("Failed to list CRDs for CR cleanup: %v", err)
		return
	}
	if len(gvrs) == 0 {
		return
	}

	for _, gvr := range gvrs {
		list, listErr := dynClient.Resource(gvr).List(ctx, v1.ListOptions{})
		if listErr != nil {
			t.Logf("Failed to list %s CRs: %v", gvr.Resource, listErr)
			continue
		}
		t.Logf("Found %d %s CRs across all namespaces", len(list.Items), gvr.Resource)
		for i := range list.Items {
			cr := &list.Items[i]
			t.Logf("Deleting %s %s/%s (ns: %s, finalizers: %v)",
				cr.GetKind(), gvr.Resource, cr.GetName(), cr.GetNamespace(), cr.GetFinalizers())
			delErr := dynClient.Resource(gvr).Namespace(cr.GetNamespace()).Delete(ctx, cr.GetName(), v1.DeleteOptions{})
			if delErr != nil && !k8serrors.IsNotFound(delErr) {
				t.Logf("Failed to delete %s/%s (ns: %s): %v", gvr.Resource, cr.GetName(), cr.GetNamespace(), delErr)
			}
		}
	}

	// Wait for operator to process finalizers and fully remove CRs.
	allGone := true
	for _, gvr := range gvrs {
		gone := assert.Eventually(t, func() bool {
			pollCtx, pollCancel := context.WithTimeout(context.Background(), 5*time.Second) //nolint:usetesting
			defer pollCancel()
			list, listErr := dynClient.Resource(gvr).List(pollCtx, v1.ListOptions{})
			if listErr != nil {
				return false
			}
			return len(list.Items) == 0
		}, 1*time.Minute, 2*time.Second, "CRs for %s not removed by operator in time", gvr.Resource)
		if gone {
			t.Logf("All %s CRs removed by operator", gvr.Resource)
		} else {
			allGone = false
		}
	}
	if allGone {
		return
	}

	// Fallback: strip finalizers from any remaining CRs.
	t.Log("Operator did not remove all CRs in time, stripping finalizers")
	for _, gvr := range gvrs {
		stripFinalizers(ctx, t, dynClient, gvr)
	}

	for _, gvr := range gvrs {
		require.Eventually(t, func() bool {
			pollCtx, pollCancel := context.WithTimeout(context.Background(), 5*time.Second) //nolint:usetesting
			defer pollCancel()
			list, listErr := dynClient.Resource(gvr).List(pollCtx, v1.ListOptions{})
			if listErr != nil {
				return false
			}
			return len(list.Items) == 0
		}, 1*time.Minute, 2*time.Second, "CRs for %s still present after stripping finalizers", gvr.Resource)
		t.Logf("All %s CRs removed after stripping finalizers", gvr.Resource)
	}
}

func deleteOperatorCRDs(t *testing.T, crdClient apiextensionsclient.Interface) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute) //nolint:usetesting // called from t.Cleanup where t.Context is canceled
	defer cancel()

	gvrs, err := otelGVRs(ctx, crdClient)
	if err != nil {
		t.Logf("Failed to list CRDs for CRD cleanup: %v", err)
		return
	}
	if len(gvrs) == 0 {
		t.Log("No opentelemetry.io CRDs found to delete")
		return
	}

	crdAPI := crdClient.ApiextensionsV1().CustomResourceDefinitions()
	var deleted []string
	for _, gvr := range gvrs {
		crdName := gvr.Resource + "." + gvr.Group
		delErr := crdAPI.Delete(ctx, crdName, v1.DeleteOptions{})
		if k8serrors.IsNotFound(delErr) {
			t.Logf("CRD %s already absent", crdName)
			continue
		}
		if delErr != nil {
			t.Logf("CRD %s not deleted: %v", crdName, delErr)
			continue
		}
		t.Logf("Deleted CRD %s, waiting for removal...", crdName)
		deleted = append(deleted, crdName)
	}

	for _, name := range deleted {
		require.Eventually(t, func() bool {
			pollCtx, pollCancel := context.WithTimeout(context.Background(), 5*time.Second) //nolint:usetesting
			defer pollCancel()
			crd, getErr := crdAPI.Get(pollCtx, name, v1.GetOptions{})
			if k8serrors.IsNotFound(getErr) {
				return true
			}
			if getErr != nil {
				t.Logf("CRD %s poll error: %v", name, getErr)
				return false
			}
			for _, c := range crd.Status.Conditions {
				if c.Type == "Terminating" {
					t.Logf("CRD %s still terminating: %s", name, c.Message)
				}
			}
			return false
		}, 5*time.Minute, 3*time.Second, "CRD %s was not removed in time", name)
		t.Logf("CRD %s fully removed", name)
	}
}

func InitHelmActionConfig(t *testing.T, kubeConfig string) *action.Configuration {
	actionConfig := new(action.Configuration)
	cf := genericclioptions.NewConfigFlags(true)
	cf.Namespace = &DefaultNamespace
	cf.KubeConfig = &kubeConfig
	require.NoError(t, actionConfig.Init(cf, DefaultNamespace, os.Getenv("HELM_DRIVER")))
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

func loadChart(t *testing.T) chart.Charter {
	return loadChartFromDir(t, defaultChartPath)
}

func loadChartFromDir(t *testing.T, dir string) chart.Charter {
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
