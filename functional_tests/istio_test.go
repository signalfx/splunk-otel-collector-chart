//go:build istio

package functional_tests

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	apiPort              = 8881
	signalFxReceiverPort = 9943
	istioVersion         = "1.24.2"
)

// Env vars to control the test behavior
// TEARDOWN_BEFORE_SETUP: if set to true, the test will run teardown before setup
// SKIP_SETUP: if set to true, the test will skip setup
// SKIP_TEARDOWN: if set to true, the test will skip teardown
// SKIP_TESTS: if set to true, the test will skip the test
// UPDATE_EXPECTED_RESULTS: if set to true, the test will update the expected results
// KUBECONFIG: the path to the kubeconfig file

var setupRun = sync.Once{}
var istioMetricsConsumer *consumertest.MetricsSink

func setupOnce(t *testing.T) *consumertest.MetricsSink {
	setupRun.Do(func() {

		if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
			t.Log("Running teardown before setup as TEARDOWN_BEFORE_SETUP is set to true")
			testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
			require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
			k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
			require.NoError(t, err)
			istioctlPath := downloadIstio(t, istioVersion)
			teardown(t, k8sClient, istioctlPath)
		}

		// create an API server
		internal.CreateApiServer(t, apiPort)

		istioMetricsConsumer = setupSignalfxReceiver(t, signalFxReceiverPort)

		if os.Getenv("SKIP_SETUP") == "true" {
			t.Log("Skipping setup as SKIP_SETUP is set to true")
			return
		}
		deployIstioAndCollector(t)
	})

	return istioMetricsConsumer
}

func deployIstioAndCollector(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	config, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	// Install Istio
	istioctlPath := downloadIstio(t, istioVersion)
	runCommand(t, fmt.Sprintf("%s install -y", istioctlPath))

	// Patch ingress gateway to work in kind cluster
	patchResource(t, clientset, "istio-system", "istio-ingressgateway", "deployments", `{"spec":{"template":{"spec":{"containers":[{"name":"istio-proxy","ports":[{"containerPort":8080,"hostPort":80},{"containerPort":8443,"hostPort":443}]}]}}}}`)
	patchResource(t, clientset, "istio-system", "istio-ingressgateway", "services", `{"spec": {"type": "ClusterIP"}}`)

	createNamespace(t, clientset, "istio-workloads")
	labelNamespace(t, clientset, "istio-workloads", "istio-injection", "enabled")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	_, err = k8stest.CreateObjects(k8sClient, "testdata_istio/testobjects")
	require.NoError(t, err)
	deployment, err := clientset.AppsV1().Deployments("istio-workloads").Get(context.TODO(), "httpbin", metav1.GetOptions{})
	require.NoError(t, err, "failed to get httpbin deployment")
	t.Logf("Deployment %s created successfully", deployment.Name)

	checkPodsReady(t, clientset, "istio-system", "app=istio-ingressgateway", 5*time.Minute)
	checkPodsReady(t, clientset, "istio-system", "app=istiod", 2*time.Minute)
	checkPodsReady(t, clientset, "istio-workloads", "app=httpbin", 3*time.Minute)

	// Send traffic through ingress gateways
	sendWorkloadHTTPRequests(t)

	chartPath := filepath.Join("..", "helm-charts", "splunk-otel-collector")
	chart, err := loader.Load(chartPath)
	require.NoError(t, err)

	valuesBytes, err := os.ReadFile(filepath.Join("testdata_istio", "istio_values.yaml.tmpl"))
	require.NoError(t, err)

	hostEp := hostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}

	replacements := struct {
		IngestURL string
		ApiURL    string
	}{
		fmt.Sprintf("http://%s:%d", hostEp, signalFxReceiverPort),
		fmt.Sprintf("http://%s:%d", hostEp, apiPort),
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

	actionConfig := new(action.Configuration)
	testKubeConfig, _ := os.LookupEnv("KUBECONFIG")
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format+"\n", v...)
	}); err != nil {
		require.NoError(t, err)
	}
	uninstall := action.NewUninstall(actionConfig)
	uninstall.IgnoreNotFound = true
	uninstall.Wait = true
	_, _ = uninstall.Run("sock")
}

func downloadIstio(t *testing.T, version string) string {
	var url string
	if runtime.GOOS == "darwin" {
		url = fmt.Sprintf("https://github.com/istio/istio/releases/download/%s/istio-%s-osx.tar.gz", version, version)
	} else if runtime.GOOS == "linux" {
		url = fmt.Sprintf("https://github.com/istio/istio/releases/download/%s/istio-%s-linux-amd64.tar.gz", version, version)
	} else {
		t.Fatalf("unsupported operating system: %s", runtime.GOOS)
	}

	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	gz, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	defer gz.Close()

	tr := tar.NewReader(gz)
	var istioDir string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		target := filepath.Join(".", hdr.Name)
		if hdr.FileInfo().IsDir() && istioDir == "" {
			istioDir = target
		}
		if hdr.FileInfo().IsDir() {
			require.NoError(t, os.MkdirAll(target, hdr.FileInfo().Mode()))
		} else {
			f, err := os.Create(target)
			require.NoError(t, err)
			defer f.Close()

			_, err = io.Copy(f, tr)
			require.NoError(t, err)
		}
	}
	require.NotEmpty(t, istioDir, "istioctl path not found")

	absIstioDir, err := filepath.Abs(istioDir)
	require.NoError(t, err, "failed to get absolute path for istioDir")

	istioctlPath := filepath.Join(absIstioDir, "bin", "istioctl")
	require.FileExists(t, istioctlPath, "istioctl binary not found")
	require.NoError(t, os.Chmod(istioctlPath, 0755), "failed to set executable permission for istioctl")

	t.Cleanup(func() {
		os.RemoveAll(absIstioDir)
	})

	return istioctlPath
}

func checkPodsReady(t *testing.T, clientset *kubernetes.Clientset, namespace, labelSelector string, timeout time.Duration) {
	require.Eventually(t, func() bool {
		pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		require.NoError(t, err)
		if len(pods.Items) == 0 {
			return false
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase != v1.PodRunning {
				return false
			}
			ready := false
			for _, condition := range pod.Status.Conditions {
				if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
					ready = true
					break
				}
			}
			if !ready {
				return false
			}
		}
		return true
	}, timeout, 5*time.Second, "Pods in namespace %s with label %s are not ready", namespace, labelSelector)
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
		_, err = clientset.AppsV1().Deployments(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
	case "services":
		_, err = clientset.CoreV1().Services(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
	}
	require.NoError(t, err)
}

func createObjectFromURL(t *testing.T, config string, url string) {
	resp, err := http.Get(url)
	require.NoError(t, err, "failed to fetch URL: %s", url)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body from URL: %s", url)
	t.Logf("Fetched YAML content from URL %s:\n%s", url, string(body))
	k8sClient, err := k8stest.NewK8sClient(config)
	require.NoError(t, err, "failed to create Kubernetes client")

	// Use a YAML decoder to parse the content
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(body), 4096)
	for {
		var doc map[string]interface{}
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "failed to decode YAML document")
		if len(doc) == 0 {
			continue
		}
		docBytes, err := sigsyaml.Marshal(doc)
		require.NoError(t, err, "failed to marshal YAML document")
		t.Logf("Creating object from document:\n%s", string(docBytes))
		_, err = k8stest.CreateObject(k8sClient, docBytes)
		require.NoError(t, err, "failed to create object from document")
	}
}

func createNamespace(t *testing.T, clientset *kubernetes.Clientset, name string) {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	require.NoError(t, err, "failed to create namespace %s", name)

	require.Eventually(t, func() bool {
		_, err := clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
		return err == nil
	}, 1*time.Minute, 5*time.Second, "namespace %s is not available", name)
}

func labelNamespace(t *testing.T, clientset *kubernetes.Clientset, name, key, value string) {
	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	require.NoError(t, err)
	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}
	ns.Labels[key] = value
	_, err = clientset.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
	require.NoError(t, err)
}

func sendHTTPRequest(t *testing.T, client *http.Client, url, host, header, path string) {
	req, err := http.NewRequest("GET", url, nil)
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
	err := yaml.Unmarshal([]byte(objYAML), obj)
	require.NoError(t, err)
	k8stest.DeleteObject(k8sClient, obj)
}

func Test_IstioMetrics(t *testing.T) {
	_ = setupOnce(t)
	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	flakyMetrics := []string{"galley_validation_config_update_error"} // only shows up when config validation fails - removed if present when comparing
	t.Run("istiod metrics captured", func(t *testing.T) {
		testIstioMetrics(t, "testdata_istio/expected_istiod.yaml", "pilot_xds_pushes", flakyMetrics, true)
	})

	flakyMetrics = []string{"istio_agent_pilot_xds_expired_nonce"}
	t.Run("istio ingress metrics captured", func(t *testing.T) {
		testIstioMetrics(t, "testdata_istio/expected_istioingress.yaml", "istio_requests_total", flakyMetrics, true)
	})
}

func testIstioMetrics(t *testing.T, expectedMetricsFile string, includeMetricName string, flakyMetricNames []string, ignoreLen bool) {
	expectedMetrics, err := golden.ReadMetrics(expectedMetricsFile)
	require.NoError(t, err)

	waitForMetrics(t, 2, istioMetricsConsumer)

	selectedMetrics := selectMetricSetWithTimeout(t, expectedMetrics, includeMetricName, istioMetricsConsumer, ignoreLen, 5*time.Minute, 30*time.Second)
	if selectedMetrics == nil {
		t.Error("No metric batch identified with the right metric count, exiting")
		return
	}
	require.NotNil(t, selectedMetrics)

	if flakyMetricNames != nil {
		removeFlakyMetrics(selectedMetrics, flakyMetricNames)
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
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreMetricAttributeValue("event"),
		pmetrictest.IgnoreSubsequentDataPoints(metricNames...),
	)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		writeNewExpectedMetricsResult(t, expectedMetricsFile, selectedMetrics)
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

func removeFlakyMetrics(metrics *pmetric.Metrics, flakyMetrics []string) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		resourceMetrics := metrics.ResourceMetrics().At(i)
		for j := 0; j < resourceMetrics.ScopeMetrics().Len(); j++ {
			scopeMetrics := resourceMetrics.ScopeMetrics().At(j)
			metricSlice := scopeMetrics.Metrics()
			metricSlice.RemoveIf(func(metric pmetric.Metric) bool {
				for _, flakyMetric := range flakyMetrics {
					if metric.Name() == flakyMetric {
						return true
					}
				}
				return false
			})
		}
	}
}
