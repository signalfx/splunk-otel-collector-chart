// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func runLocalClusterUpgradeTests(t *testing.T) {
	t.Run("test target allocator resources cleaned up and installed", testTargetAllocatorUpgrade)
	t.Run("test component health", testLocalClusterComponentHealth)
}

func testTargetAllocatorUpgrade(t *testing.T) {
	testKubeConfig := requireEnv(t, "KUBECONFIG")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	require.NoError(t, testTargetAllocatorClusterRoleUpgrade(t, client))
	require.NoError(t, testTargetAllocatorClusterRoleBindingUpgrade(t, client))
	require.NoError(t, testTargetAllocatorConfigMapUpgrade(t, client))
	require.NoError(t, testTargetAllocatorDeploymentUpgrade(t, client))
	require.NoError(t, testTargetAllocatorServiceUpgrade(t, client))
	require.NoError(t, testTargetAllocatorServiceAccountUpgrade(t, client))
}

func testTargetAllocatorClusterRoleUpgrade(t *testing.T, client *kubernetes.Clientset) error {
	allClusterRoles, err := client.RbacV1().ClusterRoles().List(t.Context(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	return validateTargetAllocatorResourceUpgrade(allClusterRoles.Kind, allClusterRoles.Items)
}

func testTargetAllocatorClusterRoleBindingUpgrade(t *testing.T, client *kubernetes.Clientset) error {
	allClusterRoleBindings, err := client.RbacV1().ClusterRoleBindings().List(t.Context(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	return validateTargetAllocatorResourceUpgrade(allClusterRoleBindings.Kind, allClusterRoleBindings.Items)
}

func testTargetAllocatorConfigMapUpgrade(t *testing.T, client *kubernetes.Clientset) error {
	allConfigMaps, err := client.CoreV1().ConfigMaps(metav1.NamespaceAll).List(t.Context(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	return validateTargetAllocatorResourceUpgrade(allConfigMaps.Kind, allConfigMaps.Items)
}

func testTargetAllocatorDeploymentUpgrade(t *testing.T, client *kubernetes.Clientset) error {
	allDeployments, err := client.AppsV1().Deployments(metav1.NamespaceAll).List(t.Context(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	return validateTargetAllocatorResourceUpgrade(allDeployments.Kind, allDeployments.Items)
}

func testTargetAllocatorServiceUpgrade(t *testing.T, client *kubernetes.Clientset) error {
	allServices, err := client.CoreV1().Services(metav1.NamespaceAll).List(t.Context(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	return validateTargetAllocatorResourceUpgrade(allServices.Kind, allServices.Items)
}

func testTargetAllocatorServiceAccountUpgrade(t *testing.T, client *kubernetes.Clientset) error {
	allServiceAccounts, err := client.CoreV1().ServiceAccounts(metav1.NamespaceAll).List(t.Context(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	return validateTargetAllocatorResourceUpgrade(allServiceAccounts.Kind, allServiceAccounts.Items)
}

type namedK8sResource interface {
	GetName() string
}

func validateTargetAllocatorResourceUpgrade[T any](resourceKind string, resources []T) error {
	if err := validateOldTargetAllocatorResourceDoesNotExist(resourceKind, resources); err != nil {
		return err
	}

	return validateNewTargetAllocatorResourceExists(resourceKind, resources)
}

func validateOldTargetAllocatorResourceDoesNotExist[T any](resourceKind string, resources []T) error {
	for i := range resources {
		resource, ok := any(&resources[i]).(namedK8sResource)
		if !ok {
			return fmt.Errorf("expected %s resource to expose a Kubernetes name", resourceKind)
		}
		resourceName := resource.GetName()
		if strings.Contains(resourceName, splunkOtelCollectorTAResourceName) {
			return fmt.Errorf("expected old %s resource %q to no longer exist", resourceKind, resourceName)
		}
	}

	return nil
}

func validateNewTargetAllocatorResourceExists[T any](resourceKind string, resources []T) error {
	for i := range resources {
		resource, ok := any(&resources[i]).(namedK8sResource)
		if !ok {
			return fmt.Errorf("expected %s resource to expose a Kubernetes name", resourceKind)
		}
		resourceName := resource.GetName()
		if strings.Contains(resourceName, taResourceName) {
			return nil
		}
	}

	return fmt.Errorf("expected at least one new %s resource with %q in the name", resourceKind, taResourceName)
}
