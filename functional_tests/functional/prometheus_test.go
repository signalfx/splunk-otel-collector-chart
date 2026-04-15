// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	corev1 "k8s.io/api/core/v1"
	appextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

func requiresPrometheusCRD(kubeTestEnv string) bool {
	return kubeTestEnv == kindTestKubeEnv
}

func deployPrometheusResources(t *testing.T, extensionsClient *clientset.Clientset, dynamicClient dynamic.Interface) {
	t.Log("Deploying Prometheus Operator CRDs (re-generate with: make update-prometheus-crds)")
	decode := scheme.Codecs.UniversalDeserializer().Decode

	// load up Prometheus PodMonitor and ServiceMonitor CRDs:
	stream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "prometheus_operator_crds.yaml"))
	require.NoError(t, err)
	sch := k8sruntime.NewScheme()

	var obj k8sruntime.Object
	var groupVersionKind *schema.GroupVersionKind
	for _, resourceYAML := range strings.Split(string(stream), "---") {
		if len(strings.TrimSpace(resourceYAML)) == 0 {
			continue
		}

		obj, groupVersionKind, err = decode(
			[]byte(resourceYAML),
			nil,
			nil)
		require.NoError(t, err)
		if groupVersionKind.Group == "apiextensions.k8s.io" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "CustomResourceDefinition" {
			crd := obj.(*appextensionsv1.CustomResourceDefinition)
			apiExtensions := extensionsClient.ApiextensionsV1().CustomResourceDefinitions()

			existing, getErr := apiExtensions.Get(t.Context(), crd.Name, metav1.GetOptions{})
			if getErr == nil && existing.DeletionTimestamp != nil {
				t.Logf("CRD %s is terminating, waiting for removal...", crd.Name)
				require.EventuallyWithT(t, func(tt *assert.CollectT) {
					_, e := apiExtensions.Get(t.Context(), crd.Name, metav1.GetOptions{})
					assert.True(tt, k8serrors.IsNotFound(e), "CRD %s still exists", crd.Name)
				}, 3*time.Minute, 3*time.Second, "CRD %s stuck in Terminating", crd.Name)
			}

			crdName := crd.Name
			crdSpec := crd.Spec
			created, createErr := apiExtensions.Create(t.Context(), crd, metav1.CreateOptions{})
			if createErr != nil {
				if k8serrors.IsAlreadyExists(createErr) {
					t.Logf("CRD %s already exists, skipping creation", crdName)
				} else {
					require.NoError(t, createErr)
				}
			} else {
				t.Logf("Deployed CRD %s", created.Name)
			}

			require.EventuallyWithT(t, func(tt *assert.CollectT) {
				latest, latestErr := apiExtensions.Get(t.Context(), crdName, metav1.GetOptions{})
				assert.NoError(tt, latestErr)
				established := false
				for _, cond := range latest.Status.Conditions {
					if cond.Type == appextensionsv1.Established && cond.Status == appextensionsv1.ConditionTrue {
						established = true
					}
				}
				assert.True(tt, established)
			}, 3*time.Minute, 3*time.Second, "CRD %s not established", crdName)

			for _, version := range crdSpec.Versions {
				sch.AddKnownTypeWithName(
					schema.GroupVersionKind{
						Group:   crdSpec.Group,
						Version: version.Name,
						Kind:    crdSpec.Names.Kind,
					},
					&unstructured.Unstructured{},
				)
			}
		}
	}

	codecs := serializer.NewCodecFactory(sch)
	crdDecode := codecs.UniversalDeserializer().Decode

	// Prometheus pod monitor
	stream, err = os.ReadFile(filepath.Join(testDir, manifestsDir, "pod_monitor.yaml"))
	require.NoError(t, err)
	podMonitor, _, err := crdDecode(stream, nil, nil)
	require.NoError(t, err)
	g := schema.GroupVersionResource{
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "podmonitors",
	}
	_, crErr := dynamicClient.Resource(g).Namespace(internal.DefaultNamespace).Create(t.Context(),
		podMonitor.(*unstructured.Unstructured), metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(crErr) {
		_, crErr = dynamicClient.Resource(g).Namespace(internal.DefaultNamespace).Update(t.Context(),
			podMonitor.(*unstructured.Unstructured), metav1.UpdateOptions{})
	}
	require.NoError(t, crErr)

	// Prometheus service monitor
	stream, err = os.ReadFile(filepath.Join(testDir, manifestsDir, "service_monitor.yaml"))
	require.NoError(t, err)
	serviceMonitor, _, err := crdDecode(stream, nil, nil)
	require.NoError(t, err)
	g = schema.GroupVersionResource{
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "servicemonitors",
	}
	_, crErr = dynamicClient.Resource(g).Namespace(internal.DefaultNamespace).Create(t.Context(),
		serviceMonitor.(*unstructured.Unstructured), metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(crErr) {
		_, crErr = dynamicClient.Resource(g).Namespace(internal.DefaultNamespace).Update(t.Context(),
			serviceMonitor.(*unstructured.Unstructured), metav1.UpdateOptions{})
	}
	require.NoError(t, crErr)
}

// prometheusCRDResources are the GVRs for PodMonitor and ServiceMonitor CRs
// managed by the test suite via deployPrometheusResources.
var prometheusCRDResources = []schema.GroupVersionResource{
	{Group: "monitoring.coreos.com", Version: "v1", Resource: "podmonitors"},
	{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"},
}

func teardownPrometheusResources(ctx context.Context, t *testing.T, extensionsClient *clientset.Clientset, dynamicClient dynamic.Interface) {
	// 1. Delete CRs first and wait for removal so finalizers don't block CRD deletion.
	for _, gvr := range prometheusCRDResources {
		list, listErr := dynamicClient.Resource(gvr).Namespace(internal.DefaultNamespace).List(ctx, metav1.ListOptions{})
		if listErr != nil {
			t.Logf("Failed to list %s CRs: %v", gvr.Resource, listErr)
			continue
		}
		for i := range list.Items {
			cr := &list.Items[i]
			delErr := dynamicClient.Resource(gvr).Namespace(cr.GetNamespace()).Delete(ctx, cr.GetName(), metav1.DeleteOptions{})
			if delErr != nil && !k8serrors.IsNotFound(delErr) {
				t.Logf("Failed to delete %s/%s: %v", gvr.Resource, cr.GetName(), delErr)
			}
		}
		assert.Eventually(t, func() bool {
			remaining, e := dynamicClient.Resource(gvr).Namespace(internal.DefaultNamespace).List(ctx, metav1.ListOptions{})
			return e != nil || len(remaining.Items) == 0
		}, 1*time.Minute, 2*time.Second, "%s CRs not fully removed", gvr.Resource)
	}

	// 2. Delete the CRDs themselves.
	decode := scheme.Codecs.UniversalDeserializer().Decode
	crdstream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "prometheus_operator_crds.yaml"))
	require.NoError(t, err)

	apiExtensions := extensionsClient.ApiextensionsV1().CustomResourceDefinitions()
	var crdNames []string
	for _, resourceYAML := range strings.Split(string(crdstream), "---") {
		if len(strings.TrimSpace(resourceYAML)) == 0 {
			continue
		}
		obj, gvk, decErr := decode([]byte(resourceYAML), nil, nil)
		require.NoError(t, decErr)
		if gvk.Group == "apiextensions.k8s.io" && gvk.Version == "v1" && gvk.Kind == "CustomResourceDefinition" {
			crd := obj.(*appextensionsv1.CustomResourceDefinition)
			delErr := apiExtensions.Delete(ctx, crd.Name, metav1.DeleteOptions{})
			if delErr != nil && !k8serrors.IsNotFound(delErr) {
				t.Logf("Failed to delete CRD %s: %v", crd.Name, delErr)
			} else {
				crdNames = append(crdNames, crd.Name)
			}
		}
	}

	// 3. Wait for CRDs to be fully removed.
	for _, name := range crdNames {
		require.Eventually(t, func() bool {
			_, getErr := apiExtensions.Get(ctx, name, metav1.GetOptions{})
			return k8serrors.IsNotFound(getErr)
		}, 3*time.Minute, 3*time.Second, "CRD %s was not removed in time", name)
		t.Logf("CRD %s fully removed", name)
	}
}

func testTargetAllocator(t *testing.T) {
	if !requiresPrometheusCRD(os.Getenv("KUBE_TEST_ENV")) {
		t.Fatalf("Required Prometheus CRDs are not installed")
	}

	testKubeConfig := requireEnv(t, "KUBECONFIG")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	// check target allocator logs
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var taPodList *corev1.PodList
		taPodList, err = internal.GetPods(t, client, internal.DefaultNamespace, internal.TargetAllocatorLabelSelector)
		assert.NoError(c, err)
		containsReadyTAPod := false

		for _, pod := range taPodList.Items {
			if pod.Status.Phase != "Running" {
				t.Logf("Skipping pod %s in phase %s", pod.Name, pod.Status.Phase)
				continue
			}
			containsReadyTAPod = true
			var podLogs string
			podLogs, err = internal.GetPodLogs(t, client, internal.DefaultNamespace, pod.Name, internal.TargetAllocatorContainerName, 100)
			assert.NoError(c, err)
			assert.Contains(c, podLogs, "Service Discovery watch event received", "Target allocator pod logs failed to successfully discover targets. Received logs: %v", podLogs)
		}
		assert.True(c, containsReadyTAPod, "No target allocator pod found ready")
	}, 3*time.Minute, 3*time.Second, "Failed to find required target allocator pod logs")

	// check agent logs
	serviceMonitorRegex := regexp.MustCompile(`Scrape job added.*"otelcol\.component\.id": "prometheus/ta.*"jobName": "serviceMonitor/default/prometheus-service-monitor/0"`)
	podMonitorRegex := regexp.MustCompile(`Scrape job added.*"otelcol\.component\.id": "prometheus/ta.*"jobName": "podMonitor/default/pod-monitor/0"`)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var agentPodList *corev1.PodList
		agentPodList, err = internal.GetPods(t, client, internal.DefaultNamespace, internal.AgentLabelSelector)
		assert.NoError(c, err)
		containsReadyAgentPod := false
		var combinedPodLogs strings.Builder

		for i, pod := range agentPodList.Items {
			if pod.Status.Phase != "Running" {
				t.Logf("Skipping pod %s in phase %s", pod.Name, pod.Status.Phase)
				continue
			}
			containsReadyAgentPod = true
			var podLogs string
			podLogs, err = internal.GetPodLogs(t, client, internal.DefaultNamespace, pod.Name, internal.CollectorContainerName, 500)
			assert.NoError(c, err)
			assert.Contains(c, podLogs, "Starting target allocator discovery", "Collector failed to start target allocator discovery. Received logs: %v", podLogs)

			if i > 0 {
				combinedPodLogs.WriteString("\n")
			}
			combinedPodLogs.WriteString(fmt.Sprintf("%v\n%v", pod.Name, podLogs))
		}
		assert.True(c, containsReadyAgentPod, "No OTel Collector agent pod found ready")
		// NOTE: The target allocator distributes scrape jobs across agents when there are more than one.
		// Compile all logs from agents first, then ensure that altogether they have the required logs.
		assert.Regexp(c, serviceMonitorRegex, combinedPodLogs.String(), "Collector failed to start scrape job for serviceMonitor. Received logs: %v", combinedPodLogs.String())
		assert.Regexp(c, podMonitorRegex, combinedPodLogs.String(), "Collector failed to start scrape job for podMonitor. Received logs: %v", combinedPodLogs.String())
	}, 3*time.Minute, 3*time.Second, "Failed to find required agent pod logs")
}

func testPrometheusAnnotationMetrics(t *testing.T) {
	agentMetricsConsumer := globalSinks.agentMetricsConsumer

	// metrics from the local prometheus_test_app.
	metricNames := []string{
		"test_requests_total",
		"test_connections_active",
		"test_uptime_seconds_total",
	}
	// The "pod" and "service" labels are Prometheus target labels added by
	// k8s SD relabeling in the TA-generated scrape configs.
	t.Logf("Checking via prometheus.io/scrape")
	checkMetrics(t, agentMetricsConsumer, metricNames, "annotation", func(_ pcommon.Map, metric pmetric.Metric) bool {
		return !metricDataPointsHaveKey(metric, "pod") && !metricDataPointsHaveKey(metric, "service")
	})
	t.Logf("Checking via pod monitor")
	checkMetrics(t, agentMetricsConsumer, metricNames, "podMonitor", func(_ pcommon.Map, metric pmetric.Metric) bool {
		return metricDataPointsHaveKey(metric, "pod") && !metricDataPointsHaveKey(metric, "service")
	})
	t.Logf("Checking via service monitor")
	checkMetrics(t, agentMetricsConsumer, metricNames, "serviceMonitor", func(_ pcommon.Map, metric pmetric.Metric) bool {
		return metricDataPointsHaveKey(metric, "pod") && metricDataPointsHaveKey(metric, "service")
	})
}
