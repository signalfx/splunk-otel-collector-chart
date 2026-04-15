// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	multilineTestNamespace      = "multiline-test"
	multilineContainerName      = "multiline-test"
	unmatchedContainerName      = "unmatched-logger"
	sidecarContainerName        = "sidecar-proxy"
	multilineValuesTemplateFile = "multiline_values.yaml.tmpl"
	multilineTestdataDir        = "testdata"
	multilineManifestsDir       = "testdata/multiline_testobjects"

	// multilineExpectedTotalRecords is the number of distinct logical log records in the
	// ConfigMap: 5 single-line entries + 3 multiline blocks (Java NPE, Python traceback, Go panic).
	multilineExpectedTotalRecords = 8
)

// Env vars to control the test behavior
// TEARDOWN_BEFORE_SETUP: if set to true, the test will run teardown before setup
// SKIP_SETUP: if set to true, the test will skip setup
// SKIP_TEARDOWN: if set to true, the test will skip teardown
// SKIP_TESTS: if set to true, the test will skip the test
// KUBECONFIG: the path to the kubeconfig file
func Test_MultilineLogs(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	config, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		t.Log("Running teardown before setup as TEARDOWN_BEFORE_SETUP is set to true")
		multilineTeardown(t, k8sClient)
	}

	logsConsumer := internal.SetupHECLogsSink(t)

	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		multilineDeployWorkloadAndCollector(t, testKubeConfig, clientset, k8sClient)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	t.Run("MultilineRecombinedAsOneRecord", func(t *testing.T) {
		checkMultilineRecombined(t, logsConsumer)
	})

	t.Run("SingleLinePassthrough", func(t *testing.T) {
		checkSingleLinePassthrough(t, logsConsumer)
	})

	t.Run("DefaultRoutePassthrough", func(t *testing.T) {
		checkDefaultRoutePassthrough(t, logsConsumer)
	})

	t.Run("SidecarUnmatchedLinesNotBatched", func(t *testing.T) {
		checkSidecarUnmatchedLinesNotBatched(t, logsConsumer)
	})
}

// checkMultilineRecombined asserts that each multiline stack trace in input.log
// arrives at the HEC sink as a single log record containing the full trace body.
func checkMultilineRecombined(t *testing.T, logsConsumer *consumertest.LogsSink) {
	t.Helper()
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		bodies := collectBodiesFromContainer(logsConsumer, multilineContainerName)

		// Java NPE block: first line begins with a timestamp, continuation lines
		// start with whitespace ("at com.example..."). All four lines must be in
		// one record body.
		javaRecord := findBodyContaining(bodies, "com.example.service.UserService.getUser")
		if assert.NotNil(tt, javaRecord, "Java stack trace must arrive as a single record") {
			assert.Contains(tt, *javaRecord, "java.lang.NullPointerException")
			assert.Contains(tt, *javaRecord, "at com.example.controller.UserController")
			assert.Contains(tt, *javaRecord, "at com.example.app.DispatcherServlet")
		}

		// Python traceback block: "Traceback (most recent call last):" starts the
		// continuation; the OperationalError closes it.
		pyRecord := findBodyContaining(bodies, "sqlite3.OperationalError")
		if assert.NotNil(tt, pyRecord, "Python traceback must arrive as a single record") {
			assert.Contains(tt, *pyRecord, "Traceback (most recent call last):")
			assert.Contains(tt, *pyRecord, `File "/app/worker.py"`)
			assert.Contains(tt, *pyRecord, `File "/app/db.py"`)
		}

		// Go panic block: "goroutine 1 [running]:" is the continuation header.
		goRecord := findBodyContaining(bodies, "goroutine 1 [running]:")
		if assert.NotNil(tt, goRecord, "Go panic must arrive as a single record") {
			assert.Contains(tt, *goRecord, "runtime/debug.Stack()")
			assert.Contains(tt, *goRecord, "main.recoverPanic")
		}
	}, 3*time.Minute, 5*time.Second)
}

// checkSingleLinePassthrough asserts that every single-line entry in input.log
// arrives as its own individual log record from the multiline-test container.
func checkSingleLinePassthrough(t *testing.T, logsConsumer *consumertest.LogsSink) {
	t.Helper()
	singleLineMarkers := []string{
		"Application started successfully",
		"Retrying database connection",
		"Cache miss for key user-profile-99",
		"Health check passed",
		"Scheduled job completed",
	}

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		bodies := collectBodiesFromContainer(logsConsumer, multilineContainerName)

		for _, marker := range singleLineMarkers {
			record := findBodyContaining(bodies, marker)
			if assert.NotNilf(tt, record, "single-line entry %q must pass through as its own record", marker) {
				assert.NotContainsf(tt, *record, "\n",
					"single-line entry %q must not be recombined into a multiline record, got body: %q", marker, *record)
			}
		}

		// Sanity check: the sink accumulates records across loop iterations (input.log
		// repeats every 30s), so the count grows over time. Assert at least one full
		// cycle worth of records has arrived.
		assert.GreaterOrEqualf(tt, len(bodies), multilineExpectedTotalRecords,
			"expected at least %d log records (one full cycle: 5 single-line + 3 multiline stacks), got %d",
			multilineExpectedTotalRecords, len(bodies))
	}, 3*time.Minute, 5*time.Second)
}

// checkDefaultRoutePassthrough asserts that logs from a container that does NOT
// match any multilineConfigs rule still arrive at the HEC sink. This is the true
// regression test for a missing router default: route — if it were absent, logs
// from unmatched containers would be silently dropped.
func checkDefaultRoutePassthrough(t *testing.T, logsConsumer *consumertest.LogsSink) {
	t.Helper()
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		bodies := collectBodiesFromContainer(logsConsumer, unmatchedContainerName)
		assert.NotEmptyf(tt, bodies,
			"logs from container %q (not matching any multilineConfigs rule) must reach the sink via the default route",
			unmatchedContainerName)
		assert.NotNil(tt, findBodyContaining(bodies, "UNMATCHED_LOG_MARKER"),
			"expected UNMATCHED_LOG_MARKER in logs from %q", unmatchedContainerName)
	}, 3*time.Minute, 5*time.Second)
}

// checkSidecarUnmatchedLinesNotBatched reproduces the mixed-container scenario
// where a namespace-scoped multilineConfigs rule (no containerName filter) is
// applied to a pod that has both a java-style app container and a sidecar emitting
// JSON access logs. JSON lines don't match firstEntryRegex, so the recombine
// operator treats them as continuations. With the default max_unmatched_batch_size
// of 100 those lines would be silently merged into a single record.
// This test asserts that each JSON access log line arrives as its own record,
// documenting the expected behavior and catching regressions if max_unmatched_batch_size
// is ever wired up in the chart.
func checkSidecarUnmatchedLinesNotBatched(t *testing.T, logsConsumer *consumertest.LogsSink) {
	t.Helper()
	accessLogMarkers := []string{
		`"path":"/api/users"`,
		`"path":"/api/orders"`,
		`"path":"/healthz"`,
	}
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		bodies := collectBodiesFromContainer(logsConsumer, sidecarContainerName)
		for _, marker := range accessLogMarkers {
			record := findBodyContaining(bodies, marker)
			if assert.NotNilf(tt, record, "access log line containing %q must arrive as its own record", marker) {
				assert.NotContainsf(tt, *record, "\n",
					"access log line %q must not be batched with other lines, got body: %q", marker, *record)
			}
		}
	}, 3*time.Minute, 5*time.Second)
}

// collectBodiesFromContainer returns the body string of every log record whose
// k8s.container.name attribute matches the given container name.
func collectBodiesFromContainer(logsConsumer *consumertest.LogsSink, container string) []string {
	var bodies []string
	for _, logs := range logsConsumer.AllLogs() {
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			rl := logs.ResourceLogs().At(i)
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				sl := rl.ScopeLogs().At(j)
				for k := 0; k < sl.LogRecords().Len(); k++ {
					lr := sl.LogRecords().At(k)
					v, ok := lr.Attributes().Get("k8s.container.name")
					if !ok || v.AsString() != container {
						continue
					}
					bodies = append(bodies, lr.Body().AsString())
				}
			}
		}
	}
	return bodies
}

// findBodyContaining returns a pointer to the first body in bodies that contains
// substr, or nil if none does.
func findBodyContaining(bodies []string, substr string) *string {
	for i := range bodies {
		if strings.Contains(bodies[i], substr) {
			return &bodies[i]
		}
	}
	return nil
}

func multilineDeployWorkloadAndCollector(t *testing.T, testKubeConfig string, clientset *kubernetes.Clientset, k8sClient *k8stest.K8sClient) {
	t.Helper()

	valuesFile, err := filepath.Abs(filepath.Join(multilineTestdataDir, multilineValuesTemplateFile))
	require.NoError(t, err)

	hostEp := internal.HostEndpoint(t)
	require.NotEmpty(t, hostEp, "host endpoint not found")

	replacements := map[string]any{
		"LogURL": internal.HostPortHTTP(hostEp, internal.HECLogsReceiverPort),
	}
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 0, internal.GetDefaultChartOptions())

	// Wait for the otel-agent pods to be ready before deploying the workload so
	// the filelog receiver is already watching when logs first appear.
	internal.CheckPodsReady(t, clientset, internal.DefaultNamespace, "component=otel-collector-agent", 3*time.Minute, 5*time.Second)

	// Deploy the workload into its own namespace.
	internal.CreateNamespace(t, clientset, multilineTestNamespace)
	internal.WaitForDefaultServiceAccount(t, clientset, multilineTestNamespace)

	createdObjs, err := k8stest.CreateObjects(k8sClient, multilineManifestsDir)
	require.NoError(t, err)
	require.NotEmpty(t, createdObjs)

	internal.CheckPodsReady(t, clientset, multilineTestNamespace, "app=multiline-test", 2*time.Minute, 0)
	internal.CheckPodsReady(t, clientset, multilineTestNamespace, "app=unmatched-logger", 2*time.Minute, 0)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		multilineTeardown(t, k8sClient)
	})
}

func multilineTeardown(t *testing.T, k8sClient *k8stest.K8sClient) {
	t.Helper()
	testKubeConfig := os.Getenv("KUBECONFIG")
	internal.ChartUninstall(t, testKubeConfig)

	internal.DeleteObject(t, k8sClient, `
apiVersion: v1
kind: Namespace
metadata:
  name: multiline-test
`)
}
