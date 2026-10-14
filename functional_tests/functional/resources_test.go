// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	persistentVolumeTestName      = "splunk-otel-collector-functional-test-pv"
	persistentVolumeClaimTestName = "splunk-otel-collector-functional-test-pvc"
	persistentVolumeStorageClass  = "splunk-otel-collector-functional-test"
)

func deployPersistentVolumeResources(t *testing.T, client *kubernetes.Clientset) {
	storage := resource.MustParse("1Gi")
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: persistentVolumeTestName},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: storage,
			},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			StorageClassName:              persistentVolumeStorageClass,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/tmp/splunk-otel-collector-functional-test"},
			},
		},
	}
	_, err := client.CoreV1().PersistentVolumes().Create(t.Context(), pv, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping PV/PVC teardown as SKIP_TEARDOWN is set to true")
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 30*time.Second)
		defer cancel()

		if deleteErr := client.CoreV1().PersistentVolumeClaims(internal.DefaultNamespace).Delete(cleanupCtx, persistentVolumeClaimTestName, metav1.DeleteOptions{}); deleteErr != nil && !k8serrors.IsNotFound(deleteErr) {
			t.Errorf("failed to delete test PVC: %v", deleteErr)
		}
		if deleteErr := client.CoreV1().PersistentVolumes().Delete(cleanupCtx, persistentVolumeTestName, metav1.DeleteOptions{}); deleteErr != nil && !k8serrors.IsNotFound(deleteErr) {
			t.Errorf("failed to delete test PV: %v", deleteErr)
		}
	})

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      persistentVolumeClaimTestName,
			Namespace: internal.DefaultNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: stringPtr(persistentVolumeStorageClass),
			VolumeName:       persistentVolumeTestName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storage,
				},
			},
		},
	}
	_, err = client.CoreV1().PersistentVolumeClaims(internal.DefaultNamespace).Create(t.Context(), pvc, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		claim, getErr := client.CoreV1().PersistentVolumeClaims(internal.DefaultNamespace).Get(t.Context(), persistentVolumeClaimTestName, metav1.GetOptions{})
		return getErr == nil && claim.Status.Phase == corev1.ClaimBound
	}, 2*time.Minute, 2*time.Second, "PVC %q did not bind to PV %q", persistentVolumeClaimTestName, persistentVolumeTestName)
}

func stringPtr(value string) *string {
	return &value
}

// deployTestResources creates workloads consumed by the kind telemetry tests.
// Hosted clusters and kind upgrade checks only need the collector chart.
func deployTestResources(t *testing.T, client kubernetes.Interface, dynamicClient dynamic.Interface) {
	kubeTestEnv := requireEnv(t, "KUBE_TEST_ENV")
	if kubeTestEnv != kindTestKubeEnv || os.Getenv("UPGRADE_FROM_VALUES") != "" {
		return
	}
	decode := scheme.Codecs.UniversalDeserializer().Decode

	deployments := client.AppsV1().Deployments(internal.DefaultNamespace)

	deployApp := func(filePath string) {
		data, readErr := os.ReadFile(filePath)
		require.NoError(t, readErr)
		obj, _, decodeErr := decode(data, nil, nil)
		require.NoError(t, decodeErr)
		dep := obj.(*appsv1.Deployment)
		_, createErr := deployments.Create(t.Context(), dep, metav1.CreateOptions{})
		if k8serrors.IsAlreadyExists(createErr) {
			existing, getErr := deployments.Get(t.Context(), dep.Name, metav1.GetOptions{})
			require.NoError(t, getErr)
			dep.ResourceVersion = existing.ResourceVersion
			_, updateErr := deployments.Update(t.Context(), dep, metav1.UpdateOptions{})
			require.NoError(t, updateErr)
		} else {
			require.NoError(t, createErr)
		}
	}
	for _, f := range []string{
		filepath.Join(testDir, "nodejs", "deployment.yaml"),
		filepath.Join(testDir, "java", "deployment.yaml"),
		filepath.Join(testDir, "dotnet", "deployment.yaml"),
		filepath.Join(testDir, "python", "deployment.yaml"),
		filepath.Join(testDir, manifestsDir, "log_attr_test_deployment.yaml"),
	} {
		deployApp(f)
	}

	if requiresPrometheusResources(kubeTestEnv) {
		deployPrometheusTestResources(t, client, dynamicClient)
	}

	var obj k8sruntime.Object
	var groupVersionKind *schema.GroupVersionKind

	// Read jobs
	jobstream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "test_jobs.yaml"))
	require.NoError(t, err)
	for _, resourceYAML := range strings.Split(string(jobstream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		obj, groupVersionKind, err = decode(
			[]byte(resourceYAML),
			nil,
			nil)
		require.NoError(t, err)
		if groupVersionKind.Group == "" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "Namespace" {
			nm := obj.(*corev1.Namespace)
			nms := client.CoreV1().Namespaces()
			_, err = nms.Create(t.Context(), nm, metav1.CreateOptions{})
			require.NoError(t, err)
			t.Logf("Deployed namespace %s", nm.Name)
		}
	}

	waitForAllNamespacesToBeCreated(t, client)

	for _, resourceYAML := range strings.Split(string(jobstream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		obj, groupVersionKind, err = decode(
			[]byte(resourceYAML),
			nil,
			nil)
		require.NoError(t, err)

		if groupVersionKind.Group == "batch" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "Job" {
			job := obj.(*batchv1.Job)
			jobClient := client.BatchV1().Jobs(job.Namespace)
			_, err = jobClient.Create(t.Context(), job, metav1.CreateOptions{})
			require.NoError(t, err)
			t.Logf("Deployed job %s", job.Name)
		}
	}
}

func teardown(ctx context.Context, t *testing.T, testKubeConfig string) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)
	extensionsClient, err := clientset.NewForConfig(kubeConfig)
	require.NoError(t, err)
	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	require.NoError(t, err)
	waitTime := int64(0)
	deployments := client.AppsV1().Deployments(internal.DefaultNamespace)
	require.NoError(t, err)
	// Also remove fixtures left by older hosted runs that deployed kind workloads.
	_ = deployments.Delete(ctx, "nodejs-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = deployments.Delete(ctx, "java-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = deployments.Delete(ctx, "python-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = deployments.Delete(ctx, "dotnet-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
	_ = deployments.Delete(ctx, "log-attr-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})

	var groupVersionKind *schema.GroupVersionKind
	var obj k8sruntime.Object

	jobstream, err := os.ReadFile(filepath.Join(testDir, manifestsDir, "test_jobs.yaml"))
	require.NoError(t, err)
	var namespaces []*corev1.Namespace
	var jobs []*batchv1.Job
	for _, resourceYAML := range strings.Split(string(jobstream), "---") {
		if len(resourceYAML) == 0 {
			continue
		}

		obj, groupVersionKind, err = decode(
			[]byte(resourceYAML),
			nil,
			nil)
		require.NoError(t, err)
		if groupVersionKind.Group == "" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "Namespace" {
			nm := obj.(*corev1.Namespace)
			namespaces = append(namespaces, nm)
		}

		if groupVersionKind.Group == "batch" &&
			groupVersionKind.Version == "v1" &&
			groupVersionKind.Kind == "Job" {
			job := obj.(*batchv1.Job)
			jobs = append(jobs, job)
		}
	}
	for _, job := range jobs {
		jobClient := client.BatchV1().Jobs(job.Namespace)
		_ = jobClient.Delete(ctx, job.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &waitTime,
		})
	}

	if requiresPrometheusResources(os.Getenv("KUBE_TEST_ENV")) {
		teardownPrometheusResources(ctx, t, client, extensionsClient, dynamicClient)
	}

	for _, nm := range namespaces {
		nmClient := client.CoreV1().Namespaces()
		_ = nmClient.Delete(ctx, nm.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &waitTime,
		})
		require.Eventually(t, func() bool {
			_, err = client.CoreV1().Namespaces().Get(ctx, nm.Name, metav1.GetOptions{})
			t.Logf("Getting Namespace: %s, Error: %v", nm.Name, err)
			return k8serrors.IsNotFound(err)
		}, 3*time.Minute, 3*time.Second, "namespace %s not removed in time", nm.Name)
	}
	internal.ChartUninstall(t, testKubeConfig)
	if os.Getenv("KUBE_TEST_ENV") == aksTestKubeEnv {
		err = client.CoreV1().Secrets(internal.DefaultNamespace).Delete(
			ctx,
			aksWindowsValidationResourceName,
			metav1.DeleteOptions{},
		)
		require.True(t, err == nil || k8serrors.IsNotFound(err), "failed to delete AKS Windows validation secret: %v", err)
	}
}

func waitForAllNamespacesToBeCreated(t *testing.T, client kubernetes.Interface) {
	require.Eventually(t, func() bool {
		nms, err := client.CoreV1().Namespaces().List(t.Context(), metav1.ListOptions{})
		require.NoError(t, err)
		for _, d := range nms.Items {
			if d.Status.Phase != corev1.NamespaceActive {
				return false
			}
		}
		return true
	}, 5*time.Minute, 10*time.Second)
}
