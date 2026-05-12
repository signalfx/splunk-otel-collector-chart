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
	otlpAnnotationPrecedenceNamespace    = "otlp-annotation-precedence"
	otlpAnnotationPrecedenceValuesFile   = "otlp_annotation_precedence_values.yaml.tmpl"
	otlpAnnotationPrecedenceTestdataDir  = "testdata"
	otlpAnnotationPrecedenceManifestsDir = "testdata/otlp_annotation_precedence_testobjects"
	otlpAnnotationPrecedenceContainer    = "otlp-annotation-precedence"
	otlpAnnotationPrecedenceMarker       = "OTLP_ANNOTATION_PRECEDENCE_MARKER"
	otlpAnnotationPodIndex               = "pod-otlp-index"
	otlpAnnotationPodSourcetype          = "pod-otlp-sourcetype"
)

// Test_OTLPAnnotationPrecedence verifies that the default index processor added
// for OTLP platform logs does not override Kubernetes annotations collected by
// the k8sattributes and resource/logs processors.
func Test_OTLPAnnotationPrecedence(t *testing.T) {
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
		otlpAnnotationPrecedenceTeardown(t, k8sClient)
	}

	logsConsumer := internal.SetupOTLPLogsSink(t)

	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		otlpAnnotationPrecedenceDeployWorkloadAndCollector(t, testKubeConfig, clientset, k8sClient)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		attrs, found := findResourceAttrsForLog(logsConsumer, otlpAnnotationPrecedenceMarker)
		if assert.True(tt, found, "expected OTLP log record with marker %q", otlpAnnotationPrecedenceMarker) {
			assert.Equal(tt, otlpAnnotationPodIndex, attrs["com.splunk.index"])
			assert.Equal(tt, otlpAnnotationPodSourcetype, attrs["com.splunk.sourcetype"])
		}
	}, 3*time.Minute, 5*time.Second)
}

func otlpAnnotationPrecedenceDeployWorkloadAndCollector(t *testing.T, testKubeConfig string, clientset *kubernetes.Clientset, k8sClient *k8stest.K8sClient) {
	t.Helper()

	valuesFile, err := filepath.Abs(filepath.Join(otlpAnnotationPrecedenceTestdataDir, otlpAnnotationPrecedenceValuesFile))
	require.NoError(t, err)

	hostEp := internal.HostEndpoint(t)
	require.NotEmpty(t, hostEp, "host endpoint not found")

	replacements := map[string]any{
		"OtlpEndpoint": internal.HostPort(hostEp, internal.OTLPGRPCReceiverPort),
	}
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 0, internal.GetDefaultChartOptions())

	internal.CheckPodsReady(t, clientset, internal.DefaultNamespace, internal.AgentLabelSelector, 3*time.Minute, 5*time.Second)

	internal.CreateNamespace(t, clientset, otlpAnnotationPrecedenceNamespace)
	internal.AnnotateNamespace(t, clientset, otlpAnnotationPrecedenceNamespace, "splunk.com/index", "namespace-otlp-index")
	internal.WaitForDefaultServiceAccount(t, clientset, otlpAnnotationPrecedenceNamespace)

	createdObjs, err := k8stest.CreateObjects(k8sClient, otlpAnnotationPrecedenceManifestsDir)
	require.NoError(t, err)
	require.NotEmpty(t, createdObjs)

	internal.CheckPodsReady(t, clientset, otlpAnnotationPrecedenceNamespace, "app=otlp-annotation-precedence", 2*time.Minute, 0)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		otlpAnnotationPrecedenceTeardown(t, k8sClient)
	})
}

func findResourceAttrsForLog(logsConsumer *consumertest.LogsSink, marker string) (map[string]string, bool) {
	for _, logs := range logsConsumer.AllLogs() {
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			rl := logs.ResourceLogs().At(i)
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				sl := rl.ScopeLogs().At(j)
				for k := 0; k < sl.LogRecords().Len(); k++ {
					lr := sl.LogRecords().At(k)
					if !strings.Contains(lr.Body().AsString(), marker) {
						continue
					}
					attrs := make(map[string]string)
					for _, key := range []string{"com.splunk.index", "com.splunk.sourcetype"} {
						if value, exists := rl.Resource().Attributes().Get(key); exists {
							attrs[key] = value.AsString()
						}
					}
					return attrs, true
				}
			}
		}
	}
	return nil, false
}

func otlpAnnotationPrecedenceTeardown(t *testing.T, k8sClient *k8stest.K8sClient) {
	t.Helper()
	testKubeConfig := os.Getenv("KUBECONFIG")
	internal.ChartUninstall(t, testKubeConfig)

	internal.DeleteObject(t, k8sClient, `
apiVersion: v1
kind: Namespace
metadata:
  name: otlp-annotation-precedence
`)
}
