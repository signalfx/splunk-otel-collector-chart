// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package secureapp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeardownContinuesAfterWorkloadDeletionFailure(t *testing.T) {
	const childConfigEnv = "SECUREAPP_CLEANUP_TEST_KUBECONFIG"
	if kubeconfig := os.Getenv(childConfigEnv); kubeconfig != "" {
		teardown(t.Context(), t, kubeconfig)
		return
	}

	var workloadDeleteAttempted, chartCleanupAttempted atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "DELETE /apis/apps/v1/namespaces/default/deployments/java-test":
			workloadDeleteAttempted.Store(true)
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprint(w, `{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"Forbidden","message":"workload deletion denied","code":403}`)
		case "GET /version":
			_, _ = fmt.Fprint(w, `{"major":"1","minor":"37","gitVersion":"v1.37.1"}`)
		case "GET /api/v1/namespaces/default/secrets":
			// Helm lists releases before removing chart resources.
			assert.True(t, workloadDeleteAttempted.Load(), "chart cleanup must follow workload deletion")
			chartCleanupAttempted.Store(true)
			_, _ = fmt.Fprint(w, `{"apiVersion":"v1","kind":"SecretList","items":[]}`)
		case "GET /apis/apiextensions.k8s.io/v1/customresourcedefinitions":
			_, _ = fmt.Fprint(w, `{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinitionList","items":[]}`)
		case "DELETE /api/v1/namespaces/default/secrets/sock-operator-controller-manager-service-cert":
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"NotFound","code":404}`)
		default:
			t.Errorf("unexpected cleanup request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	kubeconfig := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, os.WriteFile(kubeconfig, []byte(fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: %s
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user: {}
`, server.URL)), 0o600))

	// Run the fatal assertion in a child process so this test can verify both
	// the reported failure and the subsequent chart cleanup.
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	t.Setenv("HELM_DRIVER", "secret")
	binary, executableErr := os.Executable()
	require.NoError(t, executableErr)
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestTeardownContinuesAfterWorkloadDeletionFailure$")
	cmd.Env = append(os.Environ(), childConfigEnv+"="+kubeconfig)
	output, err := cmd.CombinedOutput()
	require.Error(t, err, "the failed workload deletion must fail the test")
	require.NoError(t, ctx.Err(), "cleanup subprocess timed out: %s", output)
	assert.Contains(t, string(output), "failed to delete java-test deployment")
	assert.Contains(t, string(output), "No Helm releases found for uninstall.")
	assert.True(t, workloadDeleteAttempted.Load(), "workload deletion must be attempted")
	assert.True(t, chartCleanupAttempted.Load(), "chart cleanup must run after workload deletion fails")
}
