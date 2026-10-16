// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

// controlPlaneReceiversForEnv returns the receiver names that should be discovered and
// scraping for the given cluster environment.
func controlPlaneReceiversForEnv(kubeTestEnv string) []string {
	switch kubeTestEnv {
	case kindTestKubeEnv:
		return []string{
			"prometheus/etcd",
			"prometheus/kube-controller-manager",
			"prometheus/kubernetes-scheduler",
		}
	case rosaTestKubeEnv:
		return []string{
			"prometheus/etcd",
			"prometheus/kube-controller-manager",
			"prometheus/kubernetes-apiserver",
			"prometheus/kubernetes-scheduler",
		}
	case gceTestKubeEnv:
		return []string{
			"prometheus/etcd",
			"prometheus/kube-controller-manager",
			"prometheus/kubernetes-apiserver",
			"prometheus/kubernetes-scheduler",
		}
	default:
		return nil
	}
}

// runControlPlaneReceiverChecks verifies that control-plane receivers are discovered
// and scraping without errors.
func runControlPlaneReceiverChecks(t *testing.T, client *kubernetes.Clientset, receivers []string) {
	cpAgentPods := internal.GetControlPlaneAgentPods(t, client, internal.DefaultNamespace, internal.AgentLabelSelector)
	require.NotEmpty(t, cpAgentPods, "no agent pods found on control-plane nodes")

	internal.WaitForScrapeInterval(t, 3*time.Minute)

	for _, receiver := range receivers {
		t.Run("receiver started: "+receiver, func(t *testing.T) {
			internal.CheckReceiverStarted(t, client, internal.DefaultNamespace, cpAgentPods, receiver)
		})
		t.Run("no errors: "+receiver, func(t *testing.T) {
			internal.CheckComponentHealthForPods(t, client, internal.DefaultNamespace, cpAgentPods, receiver, 10000)
		})
	}
}
