// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package db2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	testDir   = "testdata"
	valuesDir = "values"

	// Rendered by the subchart from the release name; see assertDerivedEndpoint.
	db2ConfigMapSuffix = "-db2-config"

	// Matches the startupProbe budget in testdata/db2/deployment.yaml, plus slack for the
	// image pull. Db2 is ready in about 95s once probed with a real connection rather
	// than a port check; this is a ceiling for a cold pull, not an expectation.
	db2ReadyTimeout = 25 * time.Minute
)

// Test_Db2_Metrics_Through_Collector exercises the whole path: a real Db2, a receiver
// deployed by the subchart, the Collector this release creates, and metrics arriving at a
// sink. Render and unit tests cover the subchart's output; only this covers the contract
// between the two charts.
//
// It asserts two things that can fail independently:
//
//   - the address the subchart derived names a Service this release actually created, which
//     a string comparison would not catch; and
//   - Db2 metrics reach the sink, which is the data flow itself.
//
// db2.otlp.endpoint is left unset on purpose. The subchart deriving that address from the
// parent release name is the contract under test, so hard-coding it would assert nothing.
func Test_Db2_Metrics_Through_Collector(t *testing.T) {
	kubeconfig, ok := os.LookupEnv("KUBECONFIG")
	require.True(t, ok, "KUBECONFIG must be set")

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		internal.ChartUninstall(t, kubeconfig)
	}

	client := k8sClient(t, kubeconfig)

	// Db2 first. The receiver retries, but starting it against nothing only lengthens the
	// run and turns a failure into "did Db2 come up, or did the wiring break".
	deployDb2(t, client)

	metricsSink := internal.SetupSignalfxReceiver(t, internal.SignalFxReceiverPort)

	valuesFile, err := filepath.Abs(filepath.Join(testDir, valuesDir, "db2_values.yaml.tmpl"))
	require.NoError(t, err)

	hostEp := internal.HostEndpoint(t)
	internal.ChartInstallOrUpgrade(t, kubeconfig, valuesFile, map[string]any{
		"IngestURL": "http://" + internal.HostPort(hostEp, internal.SignalFxReceiverPort),
		"ApiURL":    "http://" + internal.HostPort(hostEp, internal.SignalFxAPIPort),
	}, 0, internal.GetDefaultChartOptions())

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		internal.ChartUninstall(t, kubeconfig)
	})

	// The receiver has to come up without a reachable Db2: it retries the connection and
	// logs collection failures rather than exiting, so a crash here would mean the chart
	// produced an unusable Deployment or an unparsable config.
	internal.CheckPodsReady(t, client, internal.DefaultNamespace,
		"app.kubernetes.io/name=db2", 5*time.Minute, 10*time.Second)

	assertDerivedEndpoint(t, client)

	// The receiver scrapes on an interval and the Collector batches, so allow several
	// cycles rather than expecting the first scrape to land.
	internal.AssertMetricNames(t, metricsSink,
		filepath.Join(testDir, "expected_db2_metric_names.yaml"), 5*time.Minute, 5*time.Second)
}

func deployDb2(t *testing.T, client *kubernetes.Clientset) {
	t.Helper()

	svc := decode[*corev1.Service](t, filepath.Join(testDir, "db2", "service.yaml"))
	services := client.CoreV1().Services(internal.DefaultNamespace)
	_, err := services.Create(t.Context(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	dep := decode[*appsv1.Deployment](t, filepath.Join(testDir, "db2", "deployment.yaml"))
	deployments := client.AppsV1().Deployments(internal.DefaultNamespace)
	_, err = deployments.Create(t.Context(), dep, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			return
		}
		grace := int64(0)
		_ = deployments.Delete(t.Context(), dep.Name, metav1.DeleteOptions{GracePeriodSeconds: &grace})
		_ = services.Delete(t.Context(), svc.Name, metav1.DeleteOptions{})
	})

	// Waits on the startupProbe, which opens a real connection - so this returns when
	// SAMPLE is usable, not merely when the port is listening.
	internal.CheckPodsReady(t, client, internal.DefaultNamespace, "app=db2",
		db2ReadyTimeout, 15*time.Second)
}

// decode reads a single-document manifest and asserts its kind, so a mistake in testdata
// fails here rather than as a confusing nil later.
func decode[T any](t *testing.T, path string) T {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(raw, nil, nil)
	require.NoError(t, err)
	typed, ok := obj.(T)
	require.True(t, ok, "%s decoded to %T, not the expected type", path, obj)
	return typed
}

// assertDerivedEndpoint is the actual contract: the address the subchart derived must name a
// Service this release created, on a port it exposes. Asserting the string alone would pass
// just as happily against a Service that does not exist, which is the bug this guards.
func assertDerivedEndpoint(t *testing.T, client *kubernetes.Clientset) {
	t.Helper()

	cms, err := client.CoreV1().ConfigMaps(internal.DefaultNamespace).
		List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)

	var config string
	for _, cm := range cms.Items {
		if strings.HasSuffix(cm.Name, db2ConfigMapSuffix) {
			config = cm.Data["config.yaml"]
			break
		}
	}
	require.NotEmpty(t, config, "no ConfigMap ending in %q was created", db2ConfigMapSuffix)

	host, port := otlpTarget(t, config)

	svc, err := client.CoreV1().Services(internal.DefaultNamespace).
		Get(t.Context(), host, metav1.GetOptions{})
	require.NoError(t, err,
		"the subchart derived %q, but this release created no such Service - the receiver "+
			"would export into a name that does not resolve", host)

	var ports []int32
	for _, p := range svc.Spec.Ports {
		ports = append(ports, p.Port)
	}
	require.Contains(t, ports, port,
		"Service %q exists but does not expose port %d", host, port)
}

// otlpTarget pulls the exporter endpoint out of the rendered receiver config and reduces it
// to the Service name and port, so the assertion is about the object rather than the string.
func otlpTarget(t *testing.T, config string) (string, int32) {
	t.Helper()

	const key = "endpoint:"
	idx := strings.Index(config, key)
	require.NotEqual(t, -1, idx, "no exporter endpoint in the rendered config:\n%s", config)

	line := strings.TrimSpace(config[idx+len(key):])
	if nl := strings.IndexByte(line, '\n'); nl != -1 {
		line = line[:nl]
	}
	endpoint := strings.Trim(strings.TrimSpace(line), `"`)
	endpoint = strings.TrimPrefix(endpoint, "http://")

	hostPort := strings.SplitN(endpoint, ":", 2)
	require.Len(t, hostPort, 2, "endpoint %q has no port", endpoint)

	var port int32
	_, err := fmt.Sscanf(hostPort[1], "%d", &port)
	require.NoError(t, err, "endpoint %q has a non-numeric port", endpoint)

	// Derived as a cluster FQDN; the Service object is named by its first label.
	return strings.SplitN(hostPort[0], ".", 2)[0], port
}

func k8sClient(t *testing.T, kubeconfig string) *kubernetes.Clientset {
	t.Helper()
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)
	return client
}
