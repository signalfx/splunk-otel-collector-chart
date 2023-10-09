// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional_tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
)

const testKubeConfig = "/tmp/kube-config-splunk-otel-collector-chart-functional-testing"

// Test_Functions tests the chart with a real k8s cluster.
// Run the following command prior to running the test locally:
//
// export K8S_VERSION=v1.28.0
// kind create cluster --kubeconfig=/tmp/kube-config-splunk-otel-collector-chart-functional-testing --config=.github/workflows/configs/kind-config.yaml --image=kindest/node:$K8S_VERSION
// cd functional_tests/testdata/nodejs
// docker build -t nodejs_test:latest .
// kind load docker-image nodejs_test:latest --name kind
func Test_Functions(t *testing.T) {
	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join("testdata", "expected_traces.yaml")
	expectedTraces, err := readTraces(expectedTracesFile)
	require.NoError(t, err)

	expectedMetricsFile := filepath.Join("testdata", "expected_cluster_receiver.yaml")
	expectedMetrics, err := readMetrics(expectedMetricsFile)
	require.NoError(t, err)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	chartPath := filepath.Join("..", "helm-charts", "splunk-otel-collector")
	chart, err := loader.Load(chartPath)
	require.NoError(t, err)
	valuesBytes, err := os.ReadFile(filepath.Join("testdata", "test_values.yaml"))
	require.NoError(t, err)
	valuesStr := strings.ReplaceAll(string(valuesBytes), "$ENDPOINT", fmt.Sprintf("%s:4317", hostEndpoint(t)))
	var values map[string]interface{}
	err = yaml.Unmarshal([]byte(valuesStr), &values)
	require.NoError(t, err)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Printf(format+"\n", v)
	}); err != nil {
		require.NoError(t, err)
	}
	install := action.NewInstall(actionConfig)
	install.Namespace = "default"
	install.ReleaseName = "sock"
	_, err = install.Run(chart, values)
	if err != nil {
		fmt.Printf("error reported during helm install: %v\n", err)
		retryUpgrade := action.NewUpgrade(actionConfig)
		retryUpgrade.Namespace = "default"
		retryUpgrade.Install = true
		require.Eventually(t, func() bool {
			_, err = retryUpgrade.Run("sock", chart, values)
			if err != nil {
				fmt.Printf("error reported during helm upgrade: %v\n", err)
			}
			return err == nil
		}, 3*time.Minute, 30*time.Second)
	}

	waitForAllDeploymentsToStart(t, clientset)

	deployments := clientset.AppsV1().Deployments("default")

	decode := scheme.Codecs.UniversalDeserializer().Decode
	stream, err := os.ReadFile(filepath.Join("testdata", "nodejs", "deployment.yaml"))
	require.NoError(t, err)
	deployment, _, err := decode(stream, nil, nil)
	require.NoError(t, err)
	_, err = deployments.Create(context.Background(), deployment.(*appsv1.Deployment), metav1.CreateOptions{})
	require.NoError(t, err)

	waitForAllDeploymentsToStart(t, clientset)

	tracesConsumer := new(consumertest.TracesSink)
	metricsConsumer := new(consumertest.MetricsSink)
	logsConsumer := new(consumertest.LogsSink)
	wantEntries := 3 // Minimal number of traces, metrics, and logs to wait for.
	waitForData(t, wantEntries, tracesConsumer, metricsConsumer, logsConsumer)

	replaceWithStar := func(string) string { return "*" }
	shortenNames := func(value string) string {
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
	containerImageShorten := func(value string) string {
		return value[(strings.LastIndex(value, "/") + 1):]
	}

	latestTrace := tracesConsumer.AllTraces()[len(tracesConsumer.AllTraces())-1]
	actualSpan := latestTrace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	expectedSpan := expectedTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	expectedSpan.Attributes().Range(func(k string, _ pcommon.Value) bool {
		_, ok := actualSpan.Attributes().Get(k)
		assert.True(t, ok)
		return true
	})

	require.NoError(t,
		pmetrictest.CompareMetrics(expectedMetrics, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricValues("k8s.deployment.desired", "k8s.deployment.available", "k8s.container.restarts", "k8s.container.cpu_request", "k8s.container.memory_request", "k8s.container.memory_limit"),
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
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
		),
	)
}

func waitForAllDeploymentsToStart(t *testing.T, clientset *kubernetes.Clientset) {
	require.Eventually(t, func() bool {
		di, err := clientset.AppsV1().Deployments("default").List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)
		for _, d := range di.Items {
			if d.Status.ReadyReplicas != d.Status.Replicas {
				return false
			}
		}
		return true
	}, 5*time.Minute, 10*time.Second)
}

func waitForData(t *testing.T, entriesNum int, tc *consumertest.TracesSink, mc *consumertest.MetricsSink, lc *consumertest.LogsSink) {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	_, err := f.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, tc)
	require.NoError(t, err)
	_, err = f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mc)
	require.NoError(t, err)
	logsReceiver, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, lc)
	require.NoError(t, err)

	require.NoError(t, logsReceiver.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating logs receiver")
	defer func() {
		assert.NoError(t, logsReceiver.Shutdown(context.Background()))
	}()

	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(tc.AllTraces()) > entriesNum && len(mc.AllMetrics()) > entriesNum //&& len(lc.AllLogs()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d traces, %d metrics, %d logs in %d minutes", entriesNum,
		len(tc.AllTraces()), len(mc.AllMetrics()), len(lc.AllLogs()), timeoutMinutes)
}

func hostEndpoint(t *testing.T) string {
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
		return ipam.Gateway
	}
	require.Fail(t, "failed to find host endpoint")
	return ""
}

// readMetrics reads a pmetric.Metrics from the specified YAML or JSON file.
func readMetrics(filePath string) (pmetric.Metrics, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return pmetric.Metrics{}, err
	}
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		var m map[string]interface{}
		if err = yaml.Unmarshal(b, &m); err != nil {
			return pmetric.Metrics{}, err
		}
		b, err = json.Marshal(m)
		if err != nil {
			return pmetric.Metrics{}, err
		}
	}
	unmarshaller := &pmetric.JSONUnmarshaler{}
	return unmarshaller.UnmarshalMetrics(b)
}

// readTraces reads a ptrace.Traces from the specified YAML or JSON file.
func readTraces(filePath string) (ptrace.Traces, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return ptrace.Traces{}, err
	}
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		var m map[string]interface{}
		if err = yaml.Unmarshal(b, &m); err != nil {
			return ptrace.Traces{}, err
		}
		b, err = json.Marshal(m)
		if err != nil {
			return ptrace.Traces{}, err
		}
	}
	unmarshaler := ptrace.JSONUnmarshaler{}
	return unmarshaler.UnmarshalTraces(b)
}
