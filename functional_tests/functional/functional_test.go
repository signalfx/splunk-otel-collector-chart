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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
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
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	signalFxReceiverK8sClusterReceiverPort = 19443
	kindTestKubeEnv                        = "kind"
	eksTestKubeEnv                         = "eks"
	autopilotTestKubeEnv                   = "gke/autopilot"
	aksTestKubeEnv                         = "aks"
	gceTestKubeEnv                         = "gce"
	testDir                                = "testdata"
	valuesDir                              = "values"
	manifestsDir                           = "manifests"
	eksValuesDir                           = "expected_eks_values"
	kindValuesDir                          = "expected_kind_values"
)

var archRe = regexp.MustCompile("-amd64$|-arm64$|-ppc64le$")

var globalSinks *sinks

var expectedValuesDir string

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

	// load up Prometheus PodMonitor and ServiceMonitor CRDs:
	stream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "prometheus_operator_crds.yaml"))
	require.NoError(t, err)
	sch := k8sruntime.NewScheme()

	for _, resourceYAML := range strings.Split(string(stream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		var groupVersionKind *schema.GroupVersionKind
		var obj k8sruntime.Object
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
			crd, err = apiExtensions.Create(t.Context(), crd, metav1.CreateOptions{})
			require.NoError(t, err)
			t.Logf("Deployed CRD %s", crd.Name)

			// Wait for CRD to be Established before moving on
			require.EventuallyWithT(t, func(tt *assert.CollectT) {
				latest, getErr := apiExtensions.Get(t.Context(), crd.Name, metav1.GetOptions{})
				assert.NoError(tt, getErr)
				established := false
				for _, cond := range latest.Status.Conditions {
					if cond.Type == appextensionsv1.Established && cond.Status == appextensionsv1.ConditionTrue {
						established = true
					}
				}
				assert.True(tt, established)
			}, 3*time.Minute, 3*time.Second, "CRD %s not established", crd.Name)

			for _, version := range crd.Spec.Versions {
				sch.AddKnownTypeWithName(
					schema.GroupVersionKind{
						Group:   crd.Spec.Group,
						Version: version.Name,
						Kind:    crd.Spec.Names.Kind,
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
	_, err = dynamicClient.Resource(g).Namespace(internal.Namespace).Create(t.Context(),
		podMonitor.(*unstructured.Unstructured), metav1.CreateOptions{})
	assert.NoError(t, err)

	var valuesFile string
	switch kubeTestEnv {
	case autopilotTestKubeEnv:
		valuesFile, err = filepath.Abs(filepath.Join(testDir, valuesDir, "autopilot_test_values.yaml.tmpl"))
	case aksTestKubeEnv:
		valuesFile, err = filepath.Abs(filepath.Join(testDir, valuesDir, "aks_test_values.yaml.tmpl"))
	default:
		valuesFile, err = filepath.Abs(filepath.Join(testDir, valuesDir, "test_values.yaml.tmpl"))
	}
	assert.NoError(t, err)

	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}

	replacements := map[string]any{
		"K8sClusterEndpoint":    fmt.Sprintf("http://%s:%d", hostEp, signalFxReceiverK8sClusterReceiverPort),
		"AgentEndpoint":         fmt.Sprintf("http://%s:%d", hostEp, internal.SignalFxReceiverPort),
		"LogHecEndpoint":        fmt.Sprintf("http://%s:%d", hostEp, internal.HECLogsReceiverPort),
		"MetricHecEndpoint":     fmt.Sprintf("http://%s:%d/services/collector", hostEp, internal.HECMetricsReceiverPort),
		"OtlpEndpoint":          fmt.Sprintf("%s:%d", hostEp, internal.OTLPGRPCReceiverPort),
		"OtlpHttpEndpoint":      fmt.Sprintf("%s:%d", hostEp, internal.OTLPHTTPReceiverPort),
		"ApiURLEndpoint":        fmt.Sprintf("http://%s:%d", hostEp, internal.SignalFxAPIPort),
		"LogObjectsHecEndpoint": fmt.Sprintf("http://%s:%d/services/collector", hostEp, internal.HECObjectsReceiverPort),
		"KubeTestEnv":           kubeTestEnv,
	}
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements)

	deployments := client.AppsV1().Deployments(internal.Namespace)

	// NodeJS test app
	stream, err = os.ReadFile(filepath.Join(testDir, "nodejs", "deployment.yaml"))
	require.NoError(t, err)
	deployment, _, err := decode(stream, nil, nil)
	require.NoError(t, err)
	_, err = deployments.Create(t.Context(), deployment.(*appsv1.Deployment), metav1.CreateOptions{})
	if err != nil {
		_, err2 := deployments.Update(t.Context(), deployment.(*appsv1.Deployment), metav1.UpdateOptions{})
		assert.NoError(t, err2)
		if err2 != nil {
			require.NoError(t, err)
		}
	}
	// Java test app
	stream, err = os.ReadFile(filepath.Join(testDir, "java", "deployment.yaml"))
	require.NoError(t, err)
	deployment, _, err = decode(stream, nil, nil)
	require.NoError(t, err)
	_, err = deployments.Create(t.Context(), deployment.(*appsv1.Deployment), metav1.CreateOptions{})
	if err != nil {
		_, err2 := deployments.Update(t.Context(), deployment.(*appsv1.Deployment), metav1.UpdateOptions{})
		assert.NoError(t, err2)
		if err2 != nil {
			require.NoError(t, err)
		}
	}
	// .NET test app
	stream, err = os.ReadFile(filepath.Join(testDir, "dotnet", "deployment.yaml"))
	require.NoError(t, err)
	deployment, _, err = decode(stream, nil, nil)
	require.NoError(t, err)
	_, err = deployments.Create(t.Context(), deployment.(*appsv1.Deployment), metav1.CreateOptions{})
	if err != nil {
		_, err2 := deployments.Update(t.Context(), deployment.(*appsv1.Deployment), metav1.UpdateOptions{})
		assert.NoError(t, err2)
		if err2 != nil {
			require.NoError(t, err)
		}
	}
	// Python test app
	stream, err = os.ReadFile(filepath.Join(testDir, "python", "deployment.yaml"))
	require.NoError(t, err)
	deployment, _, err = decode(stream, nil, nil)
	require.NoError(t, err)
	_, err = deployments.Create(t.Context(), deployment.(*appsv1.Deployment), metav1.CreateOptions{})
	if err != nil {
		_, err2 := deployments.Update(t.Context(), deployment.(*appsv1.Deployment), metav1.UpdateOptions{})
		assert.NoError(t, err2)
		if err2 != nil {
			require.NoError(t, err)
		}
	}
	// Prometheus annotation
	stream, err = os.ReadFile(filepath.Join(testDir, manifestsDir, "deployment_with_prometheus_annotations.yaml"))
	require.NoError(t, err)
	deployment, _, err = decode(stream, nil, nil)
	require.NoError(t, err)
	_, err = deployments.Create(t.Context(), deployment.(*appsv1.Deployment), metav1.CreateOptions{})
	if err != nil {
		_, err2 := deployments.Update(t.Context(), deployment.(*appsv1.Deployment), metav1.UpdateOptions{})
		assert.NoError(t, err2)
		if err2 != nil {
			require.NoError(t, err)
		}
	}

	// Service
	stream, err = os.ReadFile(filepath.Join(testDir, manifestsDir, "service.yaml"))
	require.NoError(t, err)
	service, _, err := decode(stream, nil, nil)
	require.NoError(t, err)
	_, err = client.CoreV1().Services(internal.Namespace).Create(t.Context(), service.(*corev1.Service),
		metav1.CreateOptions{})
	require.NoError(t, err)

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
	_, err = dynamicClient.Resource(g).Namespace(internal.Namespace).Create(t.Context(),
		serviceMonitor.(*unstructured.Unstructured), metav1.CreateOptions{})
	assert.NoError(t, err)

	// Read jobs
	jobstream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "test_jobs.yaml"))
	require.NoError(t, err)
	for _, resourceYAML := range strings.Split(string(jobstream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		obj, groupVersionKind, err := decode(
			[]byte(resourceYAML),
			nil,
			nil)
		require.NoError(t, err)
		if groupVersionKind.Group == "" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "Namespace" {
			nm := obj.(*corev1.Namespace)
			nms := client.CoreV1().Namespaces()
			_, err := nms.Create(t.Context(), nm, metav1.CreateOptions{})
			require.NoError(t, err)
			t.Logf("Deployed namespace %s", nm.Name)
		}
	}

	waitForAllNamespacesToBeCreated(t, client)

	for _, resourceYAML := range strings.Split(string(jobstream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		obj, groupVersionKind, err := decode(
			[]byte(resourceYAML),
			nil,
			nil)
		require.NoError(t, err)

		if groupVersionKind.Group == "batch" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "Job" {
			job := obj.(*batchv1.Job)
			jobClient := client.BatchV1().Jobs(job.Namespace)
			_, err := jobClient.Create(t.Context(), job, metav1.CreateOptions{})
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
		teardown(t, context.Background(), testKubeConfig)
	})
}

func teardown(t *testing.T, ctx context.Context, testKubeConfig string) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)
	extensionsClient, err := clientset.NewForConfig(kubeConfig)
	require.NoError(t, err)
	waitTime := int64(0)
	deployments := client.AppsV1().Deployments(internal.Namespace)
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
	_ = client.CoreV1().Services(internal.Namespace).Delete(ctx, "prometheus-annotation-service",
		metav1.DeleteOptions{
			GracePeriodSeconds: &waitTime,
		})

	jobstream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "test_jobs.yaml"))
	require.NoError(t, err)
	var namespaces []*corev1.Namespace
	var jobs []*batchv1.Job
	for _, resourceYAML := range strings.Split(string(jobstream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		var groupVersionKind *schema.GroupVersionKind
		var obj k8sruntime.Object
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

	crdstream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "prometheus_operator_crds.yaml"))
	require.NoError(t, err)
	for _, resourceYAML := range strings.Split(string(crdstream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		obj, groupVersionKind, err := decode(
			[]byte(resourceYAML),
			nil,
			nil)
		require.NoError(t, err)
		if groupVersionKind.Group == "apiextensions.k8s.io" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "CustomResourceDefinition" {
			crd := obj.(*appextensionsv1.CustomResourceDefinition)
			apiExtensions := extensionsClient.ApiextensionsV1().CustomResourceDefinitions()
			_ = apiExtensions.Delete(ctx, crd.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &waitTime,
			})
		}
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

	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	internal.AcquireLeaseForTest(t, testKubeConfig)
	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t, t.Context(), testKubeConfig)
	}

	// deploy the chart and applications.
	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		deployChartsAndApps(t, testKubeConfig)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	kubeTestEnv, setKubeTestEnv := os.LookupEnv("KUBE_TEST_ENV")
	require.True(t, setKubeTestEnv, "the environment variable KUBE_TEST_ENV must be set")

	switch kubeTestEnv {
	case kindTestKubeEnv, autopilotTestKubeEnv, aksTestKubeEnv, gceTestKubeEnv:
		expectedValuesDir = kindValuesDir
	case eksTestKubeEnv:
		expectedValuesDir = eksValuesDir
	default:
		assert.Fail(t, "KUBE_TEST_ENV is set to invalid value. Must be one of [kind, eks].")
	}

	t.Run("node.js traces captured", testNodeJSTraces)
	t.Run("java traces captured", testJavaTraces)
	t.Run(".NET traces captured", testDotNetTraces)
	t.Run("Python traces captured", testPythonTraces)
	t.Run("kubernetes cluster metrics", testK8sClusterReceiverMetrics)
	t.Run("agent logs", testAgentLogs)
	t.Run("test HEC metrics", testHECMetrics)
	t.Run("test k8s objects", testK8sObjects)
	t.Run("test agent metrics", testAgentMetrics)
	t.Run("test prometheus metrics", testPrometheusAnnotationMetrics)
}

func testNodeJSTraces(t *testing.T) {
	tracesConsumer := globalSinks.tracesConsumer

	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join(testDir, expectedValuesDir, "expected_nodejs_traces.yaml")
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)

	internal.WaitForTraces(t, 10, tracesConsumer)

	var selectedTrace *ptrace.Traces

	require.Eventually(t, func() bool {
		for i := len(tracesConsumer.AllTraces()) - 1; i > 0; i-- {
			trace := tracesConsumer.AllTraces()[i]
			if val, ok := trace.ResourceSpans().At(0).Resource().Attributes().Get("telemetry.sdk.language"); ok && strings.Contains(val.Str(), "nodejs") {
				if expectedTraces.SpanCount() == trace.SpanCount() && expectedTraces.ResourceSpans().Len() == trace.ResourceSpans().Len() {
					selectedTrace = &trace
					break
				}
			}
		}
		return selectedTrace != nil
	}, 3*time.Minute, 5*time.Second)
	require.NotNil(t, selectedTrace)

	maskScopeVersion(*selectedTrace)
	maskScopeVersion(expectedTraces)

	err = ptracetest.CompareTraces(expectedTraces, *selectedTrace,
		ptracetest.IgnoreResourceAttributeValue("container.id"),
		ptracetest.IgnoreResourceAttributeValue("host.arch"),
		ptracetest.IgnoreResourceAttributeValue("k8s.deployment.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.replicaset.name"),
		ptracetest.IgnoreResourceAttributeValue("os.version"),
		ptracetest.IgnoreResourceAttributeValue("process.pid"),
		ptracetest.IgnoreResourceAttributeValue("splunk.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("process.runtime.version"),
		ptracetest.IgnoreResourceAttributeValue("process.command"),
		ptracetest.IgnoreResourceAttributeValue("process.command_args"),
		ptracetest.IgnoreResourceAttributeValue("process.executable.path"),
		ptracetest.IgnoreResourceAttributeValue("process.owner"),
		ptracetest.IgnoreResourceAttributeValue("process.runtime.description"),
		ptracetest.IgnoreResourceAttributeValue("splunk.zc.method"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.sdk.version"),
		ptracetest.IgnoreResourceAttributeValue("service.instance.id"),
		ptracetest.IgnoreSpanAttributeValue("http.user_agent"),
		ptracetest.IgnoreSpanAttributeValue("net.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("network.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("os.version"),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
		ptracetest.IgnoreScopeSpanInstrumentationScopeVersion(),
	)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedTracesResult(t, expectedTracesFile, selectedTrace)
	}
	require.NoError(t, err)
}

func testPythonTraces(t *testing.T) {
	tracesConsumer := globalSinks.tracesConsumer

	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join(testDir, expectedValuesDir, "expected_python_traces.yaml")
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)

	internal.WaitForTraces(t, 10, tracesConsumer)

	var selectedTrace *ptrace.Traces

	read := 0
	require.Eventually(t, func() bool {
		for i := len(tracesConsumer.AllTraces()) - 1; i > read; i-- {
			trace := tracesConsumer.AllTraces()[i]
			if val, ok := trace.ResourceSpans().At(0).Resource().Attributes().Get("telemetry.sdk.language"); ok && strings.Contains(val.Str(), "python") {
				if expectedTraces.SpanCount() == trace.SpanCount() && expectedTraces.ResourceSpans().Len() == trace.ResourceSpans().Len() {
					selectedTrace = &trace
					break
				}
			}
		}
		read = len(tracesConsumer.AllTraces()) - 1
		return selectedTrace != nil
	}, 1*time.Minute, 5*time.Second)
	require.NotNil(t, selectedTrace)

	maskScopeVersion(*selectedTrace)
	maskScopeVersion(expectedTraces)

	err = ptracetest.CompareTraces(expectedTraces, *selectedTrace,
		ptracetest.IgnoreResourceAttributeValue("container.id"),
		ptracetest.IgnoreResourceAttributeValue("host.arch"),
		ptracetest.IgnoreResourceAttributeValue("k8s.deployment.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.replicaset.name"),
		ptracetest.IgnoreResourceAttributeValue("os.version"),
		ptracetest.IgnoreResourceAttributeValue("process.pid"),
		ptracetest.IgnoreResourceAttributeValue("splunk.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("process.runtime.version"),
		ptracetest.IgnoreResourceAttributeValue("process.command"),
		ptracetest.IgnoreResourceAttributeValue("process.command_args"),
		ptracetest.IgnoreResourceAttributeValue("process.executable.path"),
		ptracetest.IgnoreResourceAttributeValue("process.owner"),
		ptracetest.IgnoreResourceAttributeValue("process.runtime.description"),
		ptracetest.IgnoreResourceAttributeValue("splunk.zc.method"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.sdk.version"),
		ptracetest.IgnoreResourceAttributeValue("service.instance.id"),
		ptracetest.IgnoreSpanAttributeValue("http.user_agent"),
		ptracetest.IgnoreSpanAttributeValue("net.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("network.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("os.version"),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
	)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedTracesResult(t, expectedTracesFile, selectedTrace)
	}
	require.NoError(t, err)
}

func testJavaTraces(t *testing.T) {
	tracesConsumer := globalSinks.tracesConsumer

	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join(testDir, expectedValuesDir, "expected_java_traces.yaml")
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)

	internal.WaitForTraces(t, 10, tracesConsumer)

	var selectedTrace *ptrace.Traces

	require.Eventually(t, func() bool {
		for i := len(tracesConsumer.AllTraces()) - 1; i > 0; i-- {
			trace := tracesConsumer.AllTraces()[i]
			if val, ok := trace.ResourceSpans().At(0).Resource().Attributes().Get("telemetry.sdk.language"); ok && strings.Contains(val.Str(), "java") {
				if expectedTraces.SpanCount() == trace.SpanCount() && expectedTraces.ResourceSpans().Len() == trace.ResourceSpans().Len() {
					selectedTrace = &trace
					break
				}
			}
		}
		return selectedTrace != nil
	}, 3*time.Minute, 5*time.Second)
	require.NotNil(t, selectedTrace)

	maskScopeVersion(*selectedTrace)
	maskScopeVersion(expectedTraces)

	err = ptracetest.CompareTraces(expectedTraces, *selectedTrace,
		ptracetest.IgnoreResourceAttributeValue("os.description"),
		ptracetest.IgnoreResourceAttributeValue("process.pid"),
		ptracetest.IgnoreResourceAttributeValue("container.id"),
		ptracetest.IgnoreResourceAttributeValue("k8s.deployment.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.replicaset.name"),
		ptracetest.IgnoreResourceAttributeValue("os.version"),
		ptracetest.IgnoreResourceAttributeValue("host.arch"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.sdk.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.auto.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("splunk.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("splunk.zc.method"),
		ptracetest.IgnoreResourceAttributeValue("service.instance.id"),
		ptracetest.IgnoreSpanAttributeValue("network.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("net.sock.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("thread.id"),
		ptracetest.IgnoreSpanAttributeValue("thread.name"),
		ptracetest.IgnoreSpanAttributeValue("os.version"),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
	)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedTracesResult(t, expectedTracesFile, selectedTrace)
	}
	require.NoError(t, err)
}

func testDotNetTraces(t *testing.T) {
	tracesConsumer := globalSinks.tracesConsumer

	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join(testDir, expectedValuesDir, "expected_dotnet_traces.yaml")
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)

	internal.WaitForTraces(t, 30, tracesConsumer)
	var selectedTrace *ptrace.Traces

	require.Eventually(t, func() bool {
		for i := len(tracesConsumer.AllTraces()) - 1; i > 0; i-- {
			trace := tracesConsumer.AllTraces()[i]
			if val, ok := trace.ResourceSpans().At(0).Resource().Attributes().Get("telemetry.sdk.language"); ok && strings.Contains(val.Str(), "dotnet") {
				if expectedTraces.SpanCount() == trace.SpanCount() && expectedTraces.ResourceSpans().Len() == trace.ResourceSpans().Len() {
					selectedTrace = &trace
					break
				}
			}
		}
		return selectedTrace != nil
	}, 3*time.Minute, 5*time.Second)
	require.NotNil(t, selectedTrace)

	maskScopeVersion(*selectedTrace)
	maskScopeVersion(expectedTraces)
	maskSpanParentID(*selectedTrace)
	maskSpanParentID(expectedTraces)

	err = ptracetest.CompareTraces(expectedTraces, *selectedTrace,
		ptracetest.IgnoreResourceAttributeValue("os.description"),
		ptracetest.IgnoreResourceAttributeValue("process.pid"),
		ptracetest.IgnoreResourceAttributeValue("container.id"),
		ptracetest.IgnoreResourceAttributeValue("k8s.deployment.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.replicaset.name"),
		ptracetest.IgnoreResourceAttributeValue("os.version"),
		ptracetest.IgnoreResourceAttributeValue("os.build_id"),
		ptracetest.IgnoreResourceAttributeValue("host.arch"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.sdk.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.auto.version"),
		ptracetest.IgnoreResourceAttributeValue("splunk.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("splunk.zc.method"),
		ptracetest.IgnoreResourceAttributeValue("service.instance.id"),
		ptracetest.IgnoreSpanAttributeValue("net.sock.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("thread.id"),
		ptracetest.IgnoreSpanAttributeValue("thread.name"),
		ptracetest.IgnoreSpanAttributeValue("os.version"),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
	)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedTracesResult(t, expectedTracesFile, selectedTrace)
	}
	require.NoError(t, err)
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
	require.NoError(t, err)

	replaceWithStar := func(string) string { return "*" }

	var selectedMetrics *pmetric.Metrics
	for h := len(metricsConsumer.AllMetrics()) - 1; h >= 0; h-- {
		m := metricsConsumer.AllMetrics()[h]
		foundCorrectSet := false
	OUTER:
		for i := 0; i < m.ResourceMetrics().Len(); i++ {
			for j := 0; j < m.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
				for k := 0; k < m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
					metricToConsider := m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
					if metricToConsider.Name() == "k8s.container.restarts" {
						foundCorrectSet = true
						break OUTER
					}
				}
			}
		}
		if !foundCorrectSet {
			continue
		}
		if m.ResourceMetrics().Len() == expectedMetrics.ResourceMetrics().Len() && m.MetricCount() == expectedMetrics.MetricCount() {
			selectedMetrics = &m
			break
		}
	}
	require.NotNil(t, selectedMetrics)

	metricNames := []string{"k8s.node.condition_ready", "k8s.namespace.phase", "k8s.pod.phase", "k8s.replicaset.desired", "k8s.replicaset.available", "k8s.daemonset.ready_nodes", "k8s.daemonset.misscheduled_nodes", "k8s.daemonset.desired_scheduled_nodes", "k8s.daemonset.current_scheduled_nodes", "k8s.container.ready", "k8s.container.memory_request", "k8s.container.memory_limit", "k8s.container.cpu_request", "k8s.container.cpu_limit", "k8s.deployment.desired", "k8s.deployment.available", "k8s.container.restarts", "k8s.container.cpu_request", "k8s.container.memory_request", "k8s.container.memory_limit"}

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
		pmetrictest.IgnoreMetricValues(metricNames...),
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
		pmetrictest.ChangeResourceAttributeValue("container.id", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.name", replaceWithStar),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreSubsequentDataPoints("k8s.container.ready", "k8s.container.restarts", "k8s.pod.phase"),
	)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedMetricsResult(t, expectedMetricsFile, selectedMetrics)
	}
	require.NoError(t, err)
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
					if value, ok := rl.Resource().Attributes().Get("com.splunk.sourcetype"); ok {
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

func testAgentMetrics(t *testing.T) {
	agentMetricsConsumer := globalSinks.agentMetricsConsumer

	metricNames := []string{
		"container.filesystem.available",
		"container.filesystem.capacity",
		"container.filesystem.usage",
		"container.memory.usage",
		"container_cpu_utilization",
		"k8s.pod.network.errors",
		"k8s.pod.network.io",
		"otelcol_exporter_sent_log_records",
		"otelcol_exporter_queue_capacity",
		"otelcol_exporter_send_failed_log_records",
		"otelcol_exporter_sent_spans",
		"otelcol_otelsvc_k8s_ip_lookup_miss",
		"otelcol_processor_accepted_spans",
		"otelcol_processor_accepted_log_records",
		"otelcol_exporter_queue_size",
		"otelcol_exporter_sent_metric_points",
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
		"otelcol_receiver_accepted_spans",
		"otelcol_processor_accepted_metric_points",
		"otelcol_processor_filter_logs.filtered",
		"otelcol_receiver_accepted_metric_points",
		"otelcol_receiver_accepted_log_records",
		"otelcol_receiver_refused_log_records",
		"otelcol_receiver_refused_metric_points",
		"otelcol_receiver_refused_spans",
		"otelcol_scraper_errored_metric_points",
		"otelcol_scraper_scraped_metric_points",
		"scrape_duration_seconds",
		"scrape_samples_post_metric_relabeling",
		"scrape_samples_scraped",
		"scrape_series_added",
		"system.cpu.load_average.15m",
		"system.cpu.load_average.1m",
		"system.cpu.load_average.5m",
		"system.disk.operations",
		"system.memory.usage",
		"system.network.errors",
		"system.network.io",
		"system.paging.operations",
		"up",
	}
	checkMetricsAreEmitted(t, agentMetricsConsumer, metricNames, nil)

	expectedInternalMetricsFile := filepath.Join(testDir, expectedValuesDir, "expected_internal_metrics.yaml")
	expectedInternalMetrics, err := golden.ReadMetrics(expectedInternalMetricsFile)
	require.NoError(t, err)

	replaceWithStar := func(string) string { return "*" }

	selectedInternalMetrics := selectMetricSet(expectedInternalMetrics, "otelcol_process_runtime_total_alloc_bytes", agentMetricsConsumer, false)
	if selectedInternalMetrics == nil {
		t.Skip("No metric batch identified with the right metric count, exiting")
		return
	}
	require.NotNil(t, selectedInternalMetrics)

	err = pmetrictest.CompareMetrics(expectedInternalMetrics, *selectedInternalMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricAttributeValue("container.id", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.daemonset.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.deployment.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.replicaset.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.replicaset.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.namespace.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("container.image.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("container.image.tag", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.node.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id"),
		pmetrictest.IgnoreMetricAttributeValue("service_instance_id"),
		pmetrictest.IgnoreMetricAttributeValue("service_version", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("receiver", metricNames...),
		pmetrictest.IgnoreMetricValues(),
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
		pmetrictest.ChangeResourceAttributeValue("container.id", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.name", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("service_instance_id", replaceWithStar),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreSubsequentDataPoints("otelcol_receiver_accepted_log_records", "otelcol_receiver_refused_log_records"),
	)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedMetricsResult(t, expectedInternalMetricsFile, selectedInternalMetrics)
	}
	assert.NoError(t, err)

	expectedKubeletStatsMetricsFile := filepath.Join(testDir, expectedValuesDir, "expected_kubeletstats_metrics.yaml")
	expectedKubeletStatsMetrics, err := golden.ReadMetrics(expectedKubeletStatsMetricsFile)
	require.NoError(t, err)
	selectedKubeletstatsMetrics := selectMetricSet(expectedKubeletStatsMetrics, "container.memory.usage", agentMetricsConsumer, false)
	if selectedKubeletstatsMetrics == nil {
		t.Skip("No metric batch identified with the right metric count, exiting")
		return
	}
	require.NotNil(t, selectedKubeletstatsMetrics)

	err = pmetrictest.CompareMetrics(expectedKubeletStatsMetrics, *selectedKubeletstatsMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricAttributeValue("container.id", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.daemonset.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.deployment.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.replicaset.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.replicaset.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.namespace.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("container.image.tag", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.node.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("service_instance_id", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("service_version", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("receiver", metricNames...),
		pmetrictest.IgnoreMetricValues(metricNames...),
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
		pmetrictest.ChangeResourceAttributeValue("container.id", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.name", replaceWithStar),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedMetricsResult(t, expectedKubeletStatsMetricsFile, selectedKubeletstatsMetrics)
	}
	assert.NoError(t, err)
}

func testPrometheusAnnotationMetrics(t *testing.T) {
	agentMetricsConsumer := globalSinks.agentMetricsConsumer

	metricNames := []string{
		"istio_agent_cert_expiry_seconds",
		"istio_agent_endpoint_no_pod",
		"istio_agent_go_gc_cycles_automatic_gc_cycles_total",
		"istio_agent_go_gc_cycles_forced_gc_cycles_total",
		"istio_agent_go_gc_cycles_total_gc_cycles_total",
		"istio_agent_go_gc_duration_seconds_sum",
		"istio_agent_go_gc_duration_seconds_count",
		"istio_agent_go_gc_heap_allocs_by_size_bytes_total_bucket",
		"istio_agent_go_gc_heap_allocs_by_size_bytes_total_sum",
		"istio_agent_go_gc_heap_allocs_by_size_bytes_total_count",
	}
	// when scraping via prometheus.io/scrape annotation, no additional attributes are present.
	checkMetricsAreEmitted(t, agentMetricsConsumer, metricNames, func(attrs pcommon.Map) bool {
		_, podLabelPresent := attrs.Get("pod")
		_, serviceLabelPresent := attrs.Get("service")
		return !podLabelPresent && !serviceLabelPresent
	})
	// when scraping via pod monitor, the pod attribute refers to the pod the metric is scraped from.
	checkMetricsAreEmitted(t, agentMetricsConsumer, metricNames, func(attrs pcommon.Map) bool {
		_, podLabelPresent := attrs.Get("pod")
		_, serviceLabelPresent := attrs.Get("service")
		return podLabelPresent && !serviceLabelPresent
	})
	// when scraping via service monitor, the pod attribute refers to the pod the metric is scraped from,
	// and the servicelabel attribute is added by the serviceMonitor definition.
	checkMetricsAreEmitted(t, agentMetricsConsumer, metricNames, func(attrs pcommon.Map) bool {
		_, podLabelPresent := attrs.Get("pod")
		_, serviceLabelPresent := attrs.Get("service")
		return podLabelPresent && serviceLabelPresent
	})
}

func selectMetricSet(expected pmetric.Metrics, metricName string, metricSink *consumertest.MetricsSink, ignoreLen bool) *pmetric.Metrics {
	for h := len(metricSink.AllMetrics()) - 1; h >= 0; h-- {
		m := metricSink.AllMetrics()[h]
		foundCorrectSet := false
	OUTER:
		for i := 0; i < m.ResourceMetrics().Len(); i++ {
			for j := 0; j < m.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
				for k := 0; k < m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
					metricToConsider := m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
					if metricToConsider.Name() == metricName {
						foundCorrectSet = true
						break OUTER
					}
				}
			}
		}
		if !foundCorrectSet {
			continue
		}
		if ignoreLen || m.ResourceMetrics().Len() == expected.ResourceMetrics().Len() && m.MetricCount() == expected.MetricCount() {
			return &m
		}
	}
	return nil
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
	checkMetricsAreEmitted(t, hecMetricsConsumer, metricNames, nil)
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

func checkMetricsAreEmitted(t *testing.T, mc *consumertest.MetricsSink, metricNames []string, matchFn func(pcommon.Map) bool) {
	metricsToFind := map[string]bool{}
	for _, name := range metricNames {
		metricsToFind[name] = false
	}
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		for _, m := range mc.AllMetrics() {
			for i := 0; i < m.ResourceMetrics().Len(); i++ {
				rm := m.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						metric := sm.Metrics().At(k)
						var attrs pcommon.Map
						switch metric.Type() {
						case pmetric.MetricTypeGauge:
							attrs = metric.Gauge().DataPoints().At(0).Attributes()
						case pmetric.MetricTypeSum:
							attrs = metric.Sum().DataPoints().At(0).Attributes()
						case pmetric.MetricTypeHistogram:
							attrs = metric.Histogram().DataPoints().At(0).Attributes()
						case pmetric.MetricTypeExponentialHistogram:
							attrs = metric.ExponentialHistogram().DataPoints().At(0).Attributes()
						default:
							panic("Unsupported type " + metric.Type().String())
						}
						if matchFn == nil || matchFn(attrs) {
							metricsToFind[metric.Name()] = true
						}
					}
				}
			}
		}
		var stillMissing []string
		var found []string
		missingCount := 0
		foundCount := 0
		for _, name := range metricNames {
			if !metricsToFind[name] {
				stillMissing = append(stillMissing, name)
				missingCount++
			} else {
				found = append(found, name)
				foundCount++
			}
		}
		t.Logf("Found: %s", strings.Join(found, ","))
		t.Logf("Metrics found: %d, metrics still missing: %d\n%s\n", foundCount, missingCount, strings.Join(stillMissing, ","))
		return missingCount == 0
	}, time.Duration(timeoutMinutes)*time.Minute, 10*time.Second,
		"failed to receive all metrics %d minutes", timeoutMinutes)
}

func maskScopeVersion(traces ptrace.Traces) {
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			ss.Scope().SetVersion("")
		}
	}
}

func maskSpanParentID(traces ptrace.Traces) {
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				span.SetParentSpanID(pcommon.NewSpanIDEmpty())
			}
		}
	}
}
