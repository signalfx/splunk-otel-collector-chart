// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package obi

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
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
)

type App struct {
	Name         string
	ManifestPath string
}

func (a App) LabelSelector() string {
	return fmt.Sprintf("app=%s", a.Name)
}

func Test_OBI_Minimal_Traces(t *testing.T) {
	kubeconfig, ok := os.LookupEnv("KUBECONFIG")
	require.True(t, ok, "KUBECONFIG must be set")

	apps := []App{
		{
			Name:         "python-test",
			ManifestPath: filepath.Join(testDir, "python", "deployment.yaml"),
		},
	}

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t, kubeconfig, apps)
	}

	// Start a local OTLP sink on ports 4317/4318. OBI defaults export to ${HOST_IP}:4317 (gRPC).
	tracesSink := internal.SetupOTLPTracesSink(t)

	// Install the collector chart with OBI enabled using a minimal values template.
	valuesFile, err := filepath.Abs(filepath.Join(testDir, valuesDir, "obi_values.yaml.tmpl"))
	require.NoError(t, err)

	hostEp := internal.HostEndpoint(t)
	internal.ChartInstallOrUpgrade(t, kubeconfig, valuesFile, map[string]any{
		"OTLPEndpoint": internal.HostPort(hostEp, internal.OTLPGRPCReceiverPort),
	}, 0, internal.GetDefaultChartOptions())

	// Ensure chart resources are removed after test unless explicitly skipped.
	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		internal.ChartUninstall(t, kubeconfig)
	})

	for _, app := range apps {
		deployApp(t, kubeconfig, app)
	}

	// Wait until at least one trace is received at the OTLP sink.
	internal.WaitForTraces(t, 1, tracesSink)
}

func deployApp(t *testing.T, kubeconfig string, app App) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	stream, err := os.ReadFile(app.ManifestPath)
	require.NoError(t, err)
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(stream, nil, nil)
	require.NoError(t, err)
	deployment := obj.(*appsv1.Deployment)

	deployments := client.AppsV1().Deployments(internal.DefaultNamespace)
	if _, err = deployments.Create(t.Context(), deployment, metav1.CreateOptions{}); err != nil {
		// Try update if it already exists
		_, err = deployments.Update(t.Context(), deployment, metav1.UpdateOptions{})
		require.NoError(t, err)
	}

	// Wait for the pod(s) to be Ready for a short stabilization period.
	internal.CheckPodsReady(t, client, internal.DefaultNamespace, app.LabelSelector(), 5*time.Minute, 15*time.Second)

	// Ensure the deployment gets cleaned up.
	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			return
		}
		grace := int64(0)
		_ = deployments.Delete(t.Context(), app.Name, metav1.DeleteOptions{GracePeriodSeconds: &grace})
	})
}

func teardown(t *testing.T, kubeconfig string, apps []App) {
	t.Helper()

	internal.ChartUninstall(t, kubeconfig)

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)

	client, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	deployments := client.AppsV1().Deployments(internal.DefaultNamespace)
	grace := int64(0)
	for _, app := range apps {
		_ = deployments.Delete(t.Context(), app.Name, metav1.DeleteOptions{GracePeriodSeconds: &grace})
	}
}
