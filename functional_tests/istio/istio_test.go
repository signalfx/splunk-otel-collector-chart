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
	"go.opentelemetry.io/collector/pdata/ptrace"
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
		teardown(t)
	})
}

func teardown(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	istioctlPath := downloadIstio(t)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	config, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

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
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: httpbin
  namespace: istio-system
`)
	// This is a bit of a hacky workaround to address test flakiness. internal.DeleteObject runs
	// asynchronously, so a following test may be started before the namespace is actually deleted.
	// The solution is to wait for the namespace to be deleted when possible. We can't wait for
	// the namespace to be deleted if the test context is cancelled, which is the case when
	// called from t.Cleanup. This should be fine since other tests will most likely not rely on
	// the istio-workloads namespace being deleted.
	if t.Context().Err() != nil {
		internal.DeleteObject(t, k8sClient, `
apiVersion: v1
kind: Namespace
metadata:
  name: istio-workloads
`)
	} else {
		internal.DeleteNamespace(t, clientset, "istio-workloads")
	}
	runCommand(t, fmt.Sprintf("%s uninstall --purge -y", istioctlPath))

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
		teardown(t)
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
		testIstioMetrics(t, "testdata/expected_istiod.yaml", "pilot_xds_pushes",
			flakyMetrics, true, metricsSink)
	})

	flakyMetrics = []string{"istio_agent_pilot_xds_expired_nonce"}
	t.Run("istio ingress metrics captured", func(t *testing.T) {
		testIstioMetrics(t, "testdata/expected_istioingress.yaml",
			"istio_requests_total", flakyMetrics, true, metricsSink)
	})
}

func testIstioMetrics(t *testing.T, expectedMetricsFile string, includeMetricName string, flakyMetricNames []string, ignoreLen bool, metricsSink *consumertest.MetricsSink) {
	expectedMetrics, err := golden.ReadMetrics(expectedMetricsFile)
	require.NoError(t, err)

	internal.WaitForMetrics(t, 2, metricsSink)

	var metricNames []string
	for i := 0; i < expectedMetrics.ResourceMetrics().Len(); i++ {
		for j := 0; j < expectedMetrics.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			for k := 0; k < expectedMetrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
				metric := expectedMetrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
				metricNames = append(metricNames, metric.Name())
			}
		}
	}

	require.Eventually(t, func() bool {
		for _, receivedMetrics := range metricsSink.AllMetrics() {
			if flakyMetricNames != nil {
				internal.RemoveFlakyMetrics(&receivedMetrics, flakyMetricNames)
			}

			err = pmetrictest.CompareMetrics(expectedMetrics, receivedMetrics,
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
			if err == nil {
				return true
			}
			t.Logf("Comparison error: %v", err)
		}

		selectedMetrics := internal.SelectMetricSet(t, expectedMetrics, includeMetricName, metricsSink, ignoreLen)
		if selectedMetrics != nil {
			internal.MaybeUpdateExpectedMetricsResults(t, expectedMetricsFile, selectedMetrics)
		}
		return false
	}, 5*time.Minute, 1*time.Second, "Expected metrics not found")
}

func Test_IstioTraces(t *testing.T) {
	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		t.Log("Running teardown before setup as TEARDOWN_BEFORE_SETUP is set to true")
		teardown(t)
	}

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
	}
	sendWorkloadHTTPRequests(t, requests)

	require.Eventually(t, func() bool {
		foundTraces := false
		for _, receivedTraces := range tracesSink.AllTraces() {
			internal.MaybeWriteUpdateExpectedTracesResults(t, expectedTracesFile, &receivedTraces)

			// Multiple resource spans are created intermittently in testing.
			// In an attempt to reduce flakiness the expected data just has a single
			// resource span, so the comparison here is just to ensure that at least
			// one of the received resource spans matches what's expected.
			for i := 0; i < receivedTraces.ResourceSpans().Len(); i++ {
				receivedResourceSpans := receivedTraces.ResourceSpans().At(i)
				tempTraces := ptrace.NewTraces()
				tempResourceSpans := tempTraces.ResourceSpans().AppendEmpty()
				receivedResourceSpans.CopyTo(tempResourceSpans)

				err = ptracetest.CompareTraces(expectedTraces, tempTraces,
					ptracetest.IgnoreResourceSpansOrder(),
					ptracetest.IgnoreSpansOrder(),
					ptracetest.IgnoreStartTimestamp(),
					ptracetest.IgnoreEndTimestamp(),
					ptracetest.IgnoreTraceID(),
					ptracetest.IgnoreSpanID(),
					ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
					ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
					ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
					ptracetest.IgnoreSpanAttributeValue("guid:x-request-id"),
					ptracetest.IgnoreSpanAttributeValue("node_id"),
					ptracetest.IgnoreSpanAttributeValue("peer.address"),
				)
				if err == nil {
					foundTraces = true
					break
				}
				t.Logf("Comparison error: %v", err)
			}
		}

		if !foundTraces {
			sendWorkloadHTTPRequests(t, requests)
		}
		return foundTraces
	}, 1*time.Minute, 1*time.Second, "Expected traces not found")
}
