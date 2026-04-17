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
	containerRecombineTestNamespace      = "container-recombine-test"
	containerRecombineValuesFile         = "container_recombine_values.yaml.tmpl"
	containerRecombineTestdataDir        = "testdata"
	containerRecombineManifestsDir       = "testdata/container_recombine_testobjects"
	containerRecombineCrioContainerDir   = "crio-container"
	containerRecombineCtrContainerDir    = "containerd-container"
	containerRecombineDockerContainerDir = "docker-container"
)

// Test_ContainerRecombine verifies that the filelog receiver's runtime-specific
// recombine operators correctly reassemble partial log entries into single records.
//
// A privileged init container writes raw log files directly to /var/log/pods/ on
// the host in cri-o, containerd, and docker formats. Each file contains a complete
// single-line entry and a sequence of partial chunks that must be stitched together.
// This exercises the crio-recombine, containerd-recombine, and docker-recombine
// stanza operators without needing a real container runtime to split lines.
//
// Env vars to control the test behavior:
// TEARDOWN_BEFORE_SETUP, SKIP_SETUP, SKIP_TEARDOWN, SKIP_TESTS, KUBECONFIG
func Test_ContainerRecombine(t *testing.T) {
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
		containerRecombineTeardown(t, k8sClient)
	}

	logsConsumer := internal.SetupHECLogsSink(t)

	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		containerRecombineDeployWorkloadAndCollector(t, testKubeConfig, clientset, k8sClient)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	t.Run("CrioSingleLineEntryPassthrough", func(t *testing.T) {
		checkSingleLineNotBatched(t, logsConsumer, containerRecombineCrioContainerDir, "CRIO_SINGLE_LINE_MARKER")
	})

	t.Run("CrioPartialEntriesRecombinedIntoOneRecord", func(t *testing.T) {
		checkPartialEntriesRecombined(t, logsConsumer, containerRecombineCrioContainerDir, "CRIO_PARTIAL_START", "CRIO_PARTIAL_END")
	})

	t.Run("ContainerdSingleLineEntryPassthrough", func(t *testing.T) {
		checkSingleLineNotBatched(t, logsConsumer, containerRecombineCtrContainerDir, "CONTAINERD_SINGLE_LINE_MARKER")
	})

	t.Run("ContainerdPartialEntriesRecombinedIntoOneRecord", func(t *testing.T) {
		checkPartialEntriesRecombined(t, logsConsumer, containerRecombineCtrContainerDir, "CONTAINERD_PARTIAL_START", "CONTAINERD_PARTIAL_END")
	})

	t.Run("DockerSingleLineEntryPassthrough", func(t *testing.T) {
		// Docker log bodies always end with \n — that is how the runtime signals a
		// complete line. We assert the entry arrived as its own record by checking
		// it was not merged with the partial entries that follow it.
		require.EventuallyWithT(t, func(tt *assert.CollectT) {
			bodies := collectBodiesFromSource(logsConsumer, containerRecombineDockerContainerDir)
			record := findBodyContaining(bodies, "DOCKER_SINGLE_LINE_MARKER")
			if assert.NotNil(tt, record, "Docker single-line entry must arrive as its own record") {
				assert.NotContains(tt, *record, "DOCKER_PARTIAL_START",
					"single-line entry must not be merged with partial entries, got: %q", *record)
			}
		}, 3*time.Minute, 5*time.Second)
	})

	t.Run("DockerPartialEntriesRecombinedIntoOneRecord", func(t *testing.T) {
		checkPartialEntriesRecombined(t, logsConsumer, containerRecombineDockerContainerDir, "DOCKER_PARTIAL_START", "DOCKER_PARTIAL_END")
	})
}

// checkSingleLineNotBatched asserts that a complete single-line entry arrives as
// an individual record without any newlines in the body.
func checkSingleLineNotBatched(t *testing.T, logsConsumer *consumertest.LogsSink, containerDir, marker string) {
	t.Helper()
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		bodies := collectBodiesFromSource(logsConsumer, containerDir)
		record := findBodyContaining(bodies, marker)
		if assert.NotNilf(tt, record, "single-line entry %q must arrive as its own record", marker) {
			assert.NotContainsf(tt, *record, "\n",
				"single-line entry %q must not be joined with other lines, got: %q", marker, *record)
		}
	}, 3*time.Minute, 5*time.Second)
}

// checkPartialEntriesRecombined asserts that partial entries are stitched into a
// single record containing all chunks.
func checkPartialEntriesRecombined(t *testing.T, logsConsumer *consumertest.LogsSink, containerDir, startMarker, endMarker string) {
	t.Helper()
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		bodies := collectBodiesFromSource(logsConsumer, containerDir)
		record := findBodyContaining(bodies, startMarker)
		if assert.NotNilf(tt, record, "partial entries must be recombined into a single record (start: %q)", startMarker) {
			assert.Containsf(tt, *record, "middle_chunk",
				"recombined record must contain the middle partial chunk, got: %q", *record)
			assert.Containsf(tt, *record, endMarker,
				"recombined record must contain the final chunk, got: %q", *record)
		}
	}, 3*time.Minute, 5*time.Second)
}

// collectBodiesFromSource returns the body string of every log record whose
// com.splunk.source attribute contains the given container directory name.
// We filter by source path rather than k8s.container.name because the log files
// are written by an init container directly to the host — there is no live
// container the k8s_attributes processor can attach a container name to.
func collectBodiesFromSource(logsConsumer *consumertest.LogsSink, containerDir string) []string {
	var bodies []string
	for _, logs := range logsConsumer.AllLogs() {
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			rl := logs.ResourceLogs().At(i)
			src, ok := rl.Resource().Attributes().Get("com.splunk.source")
			if !ok || !strings.Contains(src.AsString(), containerDir) {
				continue
			}
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				sl := rl.ScopeLogs().At(j)
				for k := 0; k < sl.LogRecords().Len(); k++ {
					bodies = append(bodies, sl.LogRecords().At(k).Body().AsString())
				}
			}
		}
	}
	return bodies
}

func containerRecombineDeployWorkloadAndCollector(t *testing.T, testKubeConfig string, clientset *kubernetes.Clientset, k8sClient *k8stest.K8sClient) {
	t.Helper()

	valuesFile, err := filepath.Abs(filepath.Join(containerRecombineTestdataDir, containerRecombineValuesFile))
	require.NoError(t, err)

	hostEp := internal.HostEndpoint(t)
	require.NotEmpty(t, hostEp, "host endpoint not found")

	replacements := map[string]any{
		"LogURL": internal.HostPortHTTP(hostEp, internal.HECLogsReceiverPort),
	}
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 0, internal.GetDefaultChartOptions())

	internal.CheckPodsReady(t, clientset, internal.DefaultNamespace, "component=otel-collector-agent", 3*time.Minute, 5*time.Second)

	internal.CreateNamespace(t, clientset, containerRecombineTestNamespace)
	internal.WaitForDefaultServiceAccount(t, clientset, containerRecombineTestNamespace)

	createdObjs, err := k8stest.CreateObjects(k8sClient, containerRecombineManifestsDir)
	require.NoError(t, err)
	require.NotEmpty(t, createdObjs)

	// Wait for the init container to finish writing and the sleep container to be running.
	internal.CheckPodsReady(t, clientset, containerRecombineTestNamespace, "app=container-recombine-writer", 2*time.Minute, 0)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		containerRecombineTeardown(t, k8sClient)
	})
}

func containerRecombineTeardown(t *testing.T, k8sClient *k8stest.K8sClient) {
	t.Helper()
	testKubeConfig := os.Getenv("KUBECONFIG")
	internal.ChartUninstall(t, testKubeConfig)

	internal.DeleteObject(t, k8sClient, `
apiVersion: v1
kind: Namespace
metadata:
  name: container-recombine-test
`)
}
