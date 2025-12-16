// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package obi

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	testDir      = "testdata"
	valuesDir    = "values"
	manifestsDir = "python"
)

func Test_OBI_Minimal_Traces(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("OBI functional test runs only on Linux")
	}

	kubeconfig, ok := os.LookupEnv("KUBECONFIG")
	require.True(t, ok, "KUBECONFIG must be set")

	// Start a local OTLP sink on ports 4317/4318. OBI defaults export to ${HOST_IP}:4317 (gRPC).
	tracesSink := internal.SetupOTLPTracesSink(t)

	// Install the collector chart with OBI enabled using a minimal values template.
	valuesFile, err := filepath.Abs(filepath.Join(testDir, valuesDir, "obi_values.yaml.tmpl"))
	require.NoError(t, err)

	hostEp := internal.HostEndpoint(t)
	if strings.Contains(hostEp, ":") {
		hostEp = fmt.Sprintf("[%s]", hostEp)
	}
	internal.ChartInstallOrUpgrade(t, kubeconfig, valuesFile, map[string]any{
		"OTLPEndpoint": fmt.Sprintf("%s:%d", hostEp, internal.OTLPGRPCReceiverPort),
	}, 0, internal.GetDefaultChartOptions())

	// Ensure chart resources are removed after test unless explicitly skipped.
	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		internal.ChartUninstall(t, kubeconfig)
	})

	// Deploy the simple Python app without language auto-instrumentation annotations.
	// The container generates HTTP traffic to localhost periodically.
	deployPythonApp(t, kubeconfig)

	// Wait until at least one trace is received at the OTLP sink.
	internal.WaitForTraces(t, 1, tracesSink)
}

func deployPythonApp(t *testing.T, kubeconfig string) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	// Read and create/update the Deployment for the Python app in the default namespace.
	stream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "deployment.yaml"))
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

	// Wait for the Python pod(s) to be Ready for a short stabilization period.
	internal.CheckPodsReady(t, client, internal.DefaultNamespace, "app=python-test", 5*time.Minute, 15*time.Second)

	// Ensure the deployment gets cleaned up.
	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			return
		}
		grace := int64(0)
		_ = deployments.Delete(t.Context(), "python-test", metav1.DeleteOptions{GracePeriodSeconds: &grace})
	})
}
