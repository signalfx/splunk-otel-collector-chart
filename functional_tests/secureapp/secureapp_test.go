// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

// Package secureapp contains a functional test for the SecureApp auto-instrumentation
// feature (splunkObservability.secureAppEnabled=true).
//
// The test deploys the standard java_test workload with the SecureApp Instrumentation
// CR active. The OTel Operator injects the CSA Java agent image. A small set of
// attack-pattern HTTP requests are sent to the pod via kubectl exec to trigger CSA
// security events. The test asserts that OTLP log records with
// instrumentation_scope.name == "secureapp" arrive at the local /v3/event sink,
// proving the full pipeline:
//
//	CSA agent → otlp receiver (agent) → routing/logs → logs/secureapp → otlp_http/secureapp → sink
//
// Environment variables:
//
//	TEARDOWN_BEFORE_SETUP   – run teardown before setup when set to "true"
//	SKIP_SETUP              – skip chart/app installation when set to "true"
//	SKIP_TEARDOWN           – skip cleanup when set to "true"
//	SKIP_TESTS              – skip assertions when set to "true"
//	KUBECONFIG              – path to kubeconfig file (required)
package secureapp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	secureAppLogsPort = internal.SecureAppLogsReceiverPort // 4320
	javaAppLabel      = "app=java-test"
	javaContainerName = "java-test"
	testDir           = "testdata"
)

// javaDeploymentPath is the java_test deployment manifest shared with the functional suite.
// Using the same file ensures the workload is identical to the one exercised in the
// existing auto-instrumentation tests — no duplication, no drift.
var javaDeploymentPath = filepath.Join("..", "functional", "testdata", "java", "deployment.yaml")

var decoder = serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer()

func Test_SecureApp_JavaAttackEvents(t *testing.T) {
	kubeconfig, hasKubeconfig := os.LookupEnv("KUBECONFIG")
	require.True(t, hasKubeconfig, "KUBECONFIG environment variable must be set")

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		t.Log("Running teardown before setup")
		internal.ChartUninstall(t, kubeconfig)
	}

	sink := internal.SetupOTLPLogsSinkOnPort(t, secureAppLogsPort, "/v3/event")
	internal.SetupSignalFxAPIServer(t)

	if os.Getenv("SKIP_SETUP") != "true" {
		deployAll(t, kubeconfig)
	}

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			return
		}
		internal.ChartUninstall(t, kubeconfig)
	})

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	triggerAttacks(t, kubeconfig)

	assertSecureAppEvents(t, sink)
}

// deployAll installs the Helm chart and the java_test workload.
func deployAll(t *testing.T, kubeconfig string) {
	hostEp := internal.HostEndpoint(t)
	require.NotEmpty(t, hostEp, "host endpoint not found")

	valuesFile, err := filepath.Abs(filepath.Join(testDir, "secureapp_values.yaml.tmpl"))
	require.NoError(t, err)

	replacements := map[string]any{
		"IngestURL": internal.HostPortHTTP(hostEp, secureAppLogsPort),
		"ApiURL":    internal.HostPortHTTP(hostEp, internal.SignalFxAPIPort),
	}
	internal.ChartInstallOrUpgrade(t, kubeconfig, valuesFile, replacements, 0, internal.GetDefaultChartOptions())

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)

	// Wait for the OTel operator to be ready before deploying the workload so that
	// the Instrumentation CR webhook is active and the CSA image swap happens.
	internal.CheckPodsReady(t, clientset, internal.DefaultNamespace,
		"app.kubernetes.io/name=opentelemetry-operator", 5*time.Minute, 0)

	deployJavaApp(t, clientset)

	// The operator mutates the pod on creation. Wait for the mutated pod (with the
	// CSA init-container) to reach Ready before triggering attacks.
	internal.CheckPodsReady(t, clientset, internal.DefaultNamespace, javaAppLabel, 5*time.Minute, 10*time.Second)
}

// deployJavaApp creates the java_test Deployment in the default namespace.
// If it already exists it is updated in-place — same semantics as the functional suite.
func deployJavaApp(t *testing.T, clientset *kubernetes.Clientset) {
	data, err := os.ReadFile(javaDeploymentPath)
	require.NoError(t, err)

	obj, _, err := decoder.Decode(data, nil, nil)
	require.NoError(t, err)

	dep, ok := obj.(runtime.Object)
	require.True(t, ok)

	deployments := clientset.AppsV1().Deployments(internal.DefaultNamespace)
	_, createErr := deployments.Create(t.Context(), dep.(*appsv1.Deployment), metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(createErr) {
		_, updateErr := deployments.Update(t.Context(), dep.(*appsv1.Deployment), metav1.UpdateOptions{})
		require.NoError(t, updateErr)
	} else {
		require.NoError(t, createErr)
	}
}

// triggerAttacks execs into the java-test pod and fires a small set of
// attack-pattern HTTP requests against the local Tomcat server. CSA detects
// these at the Java agent layer — Tomcat does not need a vulnerable endpoint;
// the agent intercepts the raw request before it reaches servlet code.
func triggerAttacks(t *testing.T, kubeconfig string) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)

	pods, err := clientset.CoreV1().Pods(internal.DefaultNamespace).List(
		t.Context(), metav1.ListOptions{LabelSelector: javaAppLabel})
	require.NoError(t, err)
	require.NotEmpty(t, pods.Items, "java-test pod not found")
	podName := pods.Items[0].Name

	attacks := []string{
		// SQL injection — detected by CSA SQL taint analysis
		"http://localhost:8080/?id=1+OR+1%3D1",
		// Path traversal — detected by CSA file access monitor
		"http://localhost:8080/?file=../../etc/passwd",
	}
	for _, url := range attacks {
		// Errors are ignored: Tomcat returns 400/404 for these URLs but CSA fires
		// on the request pattern regardless of the HTTP response status.
		_, _ = internal.ExecInPod(t, restConfig, clientset,
			internal.DefaultNamespace, podName, javaContainerName,
			[]string{"curl", "-sf", url})
	}
}

// assertSecureAppEvents waits for at least one OTLP log batch containing a
// scope named "secureapp" and asserts stable resource attributes on it.
// A full golden-file comparison is intentionally avoided because event payload
// details (attack type, stack trace, CVE details) vary across CSA versions.
func assertSecureAppEvents(t *testing.T, sink *consumertest.LogsSink) {
	var foundBatchIdx int
	require.Eventually(t, func() bool {
		for i := len(sink.AllLogs()) - 1; i >= 0; i-- {
			rl := sink.AllLogs()[i].ResourceLogs()
			for j := 0; j < rl.Len(); j++ {
				sl := rl.At(j).ScopeLogs()
				for k := 0; k < sl.Len(); k++ {
					if sl.At(k).Scope().Name() == "secureapp" && sl.At(k).LogRecords().Len() > 0 {
						foundBatchIdx = i
						return true
					}
				}
			}
		}
		return false
	}, 3*time.Minute, 5*time.Second,
		"timed out waiting for SecureApp log records with instrumentation_scope.name=secureapp")

	batch := sink.AllLogs()[foundBatchIdx]
	rl := batch.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		res := rl.At(i).Resource().Attributes()
		sl := rl.At(i).ScopeLogs()
		for j := 0; j < sl.Len(); j++ {
			if sl.At(j).Scope().Name() != "secureapp" {
				continue
			}
			// Verify that k8s_attributes processor enriched the resource.
			sdkLang, ok := res.Get("telemetry.sdk.language")
			assert.True(t, ok, "resource must have telemetry.sdk.language")
			if ok {
				assert.Equal(t, "java", sdkLang.Str())
			}

			svcName, ok := res.Get("service.name")
			assert.True(t, ok, "resource must have service.name")
			if ok {
				assert.Equal(t, "java-test", svcName.Str())
			}

			assert.Positive(t, sl.At(j).LogRecords().Len(),
				"secureapp scope must contain at least one log record")
		}
	}
}
