// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package histogram

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	signalFxReceiverPort = 4317
	valuesDir            = "values"
	expectedDir          = "expected"
)

func deployChart(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}
	replacements := map[string]any{
		"IngestURL": internal.HostPortHTTP(hostEp, signalFxReceiverPort),
	}
	valuesFile, err := filepath.Abs(filepath.Join("testdata", valuesDir, "test_values.yaml.tmpl"))
	require.NoError(t, err)
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 0, internal.GetDefaultChartOptions())
}

func teardown(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	internal.ChartUninstall(t, testKubeConfig)
}

type TestInput struct {
	ServiceName            string // service name
	NonHistogramMetricName string // metric name which is expected to be present for the component
	HistogramMetricName    string // metric name which is expected to be present for the component only
}

var testInputs = []TestInput{
	{
		ServiceName:            "kubernetes-scheduler",
		NonHistogramMetricName: "scheduler_queue_incoming_pods_total",
		HistogramMetricName:    "scheduler_scheduling_algorithm_duration_seconds",
	},
	{
		ServiceName:            "kubernetes-proxy",
		NonHistogramMetricName: "kubeproxy_sync_proxy_rules_service_changes_total",
		HistogramMetricName:    "kubeproxy_sync_proxy_rules_duration_seconds",
	},
	{
		ServiceName:            "kubernetes-apiserver",
		NonHistogramMetricName: "apiserver_request_total",
		HistogramMetricName:    "apiserver_request_duration_seconds",
	},
	{
		ServiceName:            "kube-controller-manager",
		NonHistogramMetricName: "workqueue_retries_total",
		HistogramMetricName:    "workqueue_queue_duration_seconds",
	},
	{
		ServiceName:            "coredns",
		NonHistogramMetricName: "coredns_dns_requests_total",
		HistogramMetricName:    "coredns_dns_request_duration_seconds",
	},
	{
		ServiceName:            "etcd",
		NonHistogramMetricName: "etcd_server_is_leader",
		HistogramMetricName:    "etcd_disk_wal_fsync_duration_seconds",
	},
}

func (input TestInput) assertionConfig(isHistogram bool) (string, string) {
	if isHistogram {
		return input.ServiceName + "_histogram_metrics_assertion.yaml",
			input.HistogramMetricName
	}
	return input.ServiceName + "_metrics_assertion.yaml",
		input.NonHistogramMetricName
}

func Test_ControlPlaneMetrics(t *testing.T) {
	if k8sVersion := os.Getenv("K8S_VERSION"); k8sVersion != "" {
		t.Logf("Running control plane metrics assertions against Kubernetes %s", k8sVersion)
	}

	metricsSink := internal.SetupSignalfxReceiver(t, signalFxReceiverPort)

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t)
	}

	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		// Prime CoreDNS cache metrics before the collector's first scrape.
		testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
		require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
		clientset, err := internal.GetKubeClient(testKubeConfig)
		require.NoError(t, err)
		performDNSQueries(t, clientset)
		deployChart(t)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	for _, isHistogram := range []bool{true, false} {
		for _, input := range testInputs {
			t.Run(fmt.Sprintf("%s_histograms=%t", input.ServiceName, isHistogram), func(t *testing.T) {
				runMetricsTest(t, isHistogram, metricsSink, input)
			})
		}
	}
}

func runMetricsTest(t *testing.T, isHistogram bool, metricsSink *consumertest.MetricsSink, input TestInput) {
	internal.WaitForMetrics(t, 5, metricsSink)

	fileName, metricName := input.assertionConfig(isHistogram)
	expectedFilePath := filepath.Join("testdata", expectedDir, fileName)

	t.Logf("checking for metrics matching component %s using target metric %s",
		input.ServiceName, metricName)
	opts := []internal.MetricsAssertionOption{
		internal.WithVolatileAttributes(commonVolatileAttributes...),
		internal.WithRegexAttributes(regexAttributes(input.ServiceName)),
		internal.WithDatapointAttributesAsExistsExcept(metricIdentityAttributes...),
		internal.WithMaxDatapointsPerMetric(1),
	}
	if isHistogram {
		opts = append(opts, internal.WithHistogramExplicitBounds())
	}
	if input.ServiceName == "kubernetes-apiserver" && !isHistogram {
		// TODO: Replace this selector with a datapoints/include assertion in the YAML once
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/48545 lands.
		opts = append(opts, internal.WithSelectedNumberDatapoint(input.NonHistogramMetricName, map[string]string{
			"code": "200", "component": "apiserver", "resource": "services",
			"scope": "resource", "verb": "GET", "version": "v1",
		}))
	}

	internal.AssertMetricsSnapshot(t, metricsSink, metricName, expectedFilePath,
		3*time.Minute, 5*time.Second, opts...)
}

func performDNSQueries(t *testing.T, clientset *kubernetes.Clientset) {
	kubernetesService, err := clientset.CoreV1().Services(corev1.NamespaceDefault).Get(
		t.Context(), "kubernetes", metav1.GetOptions{},
	)
	require.NoError(t, err)
	require.NotEmpty(t, kubernetesService.Spec.ClusterIP, "Kubernetes service did not have a cluster IP")

	const coreDNSLabelSelector = "k8s-app=kube-dns"
	internal.CheckPodsReady(
		t, clientset, metav1.NamespaceSystem, coreDNSLabelSelector, 3*time.Minute, 0,
	)
	coreDNSPods, err := internal.GetPods(t, clientset, metav1.NamespaceSystem, coreDNSLabelSelector)
	require.NoError(t, err)
	coreDNSPodIPs := make([]string, 0, len(coreDNSPods.Items))
	for _, pod := range coreDNSPods.Items {
		if pod.Status.PodIP != "" {
			coreDNSPodIPs = append(coreDNSPodIPs, pod.Status.PodIP)
		}
	}
	require.NotEmpty(t, coreDNSPodIPs, "did not find any CoreDNS pod IPs")

	overrides := `{"spec": {"dnsPolicy": "ClusterFirst"}}`
	// CoreDNS disables caching for cluster.local; reverse service lookups use the cacheable in-addr.arpa zone.
	queries := `target=$1
	shift
	for server in "$@"; do
		for i in $(seq 1 100); do
			nslookup "$target" "$server" >/dev/null || exit 1
		done
	done`
	args := []string{
		"run", "--rm", "-i", "--tty", "dns-query",
		"--image=busybox", "--restart=Never", "--overrides=" + overrides, "--",
		"sh", "-c", queries, "dns-query", kubernetesService.Spec.ClusterIP,
	}
	args = append(args, coreDNSPodIPs...)
	cmd := exec.Command("kubectl", args...)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err = cmd.Run()
	require.NoErrorf(t, err, "DNS queries failed\nStandard Output: %s\nStandard Error: %s",
		out.String(), stderr.String())
}

var commonVolatileAttributes = []string{
	"server.address",
	"server.port",
	"service.instance.id",
}

var metricRegexAttributes = map[string]string{
	"k8s.pod.uid": "^" + internal.K8sUIDRegex + "$",
	"verb":        internal.K8sAPIVerbRegex,
}

var componentMetricRegexAttributes = map[string]map[string]string{
	"coredns": {
		"k8s.pod.name": "^coredns-" + internal.K8sNameRegex + "$",
	},
	"kubernetes-proxy": {
		"k8s.pod.name": "^kube-proxy-" + internal.K8sNameRegex + "$",
	},
}

var metricIdentityAttributes = []string{
	"host.name",
	"k8s.cluster.name",
	"k8s.namespace.name",
	"k8s.node.name",
	"k8s.pod.name",
	"k8s.pod.uid",
	"os.type",
	"server.address",
	"server.port",
	"service.instance.id",
	"service.name",
	"url.scheme",
}

func regexAttributes(component string) map[string]string {
	return internal.ExtendMetricAssertionRegexAttrs(
		metricRegexAttributes,
		componentMetricRegexAttributes[component],
	)
}
