package e2e_tests

import (
	"bytes"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
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
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
)

const testKubeConfig = "/tmp/kube-config-splunk-otel-collector-chart-e2e-testing"

// TestTracesReception tests the chart with a real k8s cluster.
// Run the following command prior to running the test locally:
//
//	kind create cluster --kubeconfig=/tmp/kube-config-splunk-otel-collector-chart-e2e-testing
func TestTracesReception(t *testing.T) {
	// cannot patch "sock-splunk-otel-collector" with kind Instrumentation: Internal error occurred: failed calling webhook "minstrumentation.kb.io": failed to call webhook: Post "https://sock-operator-webhook.default.svc:443/mutate-opentelemetry-io-v1alpha1-instrumentation?timeout=10s": dial tcp 10.96.245.118:443: connect: connection refused
	t.Skip("Issue with deploying the operator on kind, skipping")
	var expected ptrace.Traces
	expectedFile := filepath.Join("testdata", "expected.yaml")
	expected, err := readTraces(expectedFile)
	require.NoError(t, err)
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	chartPath := filepath.Join("..", "helm-charts", "splunk-otel-collector")
	chart, err := loader.Load(chartPath)
	require.NoError(t, err)
	valuesBytes, err := os.ReadFile(filepath.Join("testdata", "operator_values.yaml"))
	require.NoError(t, err)
	valuesStr := strings.Replace(string(valuesBytes), "$ENDPOINT", fmt.Sprintf("%s:4317", hostEndpoint(t)), 1)
	var values map[string]interface{}
	err = yaml.Unmarshal([]byte(valuesStr), &values)
	require.NoError(t, err)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Printf(format, v)
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

	deployments := clientset.AppsV1().Deployments("default")

	var deployment *appsv1.Deployment
	b, err := os.ReadFile(filepath.Join("testdata", "nodejs", "deployment.yaml"))
	require.NoError(t, err)
	err = yaml.Unmarshal(b, &deployment)
	require.NoError(t, err)
	_, err = deployments.Create(context.Background(), deployment, metav1.CreateOptions{})
	require.NoError(t, err)

	tracesConsumer := new(consumertest.TracesSink)
	wantEntries := 3 // Minimal number of traces to wait for.
	waitForTraces(t, wantEntries, tracesConsumer)

	err = writeTraces(t, filepath.Join("testdata", "expected.yaml"), tracesConsumer.AllTraces()[len(tracesConsumer.AllTraces())-1])
	require.NoError(t, err)
	require.NoError(t, ptracetest.CompareTraces(expected, tracesConsumer.AllTraces()[len(tracesConsumer.AllTraces())-1],
		ptracetest.IgnoreSpansOrder(),
		ptracetest.IgnoreResourceSpansOrder(),
	),
	)
}

// TestClusterReceiverReception tests the chart with a real k8s cluster.
// Run the following command prior to running the test locally:
//
//	kind create cluster --kubeconfig=/tmp/kube-config-splunk-otel-collector-chart-e2e-testing
func TestClusterReceiverReception(t *testing.T) {
	expectedFile := filepath.Join("testdata", "expected_cluster_receiver.yaml")
	expected, err := readMetrics(expectedFile)
	require.NoError(t, err)
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	chartPath := filepath.Join("..", "helm-charts", "splunk-otel-collector")
	chart, err := loader.Load(chartPath)
	require.NoError(t, err)

	valuesBytes, err := os.ReadFile(filepath.Join("testdata", "cluster_receiver_values.yaml"))
	require.NoError(t, err)
	valuesStr := strings.Replace(string(valuesBytes), "$ENDPOINT", fmt.Sprintf("%s:4317", hostEndpoint(t)), 1)
	var values map[string]interface{}
	err = yaml.Unmarshal([]byte(valuesStr), &values)
	require.NoError(t, err)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Sprintf(format, v)
	}); err != nil {
		require.NoError(t, err)
	}
	install := action.NewInstall(actionConfig)
	install.Namespace = "default"
	install.ReleaseName = "sock"
	_, err = install.Run(chart, values)
	require.NoError(t, err)

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

	metricsConsumer := new(consumertest.MetricsSink)
	wantEntries := 3 // Minimal number of metrics to wait for.
	waitForMetrics(t, wantEntries, metricsConsumer)
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

		return value
	}
	containerImageShorten := func(value string) string {
		return value[(strings.LastIndex(value, "/") + 1):]
	}
	require.NoError(t, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
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

func waitForTraces(t *testing.T, entriesNum int, tc *consumertest.TracesSink) {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	rcvr, err := f.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, tc)
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating traces receiver")
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(tc.AllTraces()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d traces in %d minutes", entriesNum,
		len(tc.AllTraces()), timeoutMinutes)
}

func waitForMetrics(t *testing.T, entriesNum int, mc *consumertest.MetricsSink) {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	rcvr, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mc)
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating traces receiver")
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics in %d minutes", entriesNum,
		len(mc.AllMetrics()), timeoutMinutes)
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

// writeTraces writes a ptrace.Traces to the specified file in YAML format.
func writeTraces(t *testing.T, filePath string, traces ptrace.Traces) error {
	unmarshaler := &ptrace.JSONMarshaler{}
	fileBytes, err := unmarshaler.MarshalTraces(traces)
	if err != nil {
		return err
	}
	var jsonVal map[string]interface{}
	if err = json.Unmarshal(fileBytes, &jsonVal); err != nil {
		return err
	}
	b := &bytes.Buffer{}
	enc := yaml.NewEncoder(b)
	enc.SetIndent(2)
	if err := enc.Encode(jsonVal); err != nil {
		return err
	}
	if err := os.WriteFile(filePath, b.Bytes(), 0600); err != nil {
		return err
	}
	t.Logf("Golden file successfully written to %s.", filePath)
	t.Log("NOTE: The wwriteTraces call must be removed in order to pass the test.")
	t.Fail()
	return nil
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
