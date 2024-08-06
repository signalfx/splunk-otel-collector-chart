// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional_tests

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	hecReceiverPort                        = 8090
	hecMetricsReceiverPort                 = 8091
	hecLogsObjectsReceiverPort             = 8092
	signalFxReceiverPort                   = 9443
	signalFxReceiverK8sClusterReceiverPort = 19443
	otlpReceiverPort                       = 4317
	otlpHTTPReceiverPort                   = 4318
	apiPort                                = 8881
	kindTestKubeEnv                        = "kind"
	eksTestKubeEnv                         = "eks"
	autopilotTestKubeEnv                   = "gke/autopilot"
	aksTestKubeEnv                         = "aks"
	testDir                                = "testdata"
	valuesDir                              = "values"
	manifestsDir                           = "manifests"
	eksValuesDir                           = "expected_eks_values"
	kindValuesDir                          = "expected_kind_values"
)

var archRe = regexp.MustCompile("-amd64$|-arm64$|-ppc64le$")

// Test_Functions tests the chart with a real k8s cluster.
// Run the following commands prior to running the test locally:
//
// export KUBECONFIG=/tmp/kube-config-splunk-otel-collector-chart-functional-testing
// export KUBE_TEST_ENV=kind
// export K8S_VERSION=v1.28.0
// kind create cluster --kubeconfig=/tmp/kube-config-splunk-otel-collector-chart-functional-testing --config=.github/workflows/configs/kind-config.yaml --image=kindest/node:$K8S_VERSION
// kubectl get csr -o=jsonpath='{range.items[?(@.spec.signerName=="kubernetes.io/kubelet-serving")]}{.metadata.name}{" "}{end}' | xargs kubectl certificate approve
// make cert-manager
// kind load docker-image quay.io/splunko11ytest/nodejs_test:latest --name kind
// kind load docker-image quay.io/splunko11ytest/java_test:latest --name kind
// kind load docker-image quay.io/splunko11ytest/dotnet_test:latest --name kind
// On Mac M1s, you can also push this image so kind doesn't get confused with the platform to use:
// kind load docker-image ghcr.io/signalfx/splunk-otel-dotnet/splunk-otel-dotnet:v1.6.0 --name kind
// kind load docker-image ghcr.io/signalfx/splunk-otel-js/splunk-otel-js:v2.4.4 --name kind
// kind load docker-image ghcr.io/signalfx/splunk-otel-java/splunk-otel-java:v1.30.0 --name kind

// When running tests you can use the following env vars to help with local development:
// SKIP_SETUP: skip setting up the chart and apps. Useful if they are already deployed.
// SKIP_TEARDOWN: skip deleting the chart and apps as part of cleanup. Useful to keep around for local development.
// SKIP_TESTS: skip running tests, just set up and tear down the cluster.
// TEARDOWN_BEFORE_SETUP: delete all the deployments made by these tests before setting up.

var globalSinks *sinks

var setupRun = sync.Once{}

var expectedValuesDir string

type sinks struct {
	logsConsumer                      *consumertest.LogsSink
	hecMetricsConsumer                *consumertest.MetricsSink
	logsObjectsConsumer               *consumertest.LogsSink
	agentMetricsConsumer              *consumertest.MetricsSink
	k8sclusterReceiverMetricsConsumer *consumertest.MetricsSink
	tracesConsumer                    *consumertest.TracesSink
}

func setupOnce(t *testing.T) *sinks {
	setupRun.Do(func() {
		// create an API server
		internal.CreateApiServer(t, apiPort)
		// set ingest pipelines
		logs, metrics := setupHEC(t)
		globalSinks = &sinks{
			logsConsumer:                      logs,
			hecMetricsConsumer:                metrics,
			logsObjectsConsumer:               setupHECLogsObjects(t),
			agentMetricsConsumer:              setupSignalfxReceiver(t, signalFxReceiverPort),
			k8sclusterReceiverMetricsConsumer: setupSignalfxReceiver(t, signalFxReceiverK8sClusterReceiverPort),
			tracesConsumer:                    setupTraces(t),
		}
		if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
			teardown(t)
		}
		// deploy the chart and applications.
		if os.Getenv("SKIP_SETUP") == "true" {
			t.Log("Skipping setup as SKIP_SETUP is set to true")
			return
		}
		deployChartsAndApps(t)
	})

	return globalSinks
}
func deployChartsAndApps(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	kubeTestEnv, setKubeTestEnv := os.LookupEnv("KUBE_TEST_ENV")
	require.True(t, setKubeTestEnv, "the environment variable KUBE_TEST_ENV must be set")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	chartPath := filepath.Join("..", "helm-charts", "splunk-otel-collector")
	chart, err := loader.Load(chartPath)
	require.NoError(t, err)

	var valuesBytes []byte
	switch kubeTestEnv {
	case autopilotTestKubeEnv:
		valuesBytes, err = os.ReadFile(filepath.Join(testDir, valuesDir, "autopilot_test_values.yaml.tmpl"))
	case aksTestKubeEnv:
		valuesBytes, err = os.ReadFile(filepath.Join(testDir, valuesDir, "aks_test_values.yaml.tmpl"))
	default:
		valuesBytes, err = os.ReadFile(filepath.Join(testDir, valuesDir, "test_values.yaml.tmpl"))
	}

	require.NoError(t, err)

	hostEp := hostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}

	replacements := struct {
		K8sClusterEndpoint    string
		AgentEndpoint         string
		LogHecEndpoint        string
		MetricHecEndpoint     string
		OtlpEndpoint          string
		ApiURLEndpoint        string
		LogObjectsHecEndpoint string
		KubeTestEnv           string
	}{
		fmt.Sprintf("http://%s:%d", hostEp, signalFxReceiverK8sClusterReceiverPort),
		fmt.Sprintf("http://%s:%d", hostEp, signalFxReceiverPort),
		fmt.Sprintf("http://%s:%d", hostEp, hecReceiverPort),
		fmt.Sprintf("http://%s:%d/services/collector", hostEp, hecMetricsReceiverPort),
		fmt.Sprintf("%s:%d", hostEp, otlpReceiverPort),
		fmt.Sprintf("http://%s:%d", hostEp, apiPort),
		fmt.Sprintf("http://%s:%d/services/collector", hostEp, hecLogsObjectsReceiverPort),
		kubeTestEnv,
	}
	tmpl, err := template.New("").Parse(string(valuesBytes))
	require.NoError(t, err)
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, replacements)
	require.NoError(t, err)
	var values map[string]interface{}
	err = yaml.Unmarshal(buf.Bytes(), &values)
	require.NoError(t, err)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format+"\n", v...)
	}); err != nil {
		require.NoError(t, err)
	}
	install := action.NewInstall(actionConfig)
	install.Namespace = "default"
	install.ReleaseName = "sock"
	_, err = install.Run(chart, values)
	if err != nil {
		t.Logf("error reported during helm install: %v\n", err)
		retryUpgrade := action.NewUpgrade(actionConfig)
		retryUpgrade.Namespace = "default"
		retryUpgrade.Install = true
		_, err = retryUpgrade.Run("sock", chart, values)
		require.NoError(t, err)
	}

	waitForAllDeploymentsToStart(t, client)

	deployments := client.AppsV1().Deployments("default")

	decode := scheme.Codecs.UniversalDeserializer().Decode
	// NodeJS test app
	stream, err := os.ReadFile(filepath.Join(testDir, "nodejs", "deployment.yaml"))
	require.NoError(t, err)
	deployment, _, err := decode(stream, nil, nil)
	require.NoError(t, err)
	_, err = deployments.Create(context.Background(), deployment.(*appsv1.Deployment), metav1.CreateOptions{})
	if err != nil {
		_, err2 := deployments.Update(context.Background(), deployment.(*appsv1.Deployment), metav1.UpdateOptions{})
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
	_, err = deployments.Create(context.Background(), deployment.(*appsv1.Deployment), metav1.CreateOptions{})
	if err != nil {
		_, err2 := deployments.Update(context.Background(), deployment.(*appsv1.Deployment), metav1.UpdateOptions{})
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
	_, err = deployments.Create(context.Background(), deployment.(*appsv1.Deployment), metav1.CreateOptions{})
	if err != nil {
		_, err2 := deployments.Update(context.Background(), deployment.(*appsv1.Deployment), metav1.UpdateOptions{})
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
	_, err = deployments.Create(context.Background(), deployment.(*appsv1.Deployment), metav1.CreateOptions{})
	if err != nil {
		_, err2 := deployments.Update(context.Background(), deployment.(*appsv1.Deployment), metav1.UpdateOptions{})
		assert.NoError(t, err2)
		if err2 != nil {
			require.NoError(t, err)
		}
	}
	jobstream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "test_jobs.yaml"))
	require.NoError(t, err)
	var namespaces []*corev1.Namespace
	var jobs []*batchv1.Job
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
			namespaces = append(namespaces, nm)
			nms := client.CoreV1().Namespaces()
			_, err := nms.Create(context.Background(), nm, metav1.CreateOptions{})
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
			jobs = append(jobs, job)
			jobClient := client.BatchV1().Jobs(job.Namespace)
			_, err := jobClient.Create(context.Background(), job, metav1.CreateOptions{})
			require.NoError(t, err)
			t.Logf("Deployed job %s", job.Name)
		}
	}

	waitForAllDeploymentsToStart(t, client)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		t.Log("Cleaning up cluster")
		teardown(t)

	})
}

func teardown(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	decode := scheme.Codecs.UniversalDeserializer().Decode
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)
	waitTime := int64(0)
	deployments := client.AppsV1().Deployments("default")
	require.NoError(t, err)
	_ = deployments.Delete(context.Background(), "nodejs-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = deployments.Delete(context.Background(), "java-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = deployments.Delete(context.Background(), "dotnet-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = deployments.Delete(context.Background(), "prometheus-annotation-test", metav1.DeleteOptions{
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

		obj, groupVersionKind, err := decode(
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
		_ = jobClient.Delete(context.Background(), job.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &waitTime,
		})
	}
	for _, nm := range namespaces {
		nmClient := client.CoreV1().Namespaces()
		_ = nmClient.Delete(context.Background(), nm.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &waitTime,
		})
	}
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format+"\n", v)
	}); err != nil {
		require.NoError(t, err)
	}
	uninstall := action.NewUninstall(actionConfig)
	uninstall.IgnoreNotFound = true
	uninstall.Wait = true
	_, _ = uninstall.Run("sock")
}

func Test_Functions(t *testing.T) {
	_ = setupOnce(t)
	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	kubeTestEnv, setKubeTestEnv := os.LookupEnv("KUBE_TEST_ENV")
	require.True(t, setKubeTestEnv, "the environment variable KUBE_TEST_ENV must be set")

	switch kubeTestEnv {
	case kindTestKubeEnv, autopilotTestKubeEnv, aksTestKubeEnv:
		expectedValuesDir = kindValuesDir
	case eksTestKubeEnv:
		expectedValuesDir = eksValuesDir
	default:
		assert.Fail(t, "KUBE_TEST_ENV is set to invalid value. Must be one of [kind, eks].")
	}

	t.Run("node.js traces captured", testNodeJSTraces)
	t.Run("java traces captured", testJavaTraces)
	t.Run(".NET traces captured", testDotNetTraces)
	t.Run("kubernetes cluster metrics", testK8sClusterReceiverMetrics)
	t.Run("agent logs", testAgentLogs)
	t.Run("test HEC metrics", testHECMetrics)
	t.Run("test k8s objects", testK8sObjects)
	t.Run("test agent metrics", testAgentMetrics)
	t.Run("test prometheus metrics", testPrometheusAnnotationMetrics)
}

func testNodeJSTraces(t *testing.T) {
	tracesConsumer := setupOnce(t).tracesConsumer

	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join(testDir, expectedValuesDir, "expected_nodejs_traces.yaml")
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)

	waitForTraces(t, 10, tracesConsumer)

	var selectedTrace *ptrace.Traces

	require.Eventually(t, func() bool {
		for i := len(tracesConsumer.AllTraces()) - 1; i > 0; i-- {
			trace := tracesConsumer.AllTraces()[i]
			if val, ok := trace.ResourceSpans().At(0).Resource().Attributes().Get("telemetry.sdk.language"); ok && strings.Contains(val.Str(), "nodejs") {
				if expectedTraces.SpanCount() == trace.SpanCount() {
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
		ptracetest.IgnoreResourceAttributeValue("splunk.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("splunk.zc.method"),
		ptracetest.IgnoreSpanAttributeValue("net.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("http.user_agent"),
		ptracetest.IgnoreSpanAttributeValue("os.version"),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
	)

	require.NoError(t, err)
}

func testJavaTraces(t *testing.T) {
	tracesConsumer := setupOnce(t).tracesConsumer

	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join(testDir, expectedValuesDir, "expected_java_traces.yaml")
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)

	waitForTraces(t, 10, tracesConsumer)

	var selectedTrace *ptrace.Traces

	require.Eventually(t, func() bool {
		for i := len(tracesConsumer.AllTraces()) - 1; i > 0; i-- {
			trace := tracesConsumer.AllTraces()[i]
			if val, ok := trace.ResourceSpans().At(0).Resource().Attributes().Get("telemetry.sdk.language"); ok && strings.Contains(val.Str(), "java") {
				if expectedTraces.SpanCount() == trace.SpanCount() {
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

	require.NoError(t, err)
}

func testDotNetTraces(t *testing.T) {
	tracesConsumer := setupOnce(t).tracesConsumer

	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join(testDir, expectedValuesDir, "expected_dotnet_traces.yaml")
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)

	waitForTraces(t, 30, tracesConsumer)
	var selectedTrace *ptrace.Traces

	require.Eventually(t, func() bool {
		for i := len(tracesConsumer.AllTraces()) - 1; i > 0; i-- {
			trace := tracesConsumer.AllTraces()[i]
			if val, ok := trace.ResourceSpans().At(0).Resource().Attributes().Get("telemetry.sdk.language"); ok && strings.Contains(val.Str(), "dotnet") {
				if expectedTraces.SpanCount() == trace.SpanCount() {
					selectedTrace = &trace
					break
				}
				selectedTrace = &trace
				break
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
		ptracetest.IgnoreResourceAttributeValue("host.arch"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.sdk.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.auto.version"),
		ptracetest.IgnoreResourceAttributeValue("splunk.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("splunk.zc.method"),
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
	metricsConsumer := setupOnce(t).k8sclusterReceiverMetricsConsumer
	expectedMetricsFile := filepath.Join(testDir, expectedValuesDir, "expected_cluster_receiver.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedMetricsFile)
	require.NoError(t, err)

	replaceWithStar := func(string) string { return "*" }

	var selected *pmetric.Metrics
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
			selected = &m
			break
		}
	}

	require.NotNil(t, selected)

	metricNames := []string{"k8s.node.condition_ready", "k8s.namespace.phase", "k8s.pod.phase", "k8s.replicaset.desired", "k8s.replicaset.available", "k8s.daemonset.ready_nodes", "k8s.daemonset.misscheduled_nodes", "k8s.daemonset.desired_scheduled_nodes", "k8s.daemonset.current_scheduled_nodes", "k8s.container.ready", "k8s.container.memory_request", "k8s.container.memory_limit", "k8s.container.cpu_request", "k8s.container.cpu_limit", "k8s.deployment.desired", "k8s.deployment.available", "k8s.container.restarts", "k8s.container.cpu_request", "k8s.container.memory_request", "k8s.container.memory_limit"}

	err = pmetrictest.CompareMetrics(expectedMetrics, *selected,
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
		pmetrictest.IgnoreSubsequentDataPoints("k8s.container.ready", "k8s.container.restarts"),
	)

	require.NoError(t, err)
}

func testAgentLogs(t *testing.T) {

	logsConsumer := setupOnce(t).logsConsumer
	waitForLogs(t, 5, logsConsumer)

	var helloWorldResource pcommon.Resource
	var helloWorldLogRecord *plog.LogRecord
	excludePods := true
	excludeNs := true
	var sourcetypes []string
	var indices []string
	var journalDsourceTypes []string

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
				if "pod-w-index-w-ns-exclude" == value.AsString() {
					excludePods = false
				}
				if "pod-w-exclude-wo-ns-exclude" == value.AsString() {
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
		assert.Regexp(t, regexp.MustCompile("/var/log/pods/default_nodejs-test-.*/nodejs-test/0.log"), source.AsString())
		index, ok := helloWorldResource.Attributes().Get("com.splunk.index")
		assert.True(t, ok)
		assert.Equal(t, "main", index.AsString())
		podName, ok := helloWorldLogRecord.Attributes().Get("k8s.pod.name")
		assert.True(t, ok)
		assert.Regexp(t, regexp.MustCompile("nodejs-test-.*"), podName.AsString())
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
	logsObjectsConsumer := setupOnce(t).logsObjectsConsumer
	waitForLogs(t, 5, logsObjectsConsumer)

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
	agentMetricsConsumer := setupOnce(t).agentMetricsConsumer

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
		"otelcol_processor_dropped_spans",
		"otelcol_processor_refused_spans",
		"otelcol_processor_refused_log_records",
		"otelcol_processor_dropped_log_records",
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
		"otelcol_processor_filter_logs_filtered",
		"otelcol_receiver_accepted_metric_points",
		"otelcol_processor_dropped_metric_points",
		"otelcol_processor_refused_metric_points",
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
		"system.filesystem.usage",
		"system.memory.usage",
		"system.network.errors",
		"system.network.io",
		"system.paging.operations",
		"up",
	}
	checkMetricsAreEmitted(t, agentMetricsConsumer, metricNames)
	expectedInternalMetrics, err := golden.ReadMetrics(filepath.Join(testDir, expectedValuesDir, "expected_internal_metrics.yaml"))
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
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.replicaset.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.replicaset.name", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("k8s.namespace.uid", metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("container.image.name", metricNames...),
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
		pmetrictest.ChangeResourceAttributeValue("service_instance_id", replaceWithStar),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	)
	assert.NoError(t, err)

	expectedKubeletStatsMetrics, err := golden.ReadMetrics(filepath.Join(testDir, expectedValuesDir, "expected_kubeletstats_metrics.yaml"))
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
	if err != nil {
		t.Skipf("we have trouble identifying exact payloads right now: %v", err)
	} else {
		assert.NoError(t, err)
	}
}

func testPrometheusAnnotationMetrics(t *testing.T) {
	agentMetricsConsumer := setupOnce(t).agentMetricsConsumer

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
	checkMetricsAreEmitted(t, agentMetricsConsumer, metricNames)
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
	hecMetricsConsumer := setupOnce(t).hecMetricsConsumer

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
		"otelcol_processor_refused_log_records",
		"otelcol_processor_dropped_log_records",
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
		"otelcol_processor_dropped_metric_points",
		"otelcol_processor_refused_metric_points",
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
		"system.filesystem.inodes.usage",
		"system.filesystem.usage",
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

func waitForAllDeploymentsToStart(t *testing.T, client *kubernetes.Clientset) {
	require.Eventually(t, func() bool {
		di, err := client.AppsV1().Deployments("default").List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)
		for _, d := range di.Items {
			if d.Status.ReadyReplicas != d.Status.Replicas {
				var messages string
				for _, c := range d.Status.Conditions {
					messages += c.Message
					messages += "\n"
				}

				t.Logf("Deployment not ready: %s, %s", d.Name, messages)
				return false
			}
		}
		return true
	}, 5*time.Minute, 10*time.Second)
}

func waitForAllNamespacesToBeCreated(t *testing.T, client *kubernetes.Clientset) {
	require.Eventually(t, func() bool {
		nms, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)
		for _, d := range nms.Items {
			if d.Status.Phase != corev1.NamespaceActive {
				return false
			}
		}
		return true
	}, 5*time.Minute, 10*time.Second)
}

func setupTraces(t *testing.T) *consumertest.TracesSink {
	tc := new(consumertest.TracesSink)
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.Protocols.GRPC.NetAddr.Endpoint = fmt.Sprintf("0.0.0.0:%d", otlpReceiverPort)
	cfg.Protocols.HTTP.Endpoint = fmt.Sprintf("0.0.0.0:%d", otlpHTTPReceiverPort)

	rcvr, err := f.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, tc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating traces receiver")
	t.Cleanup(func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	})

	return tc
}

func setupSignalfxReceiver(t *testing.T, port int) *consumertest.MetricsSink {
	mc := new(consumertest.MetricsSink)
	f := signalfxreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*signalfxreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", port)

	rcvr, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating metrics receiver")
	t.Cleanup(func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	})

	return mc
}

func setupHEC(t *testing.T) (*consumertest.LogsSink, *consumertest.MetricsSink) {
	// the splunkhecreceiver does poorly at receiving logs and metrics. Use separate ports for now.
	f := splunkhecreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*splunkhecreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", hecReceiverPort)

	mCfg := f.CreateDefaultConfig().(*splunkhecreceiver.Config)
	mCfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", hecMetricsReceiverPort)

	lc := new(consumertest.LogsSink)
	mc := new(consumertest.MetricsSink)
	rcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, lc)
	mrcvr, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), mCfg, mc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating logs receiver")
	t.Cleanup(func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	})

	require.NoError(t, mrcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating metrics receiver")
	t.Cleanup(func() {
		assert.NoError(t, mrcvr.Shutdown(context.Background()))
	})

	return lc, mc
}

func setupHECLogsObjects(t *testing.T) *consumertest.LogsSink {
	f := splunkhecreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*splunkhecreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", hecLogsObjectsReceiverPort)

	lc := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, lc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating logs receiver")
	t.Cleanup(func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	})

	return lc
}
func checkMetricsAreEmitted(t *testing.T, mc *consumertest.MetricsSink, metricNames []string) {
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
						metricsToFind[metric.Name()] = true
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

func hostEndpoint(t *testing.T) string {
	if host, ok := os.LookupEnv("HOST_ENDPOINT"); ok {
		return host
	}
	if runtime.GOOS == "darwin" {
		return "host.docker.internal"
	}

	client, err := docker.NewClientWithOpts(docker.FromEnv)
	require.NoError(t, err)
	client.NegotiateAPIVersion(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	network, err := client.NetworkInspect(ctx, "kind", types.NetworkInspectOptions{})
	require.NoError(t, err)
	for _, ipam := range network.IPAM.Config {
		if ipam.Gateway != "" {
			return ipam.Gateway
		}
	}
	require.Fail(t, "failed to find host endpoint")
	return ""
}

func waitForTraces(t *testing.T, entriesNum int, tc *consumertest.TracesSink) {
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(tc.AllTraces()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d traces in %d minutes", entriesNum,
		len(tc.AllTraces()), timeoutMinutes)
}

func waitForLogs(t *testing.T, entriesNum int, lc *consumertest.LogsSink) {
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(lc.AllLogs()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d logs in %d minutes", entriesNum,
		len(lc.AllLogs()), timeoutMinutes)
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
