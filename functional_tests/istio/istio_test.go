// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const istioVersion = "1.27.1"

type request struct {
	url    string
	host   string
	header string
	path   string
}

// Env vars to control the test behavior
// TEARDOWN_BEFORE_SETUP: if set to true, the test will run teardown before setup
// SKIP_SETUP: if set to true, the test will skip setup
// SKIP_TEARDOWN: if set to true, the test will skip teardown
// SKIP_TESTS: if set to true, the test will skip the test
// UPDATE_EXPECTED_RESULTS: if set to true, the test will update the expected results
// KUBECONFIG: the path to the kubeconfig file

func deployIstioAndCollector(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	config, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)
	runCommand(t, "kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.3.0/standard-install.yaml")

	// Install Istio
	istioctlPath := downloadIstio(t)
	runCommand(t, fmt.Sprintf("%s install -y", istioctlPath))

	traceOperatorPath, err := filepath.Abs(filepath.Join("./", "testdata", "traceoperator.yaml"))
	require.NoError(t, err)
	runCommand(t, fmt.Sprintf("%s install -y -f %s", istioctlPath, traceOperatorPath))

	// Patch ingress gateway to work in kind cluster
	patchResource(t, clientset, "istio-system", "istio-ingressgateway", "deployments", `{"spec":{"template":{"spec":{"containers":[{"name":"istio-proxy","ports":[{"containerPort":8080,"hostPort":80},{"containerPort":8443,"hostPort":443}]}]}}}}`)
	patchResource(t, clientset, "istio-system", "istio-ingressgateway", "services", `{"spec": {"type": "ClusterIP"}}`)

	internal.CreateNamespace(t, clientset, "istio-workloads")
	internal.LabelNamespace(t, clientset, "istio-workloads", "istio-injection", "enabled")
	internal.LabelNamespace(t, clientset, "default", "istio-injection", "enabled")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	_, err = k8stest.CreateObjects(k8sClient, "testdata/testobjects")
	require.NoError(t, err)
	deployment, err := clientset.AppsV1().Deployments("istio-workloads").Get(t.Context(), "httpbin", metav1.GetOptions{})
	require.NoError(t, err, "failed to get httpbin deployment")
	t.Logf("Deployment %s created successfully", deployment.Name)

	internal.CheckPodsReady(t, clientset, "istio-system", "app=istio-ingressgateway", 5*time.Minute, 0)
	internal.CheckPodsReady(t, clientset, "istio-system", "app=istiod", 2*time.Minute, 0)
	internal.CheckPodsReady(t, clientset, "istio-workloads", "app=httpbin", 3*time.Minute, 0)

	// Send traffic through ingress gateways
	requests := []request{
		{"http://httpbin.example.com/status/200", "httpbin.example.com", "httpbin.example.com:80", "/status/200"},
		{"http://httpbin.example.com/status/404", "httpbin.example.com", "httpbin.example.com:80", "/status/404"},
		{"http://httpbin.example.com/delay/1", "httpbin.example.com", "httpbin.example.com:80", "/delay/0"},
	}
	sendWorkloadHTTPRequests(t, requests)

	valuesFile, err := filepath.Abs(filepath.Join("testdata", "istio_values.yaml.tmpl"))
	require.NoError(t, err)
	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}
	replacements := map[string]any{
		"IngestURL": fmt.Sprintf("http://%s:%d", hostEp, internal.SignalFxReceiverPort),
		"ApiURL":    fmt.Sprintf("http://%s:%d", hostEp, internal.SignalFxAPIPort),
	}
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 0, internal.GetDefaultChartOptions())

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		teardown(t, k8sClient, istioctlPath)
	})

	// Run after starting collector to make sure traces are generated
	//_, err = k8stest.CreateObjects(k8sClient, "testdata/testkindloadbalancer")
	//require.NoError(t, err)
	//
	//internal.CheckPodsReady(t, clientset, "default", "app=http-echo", 5*time.Minute, 0)
}

func teardown(t *testing.T, k8sClient *k8stest.K8sClient, istioctlPath string) {
	internal.DeleteObject(t, k8sClient, `
apiVersion: networking.istio.io/v1
kind: Gateway
metadata:
  name: httpbin-gateway
  namespace: istio-system
`)
	internal.DeleteObject(t, k8sClient, `
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: httpbin
  namespace: istio-system
`)
	internal.DeleteObject(t, k8sClient, `
apiVersion: v1
kind: Namespace
metadata:
  name: istio-workloads
`)
	internal.DeleteObject(t, k8sClient, `
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: httpbin
  namespace: istio-system
`)
	internal.DeleteObject(t, k8sClient, `
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: otel-demo
  namespace: istio-workloads
`)
	//	internal.DeleteObject(t, k8sClient, `
	//apiVersion: v1
	//kind: Service
	//metadata:
	//  name: httpbin
	//`)
	//	internal.DeleteObject(t, k8sClient, `
	//apiVersion: v1
	//kind: Deployment
	//metadata:
	//  name: httpbin
	//`)
	//	internal.DeleteObject(t, k8sClient, `
	//apiVersion: v1
	//kind: ServiceAccount
	//metadata:
	//  name: httpbin
	//`)
	runCommand(t, fmt.Sprintf("%s uninstall --purge -y", istioctlPath))

	testKubeConfig, _ := os.LookupEnv("KUBECONFIG")
	internal.ChartUninstall(t, testKubeConfig)
}

func downloadIstio(t *testing.T) string {
	var url string
	switch runtime.GOOS {
	case "darwin":
		url = fmt.Sprintf("https://github.com/istio/istio/releases/download/%s/istio-%s-osx.tar.gz", istioVersion, istioVersion)
	case "linux":
		url = fmt.Sprintf("https://github.com/istio/istio/releases/download/%s/istio-%s-linux-amd64.tar.gz", istioVersion, istioVersion)
	default:
		t.Fatalf("unsupported operating system: %s", runtime.GOOS)
	}

	resp, err := http.Get(url) //nolint:gosec
	require.NoError(t, err)
	defer resp.Body.Close()

	gz, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	defer gz.Close()

	tr := tar.NewReader(gz)
	var istioDir string
	for {
		var hdr *tar.Header
		hdr, err = tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		target := filepath.Join(".", hdr.Name) //nolint:gosec
		if hdr.FileInfo().IsDir() && istioDir == "" {
			istioDir = target
		}
		if hdr.FileInfo().IsDir() {
			require.NoError(t, os.MkdirAll(target, hdr.FileInfo().Mode()))
		} else {
			var f *os.File
			f, err = os.Create(target)
			require.NoError(t, err)
			defer f.Close()

			_, err = io.Copy(f, tr) //nolint:gosec
			require.NoError(t, err)
		}
	}
	require.NotEmpty(t, istioDir, "istioctl path not found")

	absIstioDir, err := filepath.Abs(istioDir)
	require.NoError(t, err, "failed to get absolute path for istioDir")

	istioctlPath := filepath.Join(absIstioDir, "bin", "istioctl")
	require.FileExists(t, istioctlPath, "istioctl binary not found")
	require.NoError(t, os.Chmod(istioctlPath, 0o755), "failed to set executable permission for istioctl")

	t.Cleanup(func() {
		os.RemoveAll(absIstioDir)
	})

	return istioctlPath
}

func runCommand(t *testing.T, command string) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "failed to run command: %s", command)
}

func patchResource(t *testing.T, clientset *kubernetes.Clientset, namespace, name, resourceType, patch string) {
	var err error
	switch resourceType {
	case "deployments":
		_, err = clientset.AppsV1().Deployments(namespace).Patch(t.Context(), name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
	case "services":
		_, err = clientset.CoreV1().Services(namespace).Patch(t.Context(), name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
	}
	require.NoError(t, err)
}

func sendHTTPRequest(t *testing.T, client *http.Client, url, host, header, path string) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Host = host
	req.Header.Set("Host", header)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	t.Logf("Response for %s: %s", path, string(body))
}

func sendWorkloadHTTPRequests(t *testing.T, requests []request) {
	resolveHost := "127.0.0.1"

	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr == "httpbin.example.com:80" {
			addr = resolveHost + ":80"
		}
		d := net.Dialer{}
		return d.DialContext(ctx, network, addr)
	}

	transport := &http.Transport{
		DialContext: dialContext,
	}

	client := &http.Client{
		Transport: transport,
	}

	for _, req := range requests {
		sendHTTPRequest(t, client, req.url, req.host, req.header, req.path)
	}
}

func Test_IstioMetrics(t *testing.T) {
	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		t.Log("Running teardown before setup as TEARDOWN_BEFORE_SETUP is set to true")
		testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
		require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
		k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
		require.NoError(t, err)
		istioctlPath := downloadIstio(t)
		teardown(t, k8sClient, istioctlPath)
	}

	// create an API server
	internal.SetupSignalFxAPIServer(t)
	metricsSink := internal.SetupSignalfxReceiver(t, internal.SignalFxReceiverPort)

	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		deployIstioAndCollector(t)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	flakyMetrics := []string{"galley_validation_config_update_error"} // only shows up when config validation fails - removed if present when comparing
	t.Run("istiod metrics captured", func(t *testing.T) {
		testIstioMetrics(t, "testdata/expected_istiod.yaml", "pilot_xds_pushes", flakyMetrics, true, metricsSink)
	})

	flakyMetrics = []string{"istio_agent_pilot_xds_expired_nonce"}
	t.Run("istio ingress metrics captured", func(t *testing.T) {
		testIstioMetrics(t, "testdata/expected_istioingress.yaml", "istio_requests_total", flakyMetrics, true, metricsSink)
	})
}

func testIstioMetrics(t *testing.T, expectedMetricsFile string, includeMetricName string, flakyMetricNames []string, ignoreLen bool, metricsSink *consumertest.MetricsSink) {
	expectedMetrics, err := golden.ReadMetrics(expectedMetricsFile)
	require.NoError(t, err)

	internal.WaitForMetrics(t, 2, metricsSink)

	selectedMetrics := internal.SelectMetricSetWithTimeout(t, expectedMetrics, includeMetricName, metricsSink, ignoreLen, 5*time.Minute, 30*time.Second)
	if selectedMetrics == nil {
		t.Error("No metric batch identified with the right metric count, exiting")
		return
	}
	require.NotNil(t, selectedMetrics)

	if flakyMetricNames != nil {
		internal.RemoveFlakyMetrics(selectedMetrics, flakyMetricNames)
	}

	var metricNames []string
	for i := 0; i < expectedMetrics.ResourceMetrics().Len(); i++ {
		for j := 0; j < expectedMetrics.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			for k := 0; k < expectedMetrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
				metric := expectedMetrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
				metricNames = append(metricNames, metric.Name())
			}
		}
	}

	internal.MaybeUpdateExpectedMetricsResults(t, expectedMetricsFile, selectedMetrics)
	err = pmetrictest.CompareMetrics(expectedMetrics, *selectedMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreMetricValues(metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("host.name"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreMetricAttributeValue("os.type"),
		pmetrictest.IgnoreMetricAttributeValue("server.address"),
		pmetrictest.IgnoreMetricAttributeValue("service.instance.id"),
		pmetrictest.IgnoreMetricAttributeValue("service.name"),
		pmetrictest.IgnoreMetricAttributeValue("url.scheme"),
		pmetrictest.IgnoreMetricAttributeValue("type", "pilot_xds_expired_nonce"),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreMetricAttributeValue("event"),
		pmetrictest.IgnoreSubsequentDataPoints(metricNames...),
	)
	require.NoError(t, err)
}

func generateIstioTraces(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	config, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	service, err := clientset.CoreV1().Services("default").Get(t.Context(), "foo-service", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, service.Status.LoadBalancer.Ingress)
	fooServiceIP := service.Status.LoadBalancer.Ingress[0].IP

	for i := 0; i < 10; i++ {
		runCommand(t, fmt.Sprintf("curl %s:5678", fooServiceIP))
	}
}

func Test_IstioTraces(t *testing.T) {
	// t.Setenv("KUBECONFIG", "/tmp/kube-config-splunk-otel-collector-chart-functional-testing")
	// t.Setenv("SKIP_TEARDOWN", "true")
	// t.Setenv("SKIP_SETUP", "true")
	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		t.Log("Running teardown before setup as TEARDOWN_BEFORE_SETUP is set to true")
		testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
		require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
		k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
		require.NoError(t, err)
		istioctlPath := downloadIstio(t)
		teardown(t, k8sClient, istioctlPath)
	}

	// create an API server
	tracesSink := internal.SetupOTLPTracesSinkWithTokenAndPorts(t, "CHANGEME", internal.OTLPGRPCReceiverPort, internal.SignalFxReceiverPort)

	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		deployIstioAndCollector(t)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	//t.Run("istiod traces captured", func(t *testing.T) {
	//	testIstioTraces(t, "testdata/expected_istio_traces.yaml", tracesSink)
	//})
	t.Run("test istio traces: httpbin traces captured", func(t *testing.T) {
		testIstioHTTPBinTraces(t, "testdata/expected_istio_httpbin_traces.yaml", tracesSink)
	})
}

func testIstioHTTPBinTraces(t *testing.T, expectedTracesFile string, tracesSink *consumertest.TracesSink) {
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)
	require.NotEmpty(t, expectedTraces)

	// Send traffic through ingress gateways
	requests := []request{
		{"http://httpbin.example.com/status/200", "httpbin.example.com", "httpbin.example.com:80", "/status/200"},
		//{"http://httpbin.example.com/delay/1", "httpbin.example.com", "httpbin.example.com:80", "/delay/0"},
	}
	sendWorkloadHTTPRequests(t, requests)

	require.Eventually(t, func() bool {
		foundTraces := false
		for _, receivedTraces := range tracesSink.AllTraces() {
			internal.MaybeWriteUpdateExpectedTracesResults(t, expectedTracesFile, &receivedTraces)

			err = ptracetest.CompareTraces(expectedTraces, receivedTraces,
				ptracetest.IgnoreResourceSpansOrder(),
				ptracetest.IgnoreSpansOrder(),
				ptracetest.IgnoreEndTimestamp(),
				ptracetest.IgnoreTraceID(),
				ptracetest.IgnoreSpanID(),
				ptracetest.IgnoreStartTimestamp(),
				ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
				ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
				ptracetest.IgnoreSpanAttributeValue("node_id"),
				// ptracetest.IgnoreSpanAttributeValue("http.status_code"),
				// ptracetest.IgnoreSpanAttributeValue("http.url"),
				ptracetest.IgnoreSpanAttributeValue("guid:x-request-id"),
				ptracetest.IgnoreSpanAttributeValue("peer.address"),
				ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
			)
			if err == nil {
				foundTraces = true
				break
			} else {
				t.Logf("Comparison error: %v", err)
			}
		}

		if !foundTraces {
			sendWorkloadHTTPRequests(t, requests)
		}
		return foundTraces
	}, 30*time.Second, 1*time.Second, "Expected traces not found")
}

func testIstioTraces(t *testing.T, expectedTracesFile string, tracesSink *consumertest.TracesSink) {
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)
	require.NotEmpty(t, expectedTraces)

	for _, receivedTraces := range tracesSink.AllTraces() {
		err = ptracetest.CompareTraces(expectedTraces, receivedTraces,
			ptracetest.IgnoreResourceSpansOrder(),
			ptracetest.IgnoreEndTimestamp(),
			ptracetest.IgnoreTraceID(),
			ptracetest.IgnoreSpanID(),
			ptracetest.IgnoreSpansOrder(),
			ptracetest.IgnoreStartTimestamp(),
			ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
			ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
			ptracetest.IgnoreResourceAttributeValue("telemetry.sdk.version"),
			ptracetest.IgnoreSpanAttributeValue("guid:x-request-id"),
		)
		if err == nil {
			break
		}
	}

	require.NoError(t, err)

	// generateIstioTraces(t)

	//require.Eventually(t, func() bool {
	//	foundTraces := false
	//	for _, receivedTraces := range tracesSink.AllTraces() {
	//		//if receivedTraces.ResourceSpans().Len() > 0 && receivedTraces.ResourceSpans().At(0).ScopeSpans().Len() > 0 {
	//		//	//val, found := receivedTraces.ResourceSpans().At(0).Resource().Attributes().Get("k8s.node.name")
	//		//	val, found := receivedTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Get("component")
	//		//	if !found || val.Str() != "proxy" {
	//		//		t.Setenv("UPDATE_EXPECTED_RESULTS", "true")
	//		//		internal.MaybeWriteUpdateExpectedTracesResults(t, expectedTracesFile, &receivedTraces)
	//		//		return true
	//		//	}
	//		//	return false
	//		//}
	//
	//		err = ptracetest.CompareTraces(expectedTraces, receivedTraces,
	//			ptracetest.IgnoreResourceSpansOrder(),
	//			ptracetest.IgnoreEndTimestamp(),
	//			ptracetest.IgnoreTraceID(),
	//			ptracetest.IgnoreSpanID(),
	//			ptracetest.IgnoreSpansOrder(),
	//			ptracetest.IgnoreStartTimestamp(),
	//			ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
	//			ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
	//			ptracetest.IgnoreResourceAttributeValue("telemetry.sdk.version"),
	//			ptracetest.IgnoreSpanAttributeValue("guid:x-request-id"),
	//		)
	//		if err == nil {
	//			foundTraces = true
	//			break
	//		}
	//	}
	//	return foundTraces
	//}, 5*time.Minute, 1*time.Second, "Expected traces not found")
}
