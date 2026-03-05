// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package instrumentationcr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli"
	helmvalues "helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	chartPath          = "helm-charts/splunk-otel-collector"
	testDeploymentName = "injection-test"
)

var instrumentationGVR = schema.GroupVersionResource{
	Group:    "opentelemetry.io",
	Version:  "v1alpha1",
	Resource: "instrumentations",
}

// Values file paths relative to testdata/values/.
const (
	valBase       = "base.yaml"
	valMethodJob  = "method_job.yaml"
	valMethodRes  = "method_resource.yaml"
	valCustomSpec = "custom_spec.yaml"
)

func crName() string {
	return internal.DefaultChartReleaseName + "-splunk-otel-collector"
}

func kubeConfig(t *testing.T) string {
	t.Helper()
	kc := os.Getenv("KUBECONFIG")
	if kc == "" {
		kc = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	return kc
}

func newClients(t *testing.T) (*kubernetes.Clientset, dynamic.Interface) {
	t.Helper()
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfig(t))
	require.NoError(t, err)
	cs, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)
	dyn, err := dynamic.NewForConfig(cfg)
	require.NoError(t, err)
	return cs, dyn
}

func valuePaths(files ...string) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = filepath.Join("testdata", "values", f)
	}
	return paths
}

func loadChart(t *testing.T) chart.Charter {
	t.Helper()
	c, err := loader.Load(filepath.Join("..", "..", chartPath))
	require.NoError(t, err)
	return c
}

func mergeValues(t *testing.T, valueFiles ...string) map[string]any {
	t.Helper()
	vopts := helmvalues.Options{ValueFiles: valuePaths(valueFiles...)}
	vals, err := vopts.MergeValues(getter.All(cli.New()))
	require.NoError(t, err)
	return vals
}

func helmInstall(t *testing.T, valueFiles ...string) {
	t.Helper()
	vals := mergeValues(t, valueFiles...)
	actionConfig := internal.InitHelmActionConfig(t, kubeConfig(t))
	install := action.NewInstall(actionConfig)
	install.Namespace = internal.DefaultNamespace
	install.ReleaseName = internal.DefaultChartReleaseName
	install.WaitStrategy = kube.StatusWatcherStrategy
	install.Timeout = internal.HelmActionTimeout
	install.Labels = map[string]string{"helm.sh/chart-name": internal.DefaultChartReleaseName}
	_, err := install.Run(loadChart(t), vals)
	require.NoError(t, err)
}

func helmUpgrade(t *testing.T, valueFiles ...string) {
	t.Helper()
	vals := mergeValues(t, valueFiles...)
	actionConfig := internal.InitHelmActionConfig(t, kubeConfig(t))
	upgrade := action.NewUpgrade(actionConfig)
	upgrade.Namespace = internal.DefaultNamespace
	upgrade.WaitStrategy = kube.StatusWatcherStrategy
	upgrade.Timeout = internal.HelmActionTimeout
	_, err := upgrade.Run(internal.DefaultChartReleaseName, loadChart(t), vals)
	require.NoError(t, err)
}

func cleanup(t *testing.T, cs *kubernetes.Clientset) {
	t.Helper()
	deleteTestWorkload(t, cs)
	internal.ChartUninstall(t, kubeConfig(t))
}

func waitOperator(t *testing.T, cs *kubernetes.Clientset) {
	t.Helper()
	internal.CheckPodsReady(t, cs, internal.DefaultNamespace, "app.kubernetes.io/name=operator", internal.HelmActionTimeout, 5*time.Second)
}

// waitWebhookEndpoint blocks until the operator's webhook Service has at least
// one ready endpoint address, meaning the webhook port (9443) is reachable.
func waitWebhookEndpoint(t *testing.T, cs *kubernetes.Clientset) {
	t.Helper()
	webhookSvc := internal.DefaultChartReleaseName + "-operator-webhook"
	labelSelector := "kubernetes.io/service-name=" + webhookSvc
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		slices, err := cs.DiscoveryV1().EndpointSlices(internal.DefaultNamespace).List(
			t.Context(), metav1.ListOptions{LabelSelector: labelSelector})
		if !assert.NoError(ct, err) {
			return
		}
		if !assert.NotEmpty(ct, slices.Items, "no EndpointSlice found for %s", webhookSvc) {
			return
		}
		hasReady := false
		for _, slice := range slices.Items {
			for _, ep := range slice.Endpoints {
				if ep.Conditions.Ready != nil && *ep.Conditions.Ready && len(ep.Addresses) > 0 {
					hasReady = true
					break
				}
			}
		}
		assert.True(ct, hasReady, "webhook has no ready endpoint addresses")
	}, 2*time.Minute, 3*time.Second)
}

func helmUpgradeInstall(t *testing.T, valueFiles ...string) {
	t.Helper()
	vals := mergeValues(t, valueFiles...)
	actionConfig := internal.InitHelmActionConfig(t, kubeConfig(t))
	upgrade := action.NewUpgrade(actionConfig)
	upgrade.Install = true
	upgrade.Namespace = internal.DefaultNamespace
	upgrade.WaitStrategy = kube.StatusWatcherStrategy
	upgrade.Timeout = internal.HelmActionTimeout
	_, err := upgrade.Run(internal.DefaultChartReleaseName, loadChart(t), vals)
	require.NoError(t, err)
}

// tryInstallOrRecover attempts a direct helm install. If it fails (e.g. the
// webhook is not ready when the CR is submitted in the same batch), it emits a
// GitHub Actions warning annotation and recovers with helm upgrade --install.
func tryInstallOrRecover(t *testing.T, cs *kubernetes.Clientset, valueFiles ...string) {
	t.Helper()
	vals := mergeValues(t, valueFiles...)

	actionConfig := internal.InitHelmActionConfig(t, kubeConfig(t))
	install := action.NewInstall(actionConfig)
	install.Namespace = internal.DefaultNamespace
	install.ReleaseName = internal.DefaultChartReleaseName
	install.WaitStrategy = kube.StatusWatcherStrategy
	install.Timeout = internal.HelmActionTimeout
	install.Labels = map[string]string{"helm.sh/chart-name": internal.DefaultChartReleaseName}
	_, installErr := install.Run(loadChart(t), vals)
	if installErr == nil {
		return
	}

	//nolint:forbidigo // GitHub Actions annotation for CI visibility
	fmt.Printf("::warning title=CR Install Recovery::"+
		"Initial helm install with resource mode failed: %v. "+
		"Recovering with helm upgrade --install.\n", installErr)
	t.Logf("Initial install failed (non-fatal): %v — recovering with upgrade --install", installErr)

	waitOperator(t, cs)
	waitWebhookEndpoint(t, cs)
	helmUpgradeInstall(t, valueFiles...)
}

// ---------------------------------------------------------------------------
// CR helpers
// ---------------------------------------------------------------------------

func getCR(t *testing.T, dyn dynamic.Interface) *unstructured.Unstructured {
	t.Helper()
	cr, err := dyn.Resource(instrumentationGVR).Namespace(internal.DefaultNamespace).Get(
		t.Context(), crName(), metav1.GetOptions{})
	if err != nil {
		return nil
	}
	return cr
}

func requireCRExists(t *testing.T, dyn dynamic.Interface) *unstructured.Unstructured {
	t.Helper()
	cr := getCR(t, dyn)
	require.NotNil(t, cr, "Instrumentation CR should exist")
	return cr
}

func waitForCR(t *testing.T, dyn dynamic.Interface) *unstructured.Unstructured {
	t.Helper()
	var cr *unstructured.Unstructured
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		cr = getCR(t, dyn)
		assert.NotNil(ct, cr, "waiting for Instrumentation CR to appear")
	}, 3*time.Minute, 5*time.Second)
	return cr
}

func getCRUID(cr *unstructured.Unstructured) types.UID {
	return cr.GetUID()
}

func getCRPropagators(t *testing.T, cr *unstructured.Unstructured) []string {
	t.Helper()
	props, found, err := unstructured.NestedStringSlice(cr.Object, "spec", "propagators")
	require.NoError(t, err)
	require.True(t, found, "spec.propagators should exist")
	return props
}

func getCREndpoint(t *testing.T, cr *unstructured.Unstructured) string {
	t.Helper()
	ep, found, err := unstructured.NestedString(cr.Object, "spec", "exporter", "endpoint")
	require.NoError(t, err)
	require.True(t, found, "spec.exporter.endpoint should exist")
	return ep
}

func ptrBool(b bool) *bool { return &b }

func injectionTestDeployment() *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeploymentName,
			Namespace: internal.DefaultNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": testDeploymentName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": testDeploymentName},
					Annotations: map[string]string{
						"instrumentation.opentelemetry.io/inject-java": "true",
					},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: ptrBool(false),
					Containers: []corev1.Container{{
						Name:    "app",
						Image:   "busybox:1.36",
						Command: []string{"sh", "-c", "sleep 3600"},
					}},
				},
			},
		},
	}
}

func deployTestWorkload(t *testing.T, cs *kubernetes.Clientset) {
	t.Helper()
	_, err := cs.AppsV1().Deployments(internal.DefaultNamespace).Create(
		t.Context(), injectionTestDeployment(), metav1.CreateOptions{})
	require.NoError(t, err)
}

func deleteTestWorkload(t *testing.T, cs *kubernetes.Clientset) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute) //nolint:usetesting
	defer cancel()
	_ = cs.AppsV1().Deployments(internal.DefaultNamespace).Delete(
		ctx, testDeploymentName, metav1.DeleteOptions{})
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		pods, err := cs.CoreV1().Pods(internal.DefaultNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=" + testDeploymentName,
		})
		assert.NoError(ct, err)
		assert.Empty(ct, pods.Items, "waiting for injection-test pods to terminate")
	}, 2*time.Minute, 3*time.Second)
}

// recycleTestWorkload deletes and recreates the test deployment so new pods
// go through admission again with the current Instrumentation CR.
func recycleTestWorkload(t *testing.T, cs *kubernetes.Clientset) {
	t.Helper()
	deleteTestWorkload(t, cs)
	deployTestWorkload(t, cs)
}

func hasInitContainer(pod corev1.Pod, substr string) bool {
	for _, ic := range pod.Spec.InitContainers {
		if strings.Contains(ic.Name, substr) {
			return true
		}
	}
	return false
}

func hasEnvVar(pod corev1.Pod, containerName, envName string) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name != containerName {
			continue
		}
		for _, e := range c.Env {
			if e.Name == envName {
				return true
			}
		}
	}
	return false
}

func initContainerNames(pod corev1.Pod) []string {
	names := make([]string, len(pod.Spec.InitContainers))
	for i, ic := range pod.Spec.InitContainers {
		names[i] = ic.Name
	}
	return names
}

// assertInjectionWorks verifies that the operator's mutating webhook injects
// the Java auto-instrumentation init container and expected OTEL env vars.
func assertInjectionWorks(t *testing.T, cs *kubernetes.Clientset, label string) {
	t.Helper()
	t.Run("injection: init container present ("+label+")", func(t *testing.T) {
		require.EventuallyWithT(t, func(ct *assert.CollectT) {
			pods, err := cs.CoreV1().Pods(internal.DefaultNamespace).List(t.Context(), metav1.ListOptions{
				LabelSelector: "app=" + testDeploymentName,
			})
			if !assert.NoError(ct, err) || !assert.NotEmpty(ct, pods.Items) {
				return
			}
			assert.True(ct, hasInitContainer(pods.Items[0], "opentelemetry-auto-instrumentation"),
				"expected opentelemetry init container, got: %v", initContainerNames(pods.Items[0]))
		}, 3*time.Minute, 5*time.Second)
	})

	t.Run("injection: OTEL env vars present ("+label+")", func(t *testing.T) {
		pods := internal.GetPods(t, cs, internal.DefaultNamespace, "app="+testDeploymentName)
		require.NotEmpty(t, pods.Items)
		pod := pods.Items[0]
		assert.True(t, hasEnvVar(pod, "app", "JAVA_TOOL_OPTIONS"),
			"expected JAVA_TOOL_OPTIONS on app container")
		assert.True(t, hasEnvVar(pod, "app", "OTEL_SERVICE_NAME"),
			"expected OTEL_SERVICE_NAME on app container")
		assert.True(t, hasEnvVar(pod, "app", "OTEL_EXPORTER_OTLP_ENDPOINT"),
			"expected OTEL_EXPORTER_OTLP_ENDPOINT on app container")
	})
}

func TestJobMode(t *testing.T) {
	cs, dyn := newClients(t)
	cleanup(t, cs)
	t.Cleanup(func() { cleanup(t, cs) })

	t.Run("install creates CR via Job", func(t *testing.T) {
		helmInstall(t, valBase, valMethodJob)
		waitOperator(t, cs)
		waitWebhookEndpoint(t, cs)
		cr := waitForCR(t, dyn)
		assert.Equal(t, []string{"tracecontext", "baggage"}, getCRPropagators(t, cr))
		assert.NotEmpty(t, getCREndpoint(t, cr))
	})

	time.Sleep(10 * time.Second)
	deployTestWorkload(t, cs)
	assertInjectionWorks(t, cs, "after install")

	var uid1 types.UID
	t.Run("capture initial UID", func(t *testing.T) {
		uid1 = getCRUID(requireCRExists(t, dyn))
		require.NotEmpty(t, uid1)
	})

	t.Run("upgrade with change patches in-place", func(t *testing.T) {
		helmUpgrade(t, valBase, valMethodJob, valCustomSpec)
		waitWebhookEndpoint(t, cs)
		cr := waitForCR(t, dyn)
		assert.Equal(t, []string{"tracecontext", "baggage", "b3", "ottrace"}, getCRPropagators(t, cr))
		assert.Equal(t, uid1, getCRUID(cr), "job mode should patch in-place (same UID)")
	})

	time.Sleep(10 * time.Second)
	recycleTestWorkload(t, cs)
	assertInjectionWorks(t, cs, "after upgrade")

	t.Run("upgrade without change keeps same UID", func(t *testing.T) {
		helmUpgrade(t, valBase, valMethodJob, valCustomSpec)
		cr := waitForCR(t, dyn)
		assert.Equal(t, uid1, getCRUID(cr), "no-op upgrade should keep same UID")
	})
}

func TestResourceMode(t *testing.T) {
	cs, dyn := newClients(t)
	cleanup(t, cs)
	t.Cleanup(func() { cleanup(t, cs) })

	// Try a single-step install with the CR enabled. This may fail because the
	// webhook is not yet ready when Helm submits the CR in the same batch. If
	// it fails, recover with "helm upgrade --install" (the operator and webhook
	// will already be running by then). A GitHub Actions ::warning:: annotation
	// is emitted so we can track how often the single-step install fails.
	t.Run("install creates CR (with recovery)", func(t *testing.T) {
		tryInstallOrRecover(t, cs, valBase, valMethodRes)
		waitOperator(t, cs)
		waitWebhookEndpoint(t, cs)
		cr := requireCRExists(t, dyn)
		assert.Equal(t, []string{"tracecontext", "baggage"}, getCRPropagators(t, cr))
		assert.NotEmpty(t, getCREndpoint(t, cr))
	})

	time.Sleep(10 * time.Second)
	deployTestWorkload(t, cs)
	assertInjectionWorks(t, cs, "after install")

	var uid1 types.UID
	t.Run("capture UID", func(t *testing.T) {
		uid1 = getCRUID(requireCRExists(t, dyn))
		require.NotEmpty(t, uid1)
	})

	t.Run("upgrade with change patches in-place", func(t *testing.T) {
		helmUpgrade(t, valBase, valMethodRes, valCustomSpec)
		waitWebhookEndpoint(t, cs)
		cr := requireCRExists(t, dyn)
		assert.Equal(t, []string{"tracecontext", "baggage", "b3", "ottrace"}, getCRPropagators(t, cr))
		assert.Equal(t, uid1, getCRUID(cr), "resource mode should patch in-place")
	})

	time.Sleep(10 * time.Second)
	recycleTestWorkload(t, cs)
	assertInjectionWorks(t, cs, "after upgrade")

	t.Run("upgrade without change keeps same UID", func(t *testing.T) {
		helmUpgrade(t, valBase, valMethodRes, valCustomSpec)
		cr := requireCRExists(t, dyn)
		assert.Equal(t, uid1, getCRUID(cr), "no-op upgrade should keep same UID")
	})
}
