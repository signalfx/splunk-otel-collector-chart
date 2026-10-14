// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package db2

import (
	"os"
	"path/filepath"
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

	// The community image initialises the instance and creates SAMPLE on first boot, which
	// takes minutes. The readiness probe runs a real query, so this waits on usability
	// rather than on the port opening.
	db2ReadyTimeout = 12 * time.Minute
)

// Test_Db2_Metrics_Through_Collector covers the part of the splunk-otel-db2 subchart that
// unit and render tests cannot: that a receiver deployed by the subchart reaches the
// Collector this release creates, and that its metrics arrive.
//
// db2.otlp.endpoint is left unset in the values on purpose. The subchart deriving that
// address from the parent release name is the contract under test; hard-coding it would
// assert nothing about the wiring.
func Test_Db2_Metrics_Through_Collector(t *testing.T) {
	kubeconfig, ok := os.LookupEnv("KUBECONFIG")
	require.True(t, ok, "KUBECONFIG must be set")

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t, kubeconfig)
	}

	client := k8sClient(t, kubeconfig)

	// Db2 first: the receiver retries, but starting it against nothing only lengthens the
	// run and muddies a failure into "did Db2 come up or did the wiring break".
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
		teardown(t, kubeconfig)
	})

	// The receiver scrapes on an interval and the Collector batches, so allow several
	// cycles rather than expecting the first scrape to arrive.
	internal.AssertMetricNames(t, metricsSink,
		filepath.Join(testDir, "expected_db2_metric_names.yaml"), 5*time.Minute, 5*time.Second)
}

func k8sClient(t *testing.T, kubeconfig string) *kubernetes.Clientset {
	t.Helper()
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)
	return client
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

	// Waits on the readiness probe, which runs a real query - so this returns when SAMPLE is
	// actually connectable, not merely when the port is listening.
	internal.CheckPodsReady(t, client, internal.DefaultNamespace, "app=db2", db2ReadyTimeout, 15*time.Second)
}

// decode reads a single-document manifest and asserts it is the expected kind, so a mistake
// in testdata fails here rather than as a confusing nil later.
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

func teardown(t *testing.T, kubeconfig string) {
	t.Helper()
	internal.ChartUninstall(t, kubeconfig)
}
