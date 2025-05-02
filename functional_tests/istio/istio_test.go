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
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const istioVersion = "1.24.2"

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

	// Install Istio
	istioctlPath := downloadIstio(t)
	runCommand(t, fmt.Sprintf("%s install -y", istioctlPath))

	// Patch ingress gateway to work in kind cluster
	patchResource(t, clientset, "istio-system", "istio-ingressgateway", "deployments", `{"spec":{"template":{"spec":{"containers":[{"name":"istio-proxy","ports":[{"containerPort":8080,"hostPort":80},{"containerPort":8443,"hostPort":443}]}]}}}}`)
	patchResource(t, clientset, "istio-system", "istio-ingressgateway", "services", `{"spec": {"type": "ClusterIP"}}`)

	internal.CreateNamespace(t, clientset, "istio-workloads")
	internal.LabelNamespace(t, clientset, "istio-workloads", "istio-injection", "enabled")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	_, err = k8stest.CreateObjects(k8sClient, "testdata/testobjects")
	require.NoError(t, err)
	deployment, err := clientset.AppsV1().Deployments("istio-workloads").Get(t.Context(), "httpbin", metav1.GetOptions{})
	require.NoError(t, err, "failed to get httpbin deployment")
	t.Logf("Deployment %s created successfully", deployment.Name)

	internal.CheckPodsReady(t, clientset, "istio-system", "app=istio-ingressgateway", 5*time.Minute)
	internal.CheckPodsReady(t, clientset, "istio-system", "app=istiod", 2*time.Minute)
	internal.CheckPodsReady(t, clientset, "istio-workloads", "app=httpbin", 3*time.Minute)

	// Send traffic through ingress gateways
	sendWorkloadHTTPRequests(t)

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
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		teardown(t, k8sClient, istioctlPath)
	})
}

func teardown(t *testing.T, k8sClient *k8stest.K8sClient, istioctlPath string) {
	deleteObject(t, k8sClient, `
apiVersion: networking.istio.io/v1
kind: Gateway
metadata:
  name: httpbin-gateway
  namespace: istio-system
`)
	deleteObject(t, k8sClient, `
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: httpbin
  namespace: istio-system
`)
	deleteObject(t, k8sClient, `
apiVersion: v1
kind: Namespace
metadata:
  name: istio-workloads
`)
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

func sendWorkloadHTTPRequests(t *testing.T) {
	resolveHost := "127.0.0.1"
	// resolveHost := hostEndpoint(t)
	resolveHeader := "httpbin.example.com:80"

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

	requests := []struct {
		url    string
		host   string
		header string
		path   string
	}{
		{"http://httpbin.example.com/status/200", "httpbin.example.com", resolveHeader, "/status/200"},
		{"http://httpbin.example.com/status/404", "httpbin.example.com", resolveHeader, "/status/404"},
		{"http://httpbin.example.com/delay/1", "httpbin.example.com", resolveHeader, "/delay/0"},
	}

	for _, req := range requests {
		sendHTTPRequest(t, client, req.url, req.host, req.header, req.path)
	}
}

func deleteObject(t *testing.T, k8sClient *k8stest.K8sClient, objYAML string) {
	obj := &unstructured.Unstructured{}
	require.NoError(t, yaml.Unmarshal([]byte(objYAML), obj))
	require.NoError(t, k8stest.DeleteObject(k8sClient, obj))
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

	selectedMetrics := selectMetricSetWithTimeout(t, expectedMetrics, includeMetricName, metricsSink, ignoreLen, 5*time.Minute, 30*time.Second)
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

	err = pmetrictest.CompareMetrics(expectedMetrics, *selectedMetrics,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreMetricValues(metricNames...),
		pmetrictest.IgnoreMetricAttributeValue("host.name"),
		pmetrictest.IgnoreMetricAttributeValue("http.scheme"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreMetricAttributeValue("k8s.pod.uid"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.name"),
		pmetrictest.IgnoreMetricAttributeValue("net.host.port"),
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
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		internal.WriteNewExpectedMetricsResult(t, expectedMetricsFile, selectedMetrics)
	}
	require.NoError(t, err)
}

func selectMetricSetWithTimeout(t *testing.T, expected pmetric.Metrics, metricName string, metricSink *consumertest.MetricsSink, ignoreLen bool, timeout time.Duration, interval time.Duration) *pmetric.Metrics {
	var selectedMetrics *pmetric.Metrics
	require.Eventuallyf(t, func() bool {
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
			if ignoreLen || (m.ResourceMetrics().Len() == expected.ResourceMetrics().Len() && m.MetricCount() == expected.MetricCount()) {
				selectedMetrics = &m
				return true
			}
		}
		return false
	}, timeout, interval, "Failed to find the expected metric %s within the timeout period", metricName)
	return selectedMetrics
}
