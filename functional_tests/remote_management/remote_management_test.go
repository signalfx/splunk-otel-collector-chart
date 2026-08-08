// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

// Package remotemanagement contains functional tests for Splunk Remote Management
// bridge installation.
package remotemanagement

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	bridgeName           = internal.DefaultChartReleaseName + "-splunk-otel-collector-opamp-bridge"
	bridgeConfigMapName  = bridgeName + "-config"
	agentConfigMapName   = internal.DefaultChartReleaseName + "-splunk-otel-collector-otel-agent"
	clusterConfigMapName = internal.DefaultChartReleaseName + "-splunk-otel-collector-otel-k8s-cluster-receiver"
	bridgeLabelSelector  = "component=opamp-bridge"
)

func Test_RemoteManagement(t *testing.T) {
	testKubeConfig, ok := os.LookupEnv("KUBECONFIG")
	require.True(t, ok, "the environment variable KUBECONFIG must be set")
	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		teardown(t, testKubeConfig)
	}

	internal.SetupSignalFxAPIServer(t)

	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		deployCollector(t, testKubeConfig)
	}
	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		teardown(t, testKubeConfig)
	})

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	clientset, err := internal.GetKubeClient(testKubeConfig)
	require.NoError(t, err)
	internal.CheckPodsReady(t, clientset, internal.DefaultNamespace, bridgeLabelSelector, 3*time.Minute, 0)

	t.Run("bridge deployment is configured", func(t *testing.T) {
		deploy := getBridgeDeployment(t, clientset)
		assert.Equal(t, int32(1), *deploy.Spec.Replicas)
		assert.Equal(t, bridgeName, deploy.Spec.Template.Spec.ServiceAccountName)

		container := findContainer(t, deploy, "bridge")
		assert.Equal(t, "ghcr.io/open-telemetry/opentelemetry-operator/operator-opamp-bridge:v0.158.0", container.Image)
		assert.Contains(t, container.Args, "--config-file=/conf/config.yaml")
		assert.Contains(t, container.Args, "--mode=standalone")
		assertEnvVar(t, container, "SPLUNK_OBSERVABILITY_ACCESS_TOKEN")
	})

	t.Run("bridge role grants required standalone permissions", func(t *testing.T) {
		role, roleErr := clientset.RbacV1().Roles(internal.DefaultNamespace).Get(t.Context(), bridgeName, metav1.GetOptions{})
		require.NoError(t, roleErr)
		assertRule(t, role, "", "configmaps", "get", "list", "watch", "update")
		assertRule(t, role, "apps", "daemonsets", "get", "list", "watch", "update", "patch")
		assertRule(t, role, "apps", "deployments", "get", "list", "watch", "update", "patch")
		assertRule(t, role, "apps", "statefulsets", "get", "list", "watch", "update", "patch")
	})

	t.Run("bridge config manages chart-created collectors", func(t *testing.T) {
		cm, cmErr := clientset.CoreV1().ConfigMaps(internal.DefaultNamespace).Get(t.Context(), bridgeConfigMapName, metav1.GetOptions{})
		require.NoError(t, cmErr)
		config := cm.Data["config.yaml"]
		require.NotEmpty(t, config)

		assert.Contains(t, config, "mode: standalone")
		assert.Contains(t, config, "endpoint: \"http://")
		assert.Contains(t, config, "X-SF-Token: \"${SPLUNK_OBSERVABILITY_ACCESS_TOKEN}\"")
		assert.Contains(t, config, "k8s.cluster.name: remote-management-test")
		assert.Contains(t, config, "kind: DaemonSet")
		assert.Contains(t, config, "name: \"sock-splunk-otel-collector-agent\"")
		assert.Contains(t, config, "name: \"sock-splunk-otel-collector-otel-agent\"")
		assert.Contains(t, config, "kind: Deployment")
		assert.Contains(t, config, "name: \"sock-splunk-otel-collector-k8s-cluster-receiver\"")
		assert.Contains(t, config, "name: \"sock-splunk-otel-collector-otel-k8s-cluster-receiver\"")
		assert.Contains(t, config, "otelcol.service.mode: agent")
		assert.Contains(t, config, "otelcol.service.mode: clusterReceiver")
	})

	t.Run("collector direct opamp config is disabled", func(t *testing.T) {
		agentConfig := getRelayConfig(t, clientset, agentConfigMapName)
		assert.NotContains(t, agentConfig, "opamp/splunk_o11y")
		assert.NotContains(t, agentConfig, "http_forwarder/opamp_splunk_o11y")

		clusterConfig := getRelayConfig(t, clientset, clusterConfigMapName)
		assert.NotContains(t, clusterConfig, "opamp/splunk_o11y")
	})
}

func deployCollector(t *testing.T, kubeConfig string) {
	t.Helper()
	hostEp := internal.HostEndpoint(t)
	valuesFile, err := filepath.Abs(filepath.Join("testdata", "remote_management_values.yaml.tmpl"))
	require.NoError(t, err)
	endpoint := internal.HostPortHTTP(hostEp, internal.SignalFxAPIPort)
	internal.ChartInstallOrUpgrade(t, kubeConfig, valuesFile, map[string]any{
		"ApiURL":    endpoint,
		"IngestURL": endpoint,
	}, 0, internal.GetDefaultChartOptions())
}

func teardown(t *testing.T, kubeConfig string) {
	t.Helper()
	internal.ChartUninstall(t, kubeConfig)
}

func getBridgeDeployment(t *testing.T, clientset kubernetes.Interface) *appsv1.Deployment {
	t.Helper()
	deploy, err := clientset.AppsV1().Deployments(internal.DefaultNamespace).Get(t.Context(), bridgeName, metav1.GetOptions{})
	require.NoError(t, err)
	return deploy
}

func findContainer(t *testing.T, deploy *appsv1.Deployment, name string) corev1.Container {
	t.Helper()
	for _, container := range deploy.Spec.Template.Spec.Containers {
		if container.Name == name {
			return container
		}
	}
	require.Failf(t, "container not found", "container %q was not found in deployment %q", name, deploy.Name)
	return corev1.Container{}
}

func assertEnvVar(t *testing.T, container corev1.Container, name string) {
	t.Helper()
	for _, env := range container.Env {
		if env.Name == name {
			assert.NotNil(t, env.ValueFrom)
			return
		}
	}
	require.Failf(t, "env var not found", "env var %q was not found in container %q", name, container.Name)
}

func assertRule(t *testing.T, role *rbacv1.Role, apiGroup, resource string, verbs ...string) {
	t.Helper()
	for _, rule := range role.Rules {
		if !slices.Contains(rule.APIGroups, apiGroup) || !slices.Contains(rule.Resources, resource) {
			continue
		}
		for _, verb := range verbs {
			assert.Contains(t, rule.Verbs, verb, "role %s rule for %s/%s", role.Name, apiGroup, resource)
		}
		return
	}
	require.Failf(t, "rbac rule not found", "role %s does not contain a rule for %s/%s", role.Name, apiGroup, resource)
}

func getRelayConfig(t *testing.T, clientset kubernetes.Interface, name string) string {
	t.Helper()
	cm, err := clientset.CoreV1().ConfigMaps(internal.DefaultNamespace).Get(t.Context(), name, metav1.GetOptions{})
	require.NoError(t, err)
	relay := cm.Data["relay"]
	require.NotEmpty(t, strings.TrimSpace(relay))
	return relay
}
