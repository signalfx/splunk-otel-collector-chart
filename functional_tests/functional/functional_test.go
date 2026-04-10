// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"helm.sh/helm/v4/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	appextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	signalFxReceiverK8sClusterReceiverPort = 19443
	kindTestKubeEnv                        = "kind"
	eksTestKubeEnv                         = "eks"
	eksAutoModeTestKubeEnv                 = "eks/auto-mode"
	eksFargateTestKubeEnv                  = "eks/fargate"
	gkeTestKubeEnv                         = "gke"
	autopilotTestKubeEnv                   = "gke/autopilot"
	aksTestKubeEnv                         = "aks"
	rosaTestKubeEnv                        = "rosa"
	gceTestKubeEnv                         = "gce"
	testDir                                = "testdata"
	valuesDir                              = "values"
	manifestsDir                           = "manifests"
	kindValuesDir                          = "expected_kind_values"
	eksValuesDir                           = "expected_eks_values"
	eksAutoModeValuesDir                   = "expected_eks_auto_mode_values"
	aksValuesDir                           = "expected_aks_values"
	gkeValuesDir                           = "expected_gke_values"
	rosaValuesDir                          = "expected_rosa_values"
	gceValuesDir                           = "expected_gce_values"
	clusterReceiverLabelSelector           = "component=otel-k8s-cluster-receiver"
	linuxPodMetricsPath                    = "/tmp/metrics.json"
	winPodMetricsPath                      = "C:\\metrics.json"
	linuxPodK8sClusterMetricsPath          = "/tmp/k8s_cluster_metrics.json"
	winPodK8sClusterMetricsPath            = "C:\\k8s_cluster_metrics.json"
)

type collectorRole string

const (
	roleAgent              collectorRole = "agent"
	roleClusterReceiver    collectorRole = "cluster_receiver"
	roleClusterReceiverK8s collectorRole = "cluster_receiver_k8s_cluster"
)

var archRe = regexp.MustCompile("-amd64$|-arm64$|-ppc64le$")

var globalSinks *sinks

var expectedValuesDir string

// Component names for health checks
const (
	kubeletstatsReceiverName = "kubeletstatsreceiver"
	k8sClusterReceiverName   = "k8sclusterreceiver"
	journaldReceiverName     = "journald"
)

type sinks struct {
	logsConsumer                      *consumertest.LogsSink
	hecMetricsConsumer                *consumertest.MetricsSink
	logsObjectsConsumer               *consumertest.LogsSink
	agentMetricsConsumer              *consumertest.MetricsSink
	k8sclusterReceiverMetricsConsumer *consumertest.MetricsSink
	tracesConsumer                    *consumertest.TracesSink
}

func setupSinks(t *testing.T) {
	// create an API server
	internal.SetupSignalFxAPIServer(t)
	globalSinks = &sinks{
		logsConsumer:         internal.SetupHECLogsSink(t),
		hecMetricsConsumer:   internal.SetupHECMetricsSink(t),
		logsObjectsConsumer:  internal.SetupHECObjectsSink(t),
		agentMetricsConsumer: internal.SetupSignalfxReceiver(t, internal.SignalFxReceiverPort),
		k8sclusterReceiverMetricsConsumer: internal.SetupSignalfxReceiver(t,
			signalFxReceiverK8sClusterReceiverPort),
		tracesConsumer: internal.SetupOTLPTracesSink(t),
	}
}

func requiresPrometheusCRD(kubeTestEnv string) bool {
	return kubeTestEnv == kindTestKubeEnv
}

func deployPrometheusResources(t *testing.T, extensionsClient *clientset.Clientset, dynamicClient dynamic.Interface) {
	decode := scheme.Codecs.UniversalDeserializer().Decode

	// load up Prometheus PodMonitor and ServiceMonitor CRDs:
	stream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "prometheus_operator_crds.yaml"))
	require.NoError(t, err)
	sch := k8sruntime.NewScheme()

	var obj k8sruntime.Object
	var groupVersionKind *schema.GroupVersionKind
	for _, resourceYAML := range strings.Split(string(stream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		obj, groupVersionKind, err = decode(
			[]byte(resourceYAML),
			nil,
			nil)
		require.NoError(t, err)
		if groupVersionKind.Group == "apiextensions.k8s.io" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "CustomResourceDefinition" {
			crd := obj.(*appextensionsv1.CustomResourceDefinition)
			apiExtensions := extensionsClient.ApiextensionsV1().CustomResourceDefinitions()

			existing, getErr := apiExtensions.Get(t.Context(), crd.Name, metav1.GetOptions{})
			if getErr == nil && existing.DeletionTimestamp != nil {
				t.Logf("CRD %s is terminating, waiting for removal...", crd.Name)
				require.EventuallyWithT(t, func(tt *assert.CollectT) {
					_, e := apiExtensions.Get(t.Context(), crd.Name, metav1.GetOptions{})
					assert.True(tt, k8serrors.IsNotFound(e), "CRD %s still exists", crd.Name)
				}, 3*time.Minute, 3*time.Second, "CRD %s stuck in Terminating", crd.Name)
			}

			crdName := crd.Name
			crdSpec := crd.Spec
			created, createErr := apiExtensions.Create(t.Context(), crd, metav1.CreateOptions{})
			if createErr != nil {
				if k8serrors.IsAlreadyExists(createErr) {
					t.Logf("CRD %s already exists, skipping creation", crdName)
				} else {
					require.NoError(t, createErr)
				}
			} else {
				t.Logf("Deployed CRD %s", created.Name)
			}

			require.EventuallyWithT(t, func(tt *assert.CollectT) {
				latest, latestErr := apiExtensions.Get(t.Context(), crdName, metav1.GetOptions{})
				assert.NoError(tt, latestErr)
				established := false
				for _, cond := range latest.Status.Conditions {
					if cond.Type == appextensionsv1.Established && cond.Status == appextensionsv1.ConditionTrue {
						established = true
					}
				}
				assert.True(tt, established)
			}, 3*time.Minute, 3*time.Second, "CRD %s not established", crdName)

			for _, version := range crdSpec.Versions {
				sch.AddKnownTypeWithName(
					schema.GroupVersionKind{
						Group:   crdSpec.Group,
						Version: version.Name,
						Kind:    crdSpec.Names.Kind,
					},
					&unstructured.Unstructured{},
				)
			}
		}
	}

	codecs := serializer.NewCodecFactory(sch)
	crdDecode := codecs.UniversalDeserializer().Decode

	// Prometheus pod monitor
	stream, err = os.ReadFile(filepath.Join(testDir, manifestsDir, "pod_monitor.yaml"))
	require.NoError(t, err)
	podMonitor, _, err := crdDecode(stream, nil, nil)
	require.NoError(t, err)
	g := schema.GroupVersionResource{
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "podmonitors",
	}
	_, err = dynamicClient.Resource(g).Namespace(internal.DefaultNamespace).Create(t.Context(),
		podMonitor.(*unstructured.Unstructured), metav1.CreateOptions{})
	assert.NoError(t, err)

	// Prometheus service monitor
	stream, err = os.ReadFile(filepath.Join(testDir, manifestsDir, "service_monitor.yaml"))
	require.NoError(t, err)
	serviceMonitor, _, err := crdDecode(stream, nil, nil)
	require.NoError(t, err)
	g = schema.GroupVersionResource{
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "servicemonitors",
	}
	_, err = dynamicClient.Resource(g).Namespace(internal.DefaultNamespace).Create(t.Context(),
		serviceMonitor.(*unstructured.Unstructured), metav1.CreateOptions{})
	assert.NoError(t, err)
}

func teardownPrometheusResources(ctx context.Context, t *testing.T, extensionsClient *clientset.Clientset) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	waitTime := int64(0)
	crdstream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "prometheus_operator_crds.yaml"))
	require.NoError(t, err)
	var obj k8sruntime.Object
	var groupVersionKind *schema.GroupVersionKind
	for _, resourceYAML := range strings.Split(string(crdstream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		obj, groupVersionKind, err = decode(
			[]byte(resourceYAML),
			nil,
			nil)
		require.NoError(t, err)
		if groupVersionKind.Group == "apiextensions.k8s.io" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "CustomResourceDefinition" {
			crd := obj.(*appextensionsv1.CustomResourceDefinition)
			apiExtensions := extensionsClient.ApiextensionsV1().CustomResourceDefinitions()
			err = apiExtensions.Delete(ctx, crd.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &waitTime,
			})
			if err != nil && !k8serrors.IsNotFound(err) {
				t.Logf("Failed to delete CRD %s during teardown: %v", crd.Name, err)
			}
		}
	}
}

func deployChartsAndApps(t *testing.T, testKubeConfig string) {
	kubeTestEnv, setKubeTestEnv := os.LookupEnv("KUBE_TEST_ENV")
	require.True(t, setKubeTestEnv, "the environment variable KUBE_TEST_ENV must be set")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)
	extensionsClient, err := clientset.NewForConfig(kubeConfig)
	require.NoError(t, err)
	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	require.NoError(t, err)
	decode := scheme.Codecs.UniversalDeserializer().Decode

	if requiresPrometheusCRD(kubeTestEnv) {
		deployPrometheusResources(t, extensionsClient, dynamicClient)
	}

	var stream []byte
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
	assert.NoError(t, err)

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

	deployments := client.AppsV1().Deployments(internal.DefaultNamespace)

	deployApp := func(filePath string) {
		data, readErr := os.ReadFile(filePath)
		require.NoError(t, readErr)
		dep, _, decodeErr := decode(data, nil, nil)
		require.NoError(t, decodeErr)
		_, createErr := deployments.Create(t.Context(), dep.(*appsv1.Deployment), metav1.CreateOptions{})
		if k8serrors.IsAlreadyExists(createErr) {
			_, updateErr := deployments.Update(t.Context(), dep.(*appsv1.Deployment), metav1.UpdateOptions{})
			require.NoError(t, updateErr)
		} else {
			require.NoError(t, createErr)
		}
	}
	for _, f := range []string{
		filepath.Join(testDir, "nodejs", "deployment.yaml"),
		filepath.Join(testDir, "java", "deployment.yaml"),
		filepath.Join(testDir, "dotnet", "deployment.yaml"),
		filepath.Join(testDir, "python", "deployment.yaml"),
		filepath.Join(testDir, manifestsDir, "log_attr_test_deployment.yaml"),
		filepath.Join(testDir, manifestsDir, "deployment_with_prometheus_annotations.yaml"),
	} {
		deployApp(f)
	}

	// Service
	stream, err = os.ReadFile(filepath.Join(testDir, manifestsDir, "service.yaml"))
	require.NoError(t, err)
	service, _, err := decode(stream, nil, nil)
	require.NoError(t, err)
	_, err = client.CoreV1().Services(internal.DefaultNamespace).Create(t.Context(), service.(*corev1.Service),
		metav1.CreateOptions{})
	require.NoError(t, err)

	var obj k8sruntime.Object
	var groupVersionKind *schema.GroupVersionKind

	// Read jobs
	jobstream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "test_jobs.yaml"))
	require.NoError(t, err)
	for _, resourceYAML := range strings.Split(string(jobstream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		obj, groupVersionKind, err = decode(
			[]byte(resourceYAML),
			nil,
			nil)
		require.NoError(t, err)
		if groupVersionKind.Group == "" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "Namespace" {
			nm := obj.(*corev1.Namespace)
			nms := client.CoreV1().Namespaces()
			_, err = nms.Create(t.Context(), nm, metav1.CreateOptions{})
			require.NoError(t, err)
			t.Logf("Deployed namespace %s", nm.Name)
		}
	}

	waitForAllNamespacesToBeCreated(t, client)

	for _, resourceYAML := range strings.Split(string(jobstream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		obj, groupVersionKind, err = decode(
			[]byte(resourceYAML),
			nil,
			nil)
		require.NoError(t, err)

		if groupVersionKind.Group == "batch" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "Job" {
			job := obj.(*batchv1.Job)
			jobClient := client.BatchV1().Jobs(job.Namespace)
			_, err = jobClient.Create(t.Context(), job, metav1.CreateOptions{})
			require.NoError(t, err)
			t.Logf("Deployed job %s", job.Name)
		}
	}

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		t.Log("Cleaning up cluster")
		// t.Cleanup is called after t.Context has been cancelled. teardown uses the passed in
		// context to cleanup resources created for the test. A valid context needs to be used
		// to properly delete k8s resources, otherwise all actions fail with a context
		// cancelled error.
		teardown(context.Background(), t, testKubeConfig) //nolint:usetesting
	})
}

func teardown(ctx context.Context, t *testing.T, testKubeConfig string) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)
	extensionsClient, err := clientset.NewForConfig(kubeConfig)
	require.NoError(t, err)
	waitTime := int64(0)
	deployments := client.AppsV1().Deployments(internal.DefaultNamespace)
	require.NoError(t, err)
	_ = deployments.Delete(ctx, "nodejs-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = deployments.Delete(ctx, "java-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = deployments.Delete(ctx, "python-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = deployments.Delete(ctx, "dotnet-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = deployments.Delete(ctx, "prometheus-annotation-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = deployments.Delete(ctx, "log-attr-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = client.CoreV1().Services(internal.DefaultNamespace).Delete(ctx, "prometheus-annotation-service",
		metav1.DeleteOptions{
			GracePeriodSeconds: &waitTime,
		})

	var groupVersionKind *schema.GroupVersionKind
	var obj k8sruntime.Object

	jobstream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "test_jobs.yaml"))
	require.NoError(t, err)
	var namespaces []*corev1.Namespace
	var jobs []*batchv1.Job
	for _, resourceYAML := range strings.Split(string(jobstream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		obj, groupVersionKind, err = decode(
			[]byte(resourceYAML),
			nil,
			nil)
		require.NoError(t, err)
		if groupVersionKind.Group == "" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "Namespace" {
			nm := obj.(*corev1.Namespace)
			namespaces = append(namespaces, nm)
		}

		if groupVersionKind.Group == "batch" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "Job" {
			job := obj.(*batchv1.Job)
			jobs = append(jobs, job)
		}
	}
	for _, job := range jobs {
		jobClient := client.BatchV1().Jobs(job.Namespace)
		_ = jobClient.Delete(ctx, job.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &waitTime,
		})
	}

	if requiresPrometheusCRD(os.Getenv("KUBE_TEST_ENV")) {
		teardownPrometheusResources(ctx, t, extensionsClient)
	}

	for _, nm := range namespaces {
		nmClient := client.CoreV1().Namespaces()
		_ = nmClient.Delete(ctx, nm.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &waitTime,
		})
		require.Eventually(t, func() bool {
			_, err = client.CoreV1().Namespaces().Get(ctx, nm.Name, metav1.GetOptions{})
			t.Logf("Getting Namespace: %s, Error: %v", nm.Name, err)
			return k8serrors.IsNotFound(err)
		}, 3*time.Minute, 3*time.Second, "namespace %s not removed in time", nm.Name)
	}
	internal.ChartUninstall(t, testKubeConfig)
}

func Test_Functions(t *testing.T) {
	setupSinks(t)

	testKubeConfig := requireEnv(t, "KUBECONFIG")
	internal.AcquireLeaseForTest(t, testKubeConfig)

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t.Context(), t, testKubeConfig)
	}

	if os.Getenv("SKIP_SETUP") != "true" {
		deployChartsAndApps(t, testKubeConfig)
	} else {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	kubeTestEnv := requireEnv(t, "KUBE_TEST_ENV")

	if kubeTestEnv == kindTestKubeEnv {
		expectedValuesDir = kindValuesDir
		runLocalClusterTests(t)
	} else {
		runHostedClusterTests(t, kubeTestEnv)
	}
}

// runLocalClusterTests runs tests that are expected to pass on local clusters like kind, minikube, etc.
// These tests are not ready to run in hosted clusters as we don't have the setup to send data to sinks.
// Eventually, we can update the tests to export to a file and run them in hosted clusters, example: testResourceAttributes
func runLocalClusterTests(t *testing.T) {
	t.Run("node.js traces captured", testNodeJSTraces)
	t.Run("java traces captured", testJavaTraces)
	t.Run(".NET traces captured", testDotNetTraces)
	t.Run("Python traces captured", testPythonTraces)
	t.Run("java metrics captured", testJavaMetrics)
	t.Run("node.js metrics captured", testNodeJSMetrics)
	t.Run(".NET metrics captured", testDotNetMetrics)
	t.Run("Python metrics captured", testPythonMetrics)
	t.Run("java profiling captured", testJavaProfiling)
	t.Run("node.js profiling captured", testNodeJSProfiling)
	t.Run(".NET profiling captured", testDotNetProfiling)
	t.Run("Python profiling captured", testPythonProfiling)
	t.Run("kubernetes cluster metrics", testK8sClusterReceiverMetrics)
	t.Run("agent logs", testAgentLogs)
	t.Run("container log attributes validation", func(t *testing.T) {
		validateLogAttributes(t, globalSinks.logsConsumer)
	})
	t.Run("test HEC metrics", testHECMetrics)
	t.Run("test k8s objects", testK8sObjects)
	t.Run("test agent metrics", testAgentMetrics)
	t.Run("test target allocator", testTargetAllocator)
	// TODO: re-enable this test in 0.129.0 https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/40788
	// t.Run("test prometheus metrics", testPrometheusAnnotationMetrics)

	// Test component health - verify no RBAC or connection errors
	t.Run("component error logs checks", func(t *testing.T) {
		testKubeConfig := requireEnv(t, "KUBECONFIG")
		kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
		require.NoError(t, err)
		client, err := kubernetes.NewForConfig(kubeConfig)
		require.NoError(t, err)

		internal.CheckComponentHealth(t, client, internal.DefaultNamespace, internal.AgentLabelSelector, kubeletstatsReceiverName)
		internal.CheckComponentHealth(t, client, internal.DefaultNamespace, clusterReceiverLabelSelector, k8sClusterReceiverName)
	})
}

// runHostedClusterTests runs tests that are specific to hosted clusters like EKS, GKE, AKS, etc.
// The test is specific to cloud provider data, example: resource attributes validation.
func runHostedClusterTests(t *testing.T, kubeTestEnv string) {
	testKubeConfig := requireEnv(t, "KUBECONFIG")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)
	switch kubeTestEnv {
	case eksTestKubeEnv, eksAutoModeTestKubeEnv, aksTestKubeEnv, gkeTestKubeEnv, rosaTestKubeEnv, gceTestKubeEnv:
		expectedValuesDir = selectExpectedValuesDir(kubeTestEnv)
		t.Run("agent resource attributes validation", func(t *testing.T) {
			validateResourceAttributes(t, client, kubeConfig, roleAgent)
		})
		t.Run("cluster receiver self-metrics resource attributes validation", func(t *testing.T) {
			validateResourceAttributes(t, client, kubeConfig, roleClusterReceiver)
		})
		t.Run("cluster receiver k8s cluster metrics resource attributes validation", func(t *testing.T) {
			validateResourceAttributes(t, client, kubeConfig, roleClusterReceiverK8s)
		})

		t.Run("component error logs checks", func(t *testing.T) {
			if kubeTestEnv == eksTestKubeEnv {
				internal.CheckComponentHealth(t, client, internal.DefaultNamespace, internal.AgentLabelSelector, journaldReceiverName)
			}
			internal.CheckComponentHealth(t, client, internal.DefaultNamespace, internal.AgentLabelSelector, kubeletstatsReceiverName)
			internal.CheckComponentHealth(t, client, internal.DefaultNamespace, clusterReceiverLabelSelector, k8sClusterReceiverName)
		})
	case autopilotTestKubeEnv:
		t.Run("component error logs checks", func(t *testing.T) {
			internal.CheckPodsReady(t, client, internal.DefaultNamespace, internal.AgentLabelSelector, 3*time.Minute, 10*time.Second)
			internal.CheckPodsReady(t, client, internal.DefaultNamespace, clusterReceiverLabelSelector, 3*time.Minute, 10*time.Second)

			internal.CheckComponentHealth(t, client, internal.DefaultNamespace, internal.AgentLabelSelector, kubeletstatsReceiverName)
			internal.CheckComponentHealth(t, client, internal.DefaultNamespace, clusterReceiverLabelSelector, k8sClusterReceiverName)
		})
	default:
		assert.Failf(t, "failed to run runHostedClusterTests", "no test available for kubeTestEnv %s", kubeTestEnv)
	}
}

func selectExpectedValuesDir(kubeTestEnv string) string {
	switch kubeTestEnv {
	case eksAutoModeTestKubeEnv:
		return eksAutoModeValuesDir
	case aksTestKubeEnv:
		return aksValuesDir
	case gkeTestKubeEnv:
		return gkeValuesDir
	case rosaTestKubeEnv:
		return rosaValuesDir
	case gceTestKubeEnv:
		return gceValuesDir
	default:
		return eksValuesDir
	}
}

func validateResourceAttributes(t *testing.T, clientset *kubernetes.Clientset, kubeConfig *rest.Config, role collectorRole) {
	var labelSelector, expectedResourceAttributesFile, podPathFile string

	switch role {
	case roleAgent:
		labelSelector = internal.AgentLabelSelector
		expectedResourceAttributesFile = filepath.Join(testDir, expectedValuesDir, "expected_resource_attributes_agent.yaml")
	case roleClusterReceiver:
		labelSelector = clusterReceiverLabelSelector
		expectedResourceAttributesFile = filepath.Join(testDir, expectedValuesDir, "expected_resource_attributes_cluster_receiver.yaml")
	case roleClusterReceiverK8s:
		labelSelector = clusterReceiverLabelSelector
		expectedResourceAttributesFile = filepath.Join(testDir, expectedValuesDir, "expected_resource_attributes_cluster_receiver_k8s_cluster.yaml")
	default:
		require.Failf(t, "failed to run validateResourceAttributes", "unknown role %q", role)
	}

	pods := internal.GetPods(t, clientset, internal.DefaultNamespace, labelSelector)
	require.NotEmpty(t, pods.Items, "no pods found for label %s", labelSelector)

	podName := pods.Items[0].Name
	isWindows := strings.ToLower(pods.Items[0].Labels["osType"]) == "windows"

	if role == roleClusterReceiverK8s {
		podPathFile = linuxPodK8sClusterMetricsPath
		if isWindows {
			podPathFile = winPodK8sClusterMetricsPath
		}
	} else {
		podPathFile = linuxPodMetricsPath
		if isWindows {
			podPathFile = winPodMetricsPath
		}
	}

	tmpFile, err := os.CreateTemp(t.TempDir(), "actualResourceAttributes*.yaml")
	require.NoError(t, err)

	internal.CopyFileFromPod(t, clientset, kubeConfig, internal.DefaultNamespace, podName, "otel-collector", podPathFile, tmpFile.Name())

	skipKeys := []string{"k8s.cluster.name", "cloud.platform"}
	expectedResourceAttributes := readAndNormalizeMetrics(t, expectedResourceAttributesFile, skipKeys...).ResourceMetrics().At(0).Resource().Attributes()

	// The k8s_cluster receiver emits multiple ResourceMetrics groups.
	// We pick a container resource for a stable comparison.
	var actualResourceAttributes pcommon.Map
	if role == roleClusterReceiverK8s {
		actualResourceAttributes = findResourceByAttr(t, tmpFile.Name(), "k8s.container.name", skipKeys...)
	} else {
		actualResourceAttributes = readAndNormalizeMetrics(t, tmpFile.Name(), skipKeys...).ResourceMetrics().At(0).Resource().Attributes()
	}

	require.True(t, expectedResourceAttributes.Equal(actualResourceAttributes), "Resource Attributes comparison failed for %s , expected values %s , actual values %s", role, internal.FormatAttributes(expectedResourceAttributes), internal.FormatAttributes(actualResourceAttributes))

	t.Cleanup(func() {
		require.NoError(t, os.Remove(tmpFile.Name()))
	})
}

func readAndNormalizeMetrics(t *testing.T, filePath string, skipKeys ...string) pmetric.Metrics {
	metrics, err := golden.ReadMetrics(filePath)
	require.NoError(t, err)
	internal.NormalizeAttributes(metrics.ResourceMetrics().At(0).Resource().Attributes(), skipKeys...)
	return metrics
}

// findResourceByAttr reads metrics from filePath, finds the first ResourceMetrics
// whose resource attributes contain the given key, normalizes it, and returns the attributes.
func findResourceByAttr(t *testing.T, filePath string, attrKey string, skipKeys ...string) pcommon.Map {
	metrics, err := golden.ReadMetrics(filePath)
	require.NoError(t, err)
	rm := metrics.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		attrs := rm.At(i).Resource().Attributes()
		if _, ok := attrs.Get(attrKey); ok {
			internal.NormalizeAttributes(attrs, skipKeys...)
			return attrs
		}
	}
	require.Failf(t, "resource not found", "no ResourceMetrics with attribute %q in %s", attrKey, filePath)
	return pcommon.NewMap()
}

func requireEnv(t *testing.T, key string) string {
	value, set := os.LookupEnv(key)
	require.True(t, set, "the environment variable %s must be set", key)
	return value
}

func containerImageShorten(value string) string {
	return archRe.ReplaceAllString(value[(strings.LastIndex(value, "/")+1):], "")
}

func shortenNames(value string) string {
	if strings.HasPrefix(value, "kube-proxy") {
		return "kube-proxy"
	}
	if strings.HasPrefix(value, "local-path-provisioner") {
		return "local-path-provisioner"
	}
	if strings.HasPrefix(value, "kindnet") {
		return "kindnet"
	}
	if strings.HasPrefix(value, "coredns") {
		return "coredns"
	}
	if strings.HasPrefix(value, "otelcol") {
		return "otelcol"
	}
	if strings.HasPrefix(value, "sock-splunk-otel-collector-agent") {
		return "sock-splunk-otel-collector-agent"
	}
	if strings.HasPrefix(value, "sock-splunk-otel-collector-k8s-cluster-receiver") {
		return "sock-splunk-otel-collector-k8s-cluster-receiver"
	}
	if strings.HasPrefix(value, "cert-manager-cainjector") {
		return "cert-manager-cainjector"
	}
	if strings.HasPrefix(value, "sock-operator") {
		return "sock-operator"
	}
	if strings.HasPrefix(value, "nodejs-test") {
		return "nodejs-test"
	}
	if strings.HasPrefix(value, "cert-manager-webhook") {
		return "cert-manager-webhook"
	}
	if strings.HasPrefix(value, "cert-manager") {
		return "cert-manager"
	}

	return value
}

func testK8sClusterReceiverMetrics(t *testing.T) {
	metricsConsumer := globalSinks.k8sclusterReceiverMetricsConsumer
	expectedMetricsFile := filepath.Join(testDir, expectedValuesDir, "expected_cluster_receiver.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedMetricsFile)
	require.NoError(t, err, "Failed to read expected metrics from expected_cluster_receiver.yaml")

	targetMetric := "k8s.pod.phase"
	selectedMetrics, exactMatch := internal.SelectMetricSetWithTimeout(t, expectedMetrics, targetMetric, metricsConsumer, 3*time.Minute, 10*time.Second)
	require.NotNil(t, selectedMetrics, "No metrics batch found containing target metric: %s", targetMetric)

	metricNames := internal.GetMetricNames(&expectedMetrics)
	err = pmetrictest.CompareMetrics(expectedMetrics, *selectedMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricAttributeValue("container.id", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.daemonset.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.deployment.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.node.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.replicaset.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.replicaset.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.namespace.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("container.image.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("container.image.tag", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.node.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.kubelet.version", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.container.status.last_terminated_reason", metricNames...),
		pmetrictest.IgnoreMetricValues(metricNames...),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreSubsequentDataPoints("k8s.container.ready", "k8s.container.restarts", "k8s.pod.phase"),
	)
	if err != nil {
		if !exactMatch {
			t.Logf("No exact count match: expected %d metrics, selected payload has %d", expectedMetrics.MetricCount(), selectedMetrics.MetricCount())
		}
		internal.MaybeUpdateExpectedMetricsResults(t, expectedMetricsFile, selectedMetrics)
		require.NoError(t, err, "K8s cluster receiver metrics comparison failed. Error: %v", err)
	}

	t.Logf("K8s cluster receiver metrics comparison passed for %d metrics", selectedMetrics.MetricCount())
}

func testAgentLogs(t *testing.T) {
	logsConsumer := globalSinks.logsConsumer
	internal.WaitForLogs(t, 5, logsConsumer)

	var helloWorldResource pcommon.Resource
	var helloWorldLogRecord *plog.LogRecord
	excludePods := true
	excludeNs := true
	var sourcetypes []string
	var indices []string
	var journalDsourceTypes []string

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		for i := 0; i < len(logsConsumer.AllLogs()); i++ {
			l := logsConsumer.AllLogs()[i]
			for j := 0; j < l.ResourceLogs().Len(); j++ {
				rl := l.ResourceLogs().At(j)
				if value, ok := rl.Resource().Attributes().Get("com.splunk.source"); ok && value.AsString() == "/run/log/journal" {
					if value, ok = rl.Resource().Attributes().Get("com.splunk.sourcetype"); ok {
						sourcetype := value.AsString()
						journalDsourceTypes = append(journalDsourceTypes, sourcetype)
					}
				}
				if value, ok := rl.Resource().Attributes().Get("com.splunk.sourcetype"); ok {
					sourcetype := value.AsString()
					sourcetypes = append(sourcetypes, sourcetype)
				}
				if value, ok := rl.Resource().Attributes().Get("com.splunk.index"); ok {
					index := value.AsString()
					indices = append(indices, index)
				}
				if value, ok := rl.Resource().Attributes().Get("k8s.container.name"); ok {
					if value.AsString() == "pod-w-index-w-ns-exclude" {
						excludePods = false
					}
					if value.AsString() == "pod-w-exclude-wo-ns-exclude" {
						excludeNs = false
					}
				}

				for k := 0; k < rl.ScopeLogs().Len(); k++ {
					sl := rl.ScopeLogs().At(k)
					for m := 0; m < sl.LogRecords().Len(); m++ {
						logRecord := sl.LogRecords().At(m)
						if logRecord.Body().AsString() == "Hello World" {
							helloWorldLogRecord = &logRecord
							helloWorldResource = rl.Resource()
						}
					}
				}
			}
		}
		assert.NotNil(tt, helloWorldLogRecord)
		assert.Contains(tt, sourcetypes, "kube:container:pod-w-index-wo-ns-index")
		assert.Contains(tt, sourcetypes, "kube:container:pod-wo-index-w-ns-index")
		assert.Contains(tt, sourcetypes, "kube:container:pod-wo-index-wo-ns-index")
		assert.Contains(tt, sourcetypes, "sourcetype-anno") // pod-wo-index-w-ns-index has a sourcetype annotation
	}, 3*time.Minute, 5*time.Second)

	if strings.HasPrefix(os.Getenv("K8S_VERSION"), "v1.30") {
		t.Log("Skipping test for journald sourcetypes for cluster version 1.30")
	} else {
		t.Run("test journald sourcetypes are set", func(t *testing.T) {
			assert.Contains(t, journalDsourceTypes, "kube:journald:containerd.service")
			assert.Contains(t, journalDsourceTypes, "kube:journald:kubelet.service")
		})
	}
	t.Run("test node.js log records", func(t *testing.T) {
		assert.NotNil(t, helloWorldLogRecord)
		sourceType, ok := helloWorldResource.Attributes().Get("com.splunk.sourcetype")
		assert.True(t, ok)
		assert.Equal(t, "kube:container:nodejs-test", sourceType.AsString())
		source, ok := helloWorldResource.Attributes().Get("com.splunk.source")
		assert.True(t, ok)
		assert.Regexp(t, "/var/log/pods/default_nodejs-test-.*/nodejs-test/0.log", source.AsString())
		index, ok := helloWorldResource.Attributes().Get("com.splunk.index")
		assert.True(t, ok)
		assert.Equal(t, "main", index.AsString())
		podName, ok := helloWorldLogRecord.Attributes().Get("k8s.pod.name")
		assert.True(t, ok)
		assert.Regexp(t, "nodejs-test-.*", podName.AsString())
	})
	t.Run("test index is set", func(t *testing.T) {
		assert.Contains(t, indices, "ns-anno")
		assert.Contains(t, indices, "pod-anno")
	})
	t.Run("test sourcetype is set", func(t *testing.T) {
		assert.Contains(t, sourcetypes, "kube:container:pod-w-index-wo-ns-index")
		assert.Contains(t, sourcetypes, "kube:container:pod-wo-index-w-ns-index")
		assert.Contains(t, sourcetypes, "kube:container:pod-wo-index-wo-ns-index")
		assert.Contains(t, sourcetypes, "sourcetype-anno") // pod-wo-index-w-ns-index has a sourcetype annotation
	})
	t.Run("excluded pods and namespaces", func(t *testing.T) {
		assert.True(t, excludePods, "excluded pods should be ignored")
		assert.True(t, excludeNs, "excluded namespaces should be ignored")
	})
	t.Run("check default metadata is attached to all the logs", func(t *testing.T) {
		_, ok := helloWorldLogRecord.Attributes().Get("k8s.pod.name")
		assert.True(t, ok)
		_, ok = helloWorldLogRecord.Attributes().Get("k8s.namespace.name")
		assert.True(t, ok)
		_, ok = helloWorldLogRecord.Attributes().Get("k8s.container.name")
		assert.True(t, ok)
		_, ok = helloWorldLogRecord.Attributes().Get("k8s.pod.uid")
		assert.True(t, ok)
	})
}

func testK8sObjects(t *testing.T) {
	logsObjectsConsumer := globalSinks.logsObjectsConsumer
	internal.WaitForLogs(t, 5, logsObjectsConsumer)

	var kinds []string
	var sourceTypes []string
	foundCustomField1 := false
	foundCustomField2 := false

	for i := 0; i < len(logsObjectsConsumer.AllLogs()); i++ {
		l := logsObjectsConsumer.AllLogs()[i]
		for j := 0; j < l.ResourceLogs().Len(); j++ {
			rl := l.ResourceLogs().At(j)
			for k := 0; k < rl.ScopeLogs().Len(); k++ {
				sl := rl.ScopeLogs().At(k)
				for m := 0; m < sl.LogRecords().Len(); m++ {
					logRecord := sl.LogRecords().At(m)
					if logRecord.Body().Type() == pcommon.ValueTypeMap {
						if kind, ok := logRecord.Body().Map().Get("kind"); ok {
							kinds = append(kinds, kind.AsString())
						}
					}
					if value, ok := logRecord.Attributes().Get("customfield1"); ok && value.AsString() == "customvalue1" {
						foundCustomField1 = true
					}
					if value, ok := logRecord.Attributes().Get("customfield2"); ok && value.AsString() == "customvalue2" {
						foundCustomField2 = true
					}
				}
			}
			if value, ok := rl.Resource().Attributes().Get("com.splunk.sourcetype"); ok {
				sourceTypes = append(sourceTypes, value.AsString())
			}
		}
	}

	assert.Contains(t, kinds, "Pod")
	assert.Contains(t, kinds, "Namespace")
	assert.Contains(t, kinds, "Node")

	assert.Contains(t, sourceTypes, "kube:object:pods")
	assert.Contains(t, sourceTypes, "kube:object:namespaces")
	assert.Contains(t, sourceTypes, "kube:object:nodes")

	assert.True(t, foundCustomField1)
	assert.True(t, foundCustomField2)
}

func testTargetAllocator(t *testing.T) {
	if !requiresPrometheusCRD(os.Getenv("KUBE_TEST_ENV")) {
		t.Skip("skipping test as required Prometheus CRDs are not installed")
	}

	testKubeConfig := requireEnv(t, "KUBECONFIG")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	// check target allocator logs
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		taPodList := internal.GetPods(t, client, internal.DefaultNamespace, internal.TargetAllocatorLabelSelector)
		containsReadyTAPod := false
		for _, pod := range taPodList.Items {
			if pod.Status.Phase != "Running" {
				t.Logf("Skipping pod %s in phase %s", pod.Name, pod.Status.Phase)
				continue
			}
			containsReadyTAPod = true
			podLogs := internal.GetPodLogs(t, client, internal.DefaultNamespace, pod.Name, internal.TargetAllocatorContainerName, 100)
			assert.Contains(c, podLogs, "Service Discovery watch event received", "Target allocator pod logs failed to successfully discover targets. Received logs: %v", podLogs)
		}
		assert.True(c, containsReadyTAPod, "No target allocator pod found ready")
	}, 3*time.Minute, 3*time.Second, "Failed to find required target allocator pod logs")

	// check agent logs
	serviceMonitorRegex := regexp.MustCompile(`Scrape job added.*"otelcol\.component\.id": "prometheus/ta.*"jobName": "serviceMonitor/default/prometheus-service-monitor/0"`)
	podMonitorRegex := regexp.MustCompile(`Scrape job added.*"otelcol\.component\.id": "prometheus/ta.*"jobName": "podMonitor/default/pod-monitor/0"`)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		agentPodList := internal.GetPods(t, client, internal.DefaultNamespace, internal.AgentLabelSelector)
		containsReadyAgentPod := false
		var combinedPodLogs strings.Builder
		for i, pod := range agentPodList.Items {
			if pod.Status.Phase != "Running" {
				t.Logf("Skipping pod %s in phase %s", pod.Name, pod.Status.Phase)
				continue
			}
			containsReadyAgentPod = true
			podLogs := internal.GetPodLogs(t, client, internal.DefaultNamespace, pod.Name, internal.CollectorContainerName, 500)
			assert.Contains(c, podLogs, "Starting target allocator discovery", "Collector failed to start target allocator discovery. Received logs: %v", podLogs)

			if i > 0 {
				combinedPodLogs.WriteString("\n")
			}
			combinedPodLogs.WriteString(fmt.Sprintf("%v\n%v", pod.Name, podLogs))
		}
		assert.True(c, containsReadyAgentPod, "No OTel Collector agent pod found ready")
		// NOTE: The target allocator distributes scrape jobs across agents when there are more than one.
		// Compile all logs from agents first, then ensure that altogether they have the required logs.
		assert.Regexp(c, serviceMonitorRegex, combinedPodLogs.String(), "Collector failed to start scrape job for serviceMonitor. Received logs: %v", combinedPodLogs.String())
		assert.Regexp(c, podMonitorRegex, combinedPodLogs.String(), "Collector failed to start scrape job for podMonitor. Received logs: %v", combinedPodLogs.String())
	}, 3*time.Minute, 3*time.Second, "Failed to find required agent pod logs")
}

// Internal telemetry metrics are only sent when an event occurs. Due to cluster
// setup and potentially different events occurring on the cluster before the
// test runs, different internal telemetry metrics will be sent. This method
// is used to ensure some events occur that would only take place occasionally
// before. This led to test flakiness as sometimes metrics are expected but the
// event hasn't taken place, or vice versa.
func generateRequiredTelemetry(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	assert.True(t, setKubeConfig)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	testNamespaceName := "test-namespace"
	internal.CreateNamespace(t, client, testNamespaceName)
	internal.WaitForDefaultServiceAccount(t, client, testNamespaceName)

	testPodConfig := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "main",
					Image:   "python:3.11",
					Command: []string{"python"},
					Args:    []string{"-c", "print('hello world')"},
				},
			},
		},
	}
	internal.CreatePod(t, client, testPodConfig.Name, testNamespaceName, testPodConfig)
	internal.DeletePod(t, client, testPodConfig.Name, testNamespaceName)
	internal.LabelNamespace(t, client, testNamespaceName, "testLabel", "true")
	internal.DeleteNamespace(t, client, testNamespaceName)
}

func testAgentMetrics(t *testing.T) {
	agentMetricsConsumer := globalSinks.agentMetricsConsumer

	generateRequiredTelemetry(t)

	t.Run("internal metrics", func(t *testing.T) {
		testAgentMetricsTemplate(t, agentMetricsConsumer, "expected_internal_metrics.yaml", "otelcol_otelsvc_k8s_pod_updated")
	})

	t.Run("kubeletstats metrics", func(t *testing.T) {
		testAgentMetricsTemplate(t, agentMetricsConsumer, "expected_kubeletstats_metrics.yaml", "container.memory.usage")
	})

	t.Run("hostmetrics", func(t *testing.T) {
		testAgentMetricsTemplate(t, agentMetricsConsumer, "expected_hostmetrics.yaml", "system.memory.usage")
	})
}

// testAgentMetricsTemplate tests metrics using template matching with target metric detection
func testAgentMetricsTemplate(t *testing.T, metricsSink *consumertest.MetricsSink, expectedFileName string, targetMetric string) {
	expectedMetricsFile := filepath.Join(testDir, expectedValuesDir, expectedFileName)
	expectedMetrics, err := golden.ReadMetrics(expectedMetricsFile)
	require.NoError(t, err, "Failed to read expected metrics from %s", expectedFileName)

	selectedMetrics, exactMatch := internal.SelectMetricSetWithTimeout(t, expectedMetrics, targetMetric, metricsSink, 3*time.Minute, 10*time.Second)
	require.NotNil(t, selectedMetrics, "No metrics batch found containing target metric: %s", targetMetric)

	testName := t.Name()
	if lastSlash := strings.LastIndex(testName, "/"); lastSlash != -1 {
		testName = testName[lastSlash+1:]
	}

	err = tryMetricsComparison(expectedMetrics, *selectedMetrics)
	if err != nil {
		if !exactMatch {
			t.Logf("No exact count match: expected %d metrics, selected payload has %d", expectedMetrics.MetricCount(), selectedMetrics.MetricCount())
		}
		t.Logf("Metric comparison failed for %s: %v", testName, err)
		internal.MaybeUpdateExpectedMetricsResults(t, expectedMetricsFile, selectedMetrics)
		require.NoError(t, err, "Metric comparison failed for %s test. Error: %v", testName, err)
	}

	t.Logf("Metric comparison passed for %d metrics in %s test", selectedMetrics.MetricCount(), testName)
}

// tryMetricsComparison performs metric comparison using pmetrictest.CompareMetrics and returns error
func tryMetricsComparison(expected pmetric.Metrics, actual pmetric.Metrics) error {
	replaceWithStar := func(string) string { return "*" }
	metricNames := internal.GetMetricNames(&expected)

	return pmetrictest.CompareMetrics(expected, actual,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricAttributeValue("container.id", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.daemonset.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.deployment.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreMetricAttributeValue("pod_identifier", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("otelcol_signal", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.replicaset.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.replicaset.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.namespace.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.namespace.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.node.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("container.image.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("container.image.tag", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.node.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id"),
		pmetrictest.IgnoreMetricAttributeValue("service_instance_id"),
		pmetrictest.IgnoreMetricAttributeValue("service_version", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("service.version", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("receiver", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("transport", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("exporter", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("com.splunk.sourcetype", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("device", metricNames...),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.ChangeResourceAttributeValue("k8s.container.name", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.id", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.image.tag", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.node.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.namespace.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.image.name", containerImageShorten),
		pmetrictest.ChangeResourceAttributeValue("host.name", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("service_instance_id", replaceWithStar),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreDatapointAttributesOrder(),
		pmetrictest.IgnoreSubsequentDataPoints(metricNames...),
		// pmetrictest.IgnoreSubsequentDataPoints("otelcol_receiver_accepted_log_records", "otelcol_receiver_refused_log_records"),
	)
}

func testHECMetrics(t *testing.T) {
	hecMetricsConsumer := globalSinks.hecMetricsConsumer

	metricNames := []string{
		"container.cpu.time",
		"container.filesystem.available",
		"container.filesystem.capacity",
		"container.filesystem.usage",
		"container.memory.available",
		"container.memory.major_page_faults",
		"container.memory.page_faults",
		"container.memory.rss",
		"container.memory.usage",
		"container.memory.working_set",
		"k8s.node.network.errors",
		"k8s.node.network.io",
		"k8s.pod.cpu.time",
		"k8s.pod.filesystem.available",
		"k8s.pod.filesystem.capacity",
		"k8s.pod.filesystem.usage",
		"k8s.pod.memory.available",
		"k8s.pod.memory.major_page_faults",
		"k8s.pod.memory.page_faults",
		"k8s.pod.memory.rss",
		"k8s.pod.memory.usage",
		"k8s.pod.memory.working_set",
		"k8s.pod.network.errors",
		"k8s.pod.network.io",
		"otelcol_exporter_queue_size",
		"otelcol_exporter_sent_metric_points",
		"otelcol_exporter_sent_log_records",
		"otelcol_otelsvc_k8s_ip_lookup_miss",
		"otelcol_processor_accepted_log_records",
		"otelcol_otelsvc_k8s_namespace_added",
		"otelcol_otelsvc_k8s_pod_added",
		"otelcol_otelsvc_k8s_pod_table_size",
		"otelcol_otelsvc_k8s_pod_updated",
		"otelcol_process_cpu_seconds",
		"otelcol_process_memory_rss",
		"otelcol_process_runtime_heap_alloc_bytes",
		"otelcol_process_runtime_total_alloc_bytes",
		"otelcol_process_runtime_total_sys_memory_bytes",
		"otelcol_process_uptime",
		"otelcol_processor_accepted_metric_points",
		"otelcol_receiver_accepted_metric_points",
		"otelcol_receiver_refused_metric_points",
		"otelcol_scraper_errored_metric_points",
		"otelcol_scraper_scraped_metric_points",
		"system.cpu.load_average.15m",
		"system.cpu.load_average.1m",
		"system.cpu.load_average.5m",
		"system.cpu.time",
		"system.disk.io",
		"system.disk.io_time",
		"system.disk.merged",
		"system.disk.operation_time",
		"system.disk.operations",
		"system.disk.pending_operations",
		"system.disk.weighted_io_time",
		"system.memory.usage",
		"system.network.connections",
		"system.network.dropped",
		"system.network.errors",
		"system.network.io",
		"system.network.packets",
		"system.paging.faults",
		"system.paging.operations",
		"system.paging.usage",
		"system.processes.count",
		"system.processes.created",
	}
	checkMetricsAreEmitted(t, hecMetricsConsumer, metricNames)
}

func waitForAllNamespacesToBeCreated(t *testing.T, client *kubernetes.Clientset) {
	require.Eventually(t, func() bool {
		nms, err := client.CoreV1().Namespaces().List(t.Context(), metav1.ListOptions{})
		require.NoError(t, err)
		for _, d := range nms.Items {
			if d.Status.Phase != corev1.NamespaceActive {
				return false
			}
		}
		return true
	}, 5*time.Minute, 10*time.Second)
}

// metricMatchFn decides whether a given metric (with its resource attributes)
// should be counted. Return true to accept the metric.
type metricMatchFn func(resAttrs pcommon.Map, metric pmetric.Metric) bool

func checkMetricsAreEmitted(t *testing.T, mc *consumertest.MetricsSink, metricNames []string) {
	checkMetrics(t, mc, metricNames, "", nil)
}

func checkMetricsFromApp(t *testing.T, mc *consumertest.MetricsSink, sdkLanguage, serviceName string, metricNames []string) {
	checkMetrics(t, mc, metricNames, sdkLanguage+"/"+serviceName, func(resAttrs pcommon.Map, metric pmetric.Metric) bool {
		if hasAttrMatch(resAttrs, "telemetry.sdk.language", sdkLanguage) && hasAttrMatch(resAttrs, "service.name", serviceName) {
			return true
		}
		return metricDataPointsHaveAttrs(metric, "telemetry.sdk.language", sdkLanguage, "service.name", serviceName)
	})
}

func checkMetrics(t *testing.T, mc *consumertest.MetricsSink, metricNames []string, label string, match metricMatchFn) {
	metricsToFind := map[string]bool{}
	for _, name := range metricNames {
		metricsToFind[name] = false
	}
	require.Eventuallyf(t, func() bool {
		for _, m := range mc.AllMetrics() {
			for i := 0; i < m.ResourceMetrics().Len(); i++ {
				rm := m.ResourceMetrics().At(i)
				resAttrs := rm.Resource().Attributes()
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						metric := sm.Metrics().At(k)
						if match == nil || match(resAttrs, metric) {
							metricsToFind[metric.Name()] = true
						}
					}
				}
			}
		}
		var stillMissing, found []string
		for _, name := range metricNames {
			if metricsToFind[name] {
				found = append(found, name)
			} else {
				stillMissing = append(stillMissing, name)
			}
		}
		if label != "" {
			t.Logf("[%s] found=%d missing=%d (%s)", label, len(found), len(stillMissing), strings.Join(stillMissing, ", "))
		} else {
			t.Logf("found=%d missing=%d (%s)", len(found), len(stillMissing), strings.Join(stillMissing, ", "))
		}
		return len(stillMissing) == 0
	}, 3*time.Minute, 10*time.Second,
		"failed to receive all metrics %s in 3 minutes", label)
}

func hasAttrMatch(attrs pcommon.Map, key, expected string) bool {
	v, ok := attrs.Get(key)
	return ok && v.Str() == expected
}

// metricDataPointsHaveAttrs checks whether any data point in a metric carries
// all of the given key/value pairs. Pairs are passed as alternating key, value strings.
func metricDataPointsHaveAttrs(metric pmetric.Metric, kvPairs ...string) bool {
	check := func(attrs pcommon.Map) bool {
		for i := 0; i < len(kvPairs)-1; i += 2 {
			if !hasAttrMatch(attrs, kvPairs[i], kvPairs[i+1]) {
				return false
			}
		}
		return true
	}
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < metric.Gauge().DataPoints().Len(); i++ {
			if check(metric.Gauge().DataPoints().At(i).Attributes()) {
				return true
			}
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
			if check(metric.Sum().DataPoints().At(i).Attributes()) {
				return true
			}
		}
	case pmetric.MetricTypeHistogram:
		for i := 0; i < metric.Histogram().DataPoints().Len(); i++ {
			if check(metric.Histogram().DataPoints().At(i).Attributes()) {
				return true
			}
		}
	case pmetric.MetricTypeSummary:
		for i := 0; i < metric.Summary().DataPoints().Len(); i++ {
			if check(metric.Summary().DataPoints().At(i).Attributes()) {
				return true
			}
		}
	case pmetric.MetricTypeExponentialHistogram:
		for i := 0; i < metric.ExponentialHistogram().DataPoints().Len(); i++ {
			if check(metric.ExponentialHistogram().DataPoints().At(i).Attributes()) {
				return true
			}
		}
	}
	return false
}
