// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

func TestDeployTestResourcesHostedClusters(t *testing.T) {
	for _, environment := range []string{
		eksTestKubeEnv, eksAutoModeTestKubeEnv, eksFargateTestKubeEnv,
		gkeTestKubeEnv, autopilotTestKubeEnv, aksTestKubeEnv, rosaTestKubeEnv, gceTestKubeEnv,
	} {
		t.Run(environment, func(t *testing.T) {
			for _, skipTests := range []bool{false, true} {
				t.Run("skip-tests="+strconv.FormatBool(skipTests), func(t *testing.T) {
					t.Setenv("KUBE_TEST_ENV", environment)
					t.Setenv("UPGRADE_FROM_VALUES", "")
					t.Setenv("SKIP_TESTS", strconv.FormatBool(skipTests))
					client := fake.NewClientset()
					dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

					deployTestResources(t, client, dynamicClient)

					assert.Empty(t, client.Actions(), "hosted tests must not deploy kind workloads")
					assert.Empty(t, dynamicClient.Actions(), "hosted tests must not create Prometheus fixtures")
				})
			}
		})
	}
}

func TestDeployTestResourcesKindUpgrade(t *testing.T) {
	t.Setenv("KUBE_TEST_ENV", kindTestKubeEnv)
	t.Setenv("UPGRADE_FROM_VALUES", "upgrade_from_previous_release_values.yaml")
	client := fake.NewClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

	deployTestResources(t, client, dynamicClient)

	assert.Empty(t, client.Actions(), "upgrade checks must not deploy unused workloads")
	assert.Empty(t, dynamicClient.Actions(), "upgrade checks must not create Prometheus fixtures")
}

func TestDeployTestResourcesKindFixtures(t *testing.T) {
	for _, testCase := range []struct {
		name               string
		skipTests          bool
		existingDeployment bool
		updateConflict     bool
	}{
		{name: "fresh cluster"},
		{name: "setup only", skipTests: true},
		{name: "existing deployment", existingDeployment: true},
		{name: "concurrent deployment update", existingDeployment: true, updateConflict: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("KUBE_TEST_ENV", kindTestKubeEnv)
			t.Setenv("UPGRADE_FROM_VALUES", "")
			t.Setenv("SKIP_TESTS", strconv.FormatBool(testCase.skipTests))
			client := fake.NewClientset()
			client.PrependReactor("create", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
				// A real namespace controller activates newly created namespaces.
				action.(k8stesting.CreateAction).GetObject().(*corev1.Namespace).Status.Phase = corev1.NamespaceActive
				return false, nil, nil
			})
			updates := 0
			updateAttempts := 0
			if testCase.existingDeployment {
				deployments := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
				require.NoError(t, client.Tracker().Create(deployments, &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: "nodejs-test", Namespace: internal.DefaultNamespace, ResourceVersion: "7"},
				}, internal.DefaultNamespace))
				client.PrependReactor("update", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
					deployment := action.(k8stesting.UpdateAction).GetObject().(*appsv1.Deployment)
					updateAttempts++
					current, getErr := client.Tracker().Get(deployments, internal.DefaultNamespace, deployment.Name)
					if getErr != nil {
						return true, nil, getErr
					}
					existing := current.(*appsv1.Deployment)
					if deployment.ResourceVersion != existing.ResourceVersion {
						return true, nil, k8serrors.NewConflict(deployments.GroupResource(), deployment.Name,
							errors.New("resourceVersion must match the existing deployment"))
					}
					if testCase.updateConflict && updateAttempts == 1 {
						// Simulate a controller updating status between the GET and UPDATE.
						existing.ResourceVersion = "8"
						if updateErr := client.Tracker().Update(deployments, existing, internal.DefaultNamespace); updateErr != nil {
							return true, nil, updateErr
						}
						return true, nil, k8serrors.NewConflict(deployments.GroupResource(), deployment.Name,
							errors.New("deployment changed after GET"))
					}
					updates++
					return false, nil, nil
				})
			}
			dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
				map[schema.GroupVersionResource]string{
					{Group: "monitoring.coreos.com", Version: "v1", Resource: "podmonitors"}:     "PodMonitorList",
					{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"}: "ServiceMonitorList",
				})

			deployTestResources(t, client, dynamicClient)

			deployments, err := client.AppsV1().Deployments(internal.DefaultNamespace).List(t.Context(), metav1.ListOptions{})
			require.NoError(t, err)
			images := make(map[string]string, len(deployments.Items))
			for _, deployment := range deployments.Items {
				require.Len(t, deployment.Spec.Template.Spec.Containers, 1)
				images[deployment.Name] = deployment.Spec.Template.Spec.Containers[0].Image
			}
			assert.Len(t, images, 6)
			for name, image := range map[string]string{
				"nodejs-test": "nodejs_test:latest",
				"java-test":   "java_test:latest",
				"dotnet-test": "dotnet_test:latest",
				"python-test": "python_test:latest",
			} {
				assert.Equal(t, image, images[name], "kind must retain its locally loaded test app")
			}
			assert.Contains(t, images, "log-attr-test")
			assert.Contains(t, images, "prometheus-annotation-test")
			if testCase.existingDeployment {
				assert.Equal(t, 1, updates, "existing workload must be updated with its current resourceVersion")
				if testCase.updateConflict {
					assert.Equal(t, 2, updateAttempts, "a conflict must trigger a fresh GET and another UPDATE")
				} else {
					assert.Equal(t, 1, updateAttempts)
				}
			}
			namespaces, err := client.CoreV1().Namespaces().List(t.Context(), metav1.ListOptions{})
			require.NoError(t, err)
			var namespaceNames []string
			for _, namespace := range namespaces.Items {
				namespaceNames = append(namespaceNames, namespace.Name)
			}
			assert.ElementsMatch(t, []string{"ns-w-index", "ns-wo-index", "ns-w-exclude"}, namespaceNames)
			jobs, err := client.BatchV1().Jobs(metav1.NamespaceAll).List(t.Context(), metav1.ListOptions{})
			require.NoError(t, err)
			assert.Len(t, jobs.Items, 6)
			_, err = client.CoreV1().ConfigMaps(internal.DefaultNamespace).Get(t.Context(), "prometheus-test-metrics", metav1.GetOptions{})
			assert.NoError(t, err)
			_, err = client.CoreV1().Services(internal.DefaultNamespace).Get(t.Context(), "prometheus-annotation-service", metav1.GetOptions{})
			assert.NoError(t, err)
			for _, resource := range prometheusCRDResources {
				monitors, listErr := dynamicClient.Resource(resource).Namespace(internal.DefaultNamespace).List(t.Context(), metav1.ListOptions{})
				require.NoError(t, listErr)
				assert.Len(t, monitors.Items, 1, "missing %s fixture", resource.Resource)
			}
		})
	}
}
