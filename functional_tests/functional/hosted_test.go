// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

// runHostedClusterTests runs tests that are specific to hosted clusters like EKS, GKE, AKS, etc.
// The test is specific to cloud provider data, example: resource attributes validation.
func runHostedClusterTests(t *testing.T, kubeTestEnv string) {
	testKubeConfig := requireEnv(t, "KUBECONFIG")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)
	switch kubeTestEnv {
	case eksTestKubeEnv, eksAutoModeTestKubeEnv, aksTestKubeEnv, gkeTestKubeEnv, rosaTestKubeEnv, gceTestKubeEnv:
		expectedValuesDir = selectExpectedValuesDir(kubeTestEnv)
		t.Run("agent resource attributes validation", func(t *testing.T) {
			validateResourceAttributes(t, client, kubeConfig, roleAgent)
		})
		t.Run("cluster receiver self-metrics resource attributes validation", func(t *testing.T) {
			validateResourceAttributes(t, client, kubeConfig, roleClusterReceiver)
		})
		t.Run("cluster receiver k8s cluster metrics resource attributes validation", func(t *testing.T) {
			validateResourceAttributes(t, client, kubeConfig, roleClusterReceiverK8s)
		})

		t.Run("component error logs checks", func(t *testing.T) {
			if kubeTestEnv == eksTestKubeEnv {
				internal.CheckComponentHealth(t, client, internal.DefaultNamespace, internal.AgentLabelSelector, journaldReceiverName, 500)
			}
			internal.CheckComponentHealth(t, client, internal.DefaultNamespace, internal.AgentLabelSelector, kubeletstatsReceiverName, 500)
			internal.CheckComponentHealth(t, client, internal.DefaultNamespace, clusterReceiverLabelSelector, k8sClusterReceiverName, 500)
		})

		if kubeTestEnv == rosaTestKubeEnv || kubeTestEnv == gceTestKubeEnv {
			t.Run("control plane receiver checks", func(t *testing.T) {
				runControlPlaneReceiverChecks(t, client, controlPlaneReceiversForEnv(kubeTestEnv))
			})
		}
	case autopilotTestKubeEnv:
		t.Run("component error logs checks", func(t *testing.T) {
			internal.CheckPodsReady(t, client, internal.DefaultNamespace, internal.AgentLabelSelector, 3*time.Minute, 10*time.Second)
			internal.CheckPodsReady(t, client, internal.DefaultNamespace, clusterReceiverLabelSelector, 3*time.Minute, 10*time.Second)

			internal.CheckComponentHealth(t, client, internal.DefaultNamespace, internal.AgentLabelSelector, kubeletstatsReceiverName, 500)
			internal.CheckComponentHealth(t, client, internal.DefaultNamespace, clusterReceiverLabelSelector, k8sClusterReceiverName, 500)
		})
	default:
		assert.Failf(t, "failed to run runHostedClusterTests", "no test available for kubeTestEnv %s", kubeTestEnv)
	}
}

func selectExpectedValuesDir(kubeTestEnv string) string {
	switch kubeTestEnv {
	case eksAutoModeTestKubeEnv:
		return eksAutoModeValuesDir
	case aksTestKubeEnv:
		return aksValuesDir
	case gkeTestKubeEnv:
		return gkeValuesDir
	case rosaTestKubeEnv:
		return rosaValuesDir
	case gceTestKubeEnv:
		return gceValuesDir
	default:
		return eksValuesDir
	}
}

func validateResourceAttributes(t *testing.T, clientset *kubernetes.Clientset, kubeConfig *rest.Config, role collectorRole) {
	var labelSelector, expectedResourceAttributesFile, podPathFile string

	switch role {
	case roleAgent:
		labelSelector = internal.AgentLabelSelector
		expectedResourceAttributesFile = filepath.Join(testDir, expectedValuesDir, "expected_resource_attributes_agent.yaml")
	case roleClusterReceiver:
		labelSelector = clusterReceiverLabelSelector
		expectedResourceAttributesFile = filepath.Join(testDir, expectedValuesDir, "expected_resource_attributes_cluster_receiver.yaml")
	case roleClusterReceiverK8s:
		labelSelector = clusterReceiverLabelSelector
		expectedResourceAttributesFile = filepath.Join(testDir, expectedValuesDir, "expected_resource_attributes_cluster_receiver_k8s_cluster.yaml")
	default:
		require.Failf(t, "failed to run validateResourceAttributes", "unknown role %q", role)
	}

	pods, err := internal.GetPods(t, clientset, internal.DefaultNamespace, labelSelector)
	require.NoError(t, err)
	require.NotEmpty(t, pods.Items, "no pods found for label %s", labelSelector)

	podName := pods.Items[0].Name
	isWindows := strings.ToLower(pods.Items[0].Labels["osType"]) == "windows"

	if role == roleClusterReceiverK8s {
		podPathFile = linuxPodK8sClusterMetricsPath
		if isWindows {
			podPathFile = winPodK8sClusterMetricsPath
		}
	} else {
		podPathFile = linuxPodMetricsPath
		if isWindows {
			podPathFile = winPodMetricsPath
		}
	}

	tmpFile, err := os.CreateTemp(t.TempDir(), "actualResourceAttributes*.yaml")
	require.NoError(t, err)

	internal.CopyFileFromPod(t, clientset, kubeConfig, internal.DefaultNamespace, podName, "otel-collector", podPathFile, tmpFile.Name())

	skipKeys := []string{"k8s.cluster.name", "cloud.platform"}
	expectedResourceAttributes := readAndNormalizeMetrics(t, expectedResourceAttributesFile, skipKeys...).ResourceMetrics().At(0).Resource().Attributes()

	// The k8s_cluster receiver emits multiple ResourceMetrics groups.
	// We pick a container resource for a stable comparison.
	var actualResourceAttributes pcommon.Map
	if role == roleClusterReceiverK8s {
		actualResourceAttributes = findResourceByAttr(t, tmpFile.Name(), "k8s.container.name", skipKeys...)
	} else {
		actualResourceAttributes = readAndNormalizeMetrics(t, tmpFile.Name(), skipKeys...).ResourceMetrics().At(0).Resource().Attributes()
	}

	require.True(t, expectedResourceAttributes.Equal(actualResourceAttributes), "Resource Attributes comparison failed for %s , expected values %s , actual values %s", role, internal.FormatAttributes(expectedResourceAttributes), internal.FormatAttributes(actualResourceAttributes))

	t.Cleanup(func() {
		require.NoError(t, os.Remove(tmpFile.Name()))
	})
}

func readAndNormalizeMetrics(t *testing.T, filePath string, skipKeys ...string) pmetric.Metrics {
	metrics, err := golden.ReadMetrics(filePath)
	require.NoError(t, err)
	require.Positive(t, metrics.ResourceMetrics().Len(), "metrics file %s contains no ResourceMetrics", filePath)
	internal.NormalizeAttributes(metrics.ResourceMetrics().At(0).Resource().Attributes(), skipKeys...)
	return metrics
}

// findResourceByAttr reads metrics from filePath, finds the first ResourceMetrics
// whose resource attributes contain the given key, normalizes it, and returns the attributes.
func findResourceByAttr(t *testing.T, filePath string, attrKey string, skipKeys ...string) pcommon.Map {
	metrics, err := golden.ReadMetrics(filePath)
	require.NoError(t, err)
	rm := metrics.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		attrs := rm.At(i).Resource().Attributes()
		if _, ok := attrs.Get(attrKey); ok {
			internal.NormalizeAttributes(attrs, skipKeys...)
			return attrs
		}
	}
	require.Failf(t, "resource not found", "no ResourceMetrics with attribute %q in %s", attrKey, filePath)
	return pcommon.NewMap()
}
