// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	signalFxReceiverK8sClusterReceiverPort = 19443
	kindTestKubeEnv                        = "kind"
	eksTestKubeEnv                         = "eks"
	eksAutoModeTestKubeEnv                 = "eks/auto-mode"
	eksFargateTestKubeEnv                  = "eks/fargate"
	gkeTestKubeEnv                         = "gke"
	autopilotTestKubeEnv                   = "gke/autopilot"
	aksTestKubeEnv                         = "aks"
	rosaTestKubeEnv                        = "rosa"
	gceTestKubeEnv                         = "gce"
	testDir                                = "testdata"
	valuesDir                              = "values"
	manifestsDir                           = "manifests"
	kindValuesDir                          = "expected_kind_values"
	eksValuesDir                           = "expected_eks_values"
	eksAutoModeValuesDir                   = "expected_eks_auto_mode_values"
	aksValuesDir                           = "expected_aks_values"
	gkeValuesDir                           = "expected_gke_values"
	rosaValuesDir                          = "expected_rosa_values"
	gceValuesDir                           = "expected_gce_values"
	clusterReceiverLabelSelector           = "component=otel-k8s-cluster-receiver"
	splunkOtelCollectorTAResourceName      = "splunk-otel-collector-ta"
	taResourceName                         = "targetallocator-ta"
	kindAgentPodNamePrefix                 = internal.DefaultChartReleaseName + "-splunk-otel-collector-agent"
	linuxPodMetricsPath                    = "/splunk-metrics/metrics.json"
	winPodMetricsPath                      = "C:\\Users\\ContainerUser\\AppData\\Local\\Temp\\metrics.json"
	linuxPodK8sClusterMetricsPath          = "/splunk-metrics/k8s_cluster_metrics.json"
	winPodK8sClusterMetricsPath            = "C:\\Users\\ContainerUser\\AppData\\Local\\Temp\\k8s_cluster_metrics.json"
	aksWindowsValidationResourceName       = "aks-win-validation-secret"
)

type collectorRole string

const (
	roleAgent              collectorRole = "agent"
	roleClusterReceiver    collectorRole = "cluster_receiver"
	roleClusterReceiverK8s collectorRole = "cluster_receiver_k8s_cluster"
)

var expectedValuesDir string

// Component names for health checks
const (
	kubeletstatsReceiverName = "kubeletstatsreceiver"
	k8sClusterReceiverName   = "k8sclusterreceiver"
	journaldReceiverName     = "journald"
)

func Test_Functions(t *testing.T) {
	kubeTestEnv := requireEnv(t, "KUBE_TEST_ENV")
	skipSetup := os.Getenv("SKIP_SETUP") == "true"
	skipTests := os.Getenv("SKIP_TESTS") == "true"
	switch kubeTestEnv {
	case kindTestKubeEnv:
		setupSinks(t)
		expectedValuesDir = kindValuesDir
	case eksTestKubeEnv, eksAutoModeTestKubeEnv, gkeTestKubeEnv, autopilotTestKubeEnv, aksTestKubeEnv, rosaTestKubeEnv, gceTestKubeEnv:
	case eksFargateTestKubeEnv:
		require.True(t, skipTests, "EKS Fargate supports chart install/upgrade checks only; set SKIP_TESTS=true")
	default:
		t.Fatalf("unsupported KUBE_TEST_ENV %q", kubeTestEnv)
	}
	testKubeConfig := requireEnv(t, "KUBECONFIG")

	internal.AcquireLeaseForTest(t, testKubeConfig)

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t.Context(), t, testKubeConfig)
	}

	if !skipSetup {
		kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
		require.NoError(t, err)
		client, err := kubernetes.NewForConfig(kubeConfig)
		require.NoError(t, err)
		extensionsClient, err := clientset.NewForConfig(kubeConfig)
		require.NoError(t, err)
		dynamicClient, err := dynamic.NewForConfig(kubeConfig)
		require.NoError(t, err)

		if kubeTestEnv == kindTestKubeEnv && os.Getenv("UPGRADE_FROM_VALUES") == "" {
			deployPersistentVolumeResources(t, client)
		}

		// Register cleanup before installation so failed installs are also removed.
		t.Cleanup(func() {
			if os.Getenv("SKIP_TEARDOWN") == "true" {
				t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
				return
			}
			t.Log("Cleaning up cluster")
			// t.Context is cancelled before cleanup runs.
			teardown(context.Background(), t, testKubeConfig)
		})

		deployCharts(t, testKubeConfig, client, extensionsClient)
		if !skipTests && kubeTestEnv == kindTestKubeEnv && os.Getenv("UPGRADE_FROM_VALUES") == "" {
			waitForInstallationHookPodsToDisappear(t, client)
			internal.ResetMetricsSink(t, globalSinks.k8sclusterReceiverMetricsConsumer)
			t.Run("kubernetes cluster metrics", testK8sClusterReceiverMetrics)
		}
		deployTestResources(t, client, dynamicClient)
	} else {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	}

	if skipTests {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	if kubeTestEnv == kindTestKubeEnv {
		if os.Getenv("UPGRADE_FROM_VALUES") != "" {
			runLocalClusterUpgradeTests(t)
		} else {
			runLocalClusterTests(t, skipSetup)
		}
	} else {
		runHostedClusterTests(t, kubeTestEnv)
	}
}

// waitForInstallationHookPodsToDisappear waits for the instrumentation hook
// Pods to disappear before the cluster metrics snapshot. Helm's hook-succeeded
// policy deletes the Job, but its Pods can otherwise still appear in the
// snapshot briefly.
func waitForInstallationHookPodsToDisappear(t *testing.T, client kubernetes.Interface) {
	t.Helper()

	pods := client.CoreV1().Pods(internal.DefaultNamespace)
	require.Eventually(t, func() bool {
		hookPods, err := pods.List(t.Context(), metav1.ListOptions{
			LabelSelector: "app=splunk-otel-collector-inst-hook",
		})
		if err != nil {
			t.Logf("failed to list instrumentation hook Pods: %v", err)
			return false
		}
		return len(hookPods.Items) == 0
	}, 30*time.Second, time.Second, "instrumentation hook Pods did not disappear")
}
