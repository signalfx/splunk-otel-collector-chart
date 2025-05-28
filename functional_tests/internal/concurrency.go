// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

const (
	leaseName      = "functional-test-lock"
	leaseNamespace = "default"

	leaseDuration = 15 * time.Second
	renewDeadline = 10 * time.Second
	retryPeriod   = 2 * time.Second
)

// AcquireLeaseForTest acquires (and holds) a cluster-wide lease for the duration of the
// current test. Once acquired, it registers a t.Cleanup function that will release the
// lease after the test completes (including any other Cleanup functions).
func AcquireLeaseForTest(t *testing.T, testKubeConfig string) {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	// Generate an identity for the test execution
	hostname, err := os.Hostname()
	_, filename, _, _ := runtime.Caller(2)
	require.NoError(t, err)
	holderIdentity := fmt.Sprintf("%s:%s:%s:%d", hostname, filename, t.Name(), time.Now().UnixNano())
	// holderIdentity := fmt.Sprintf("%s:%s:%s", hostname, filename, t.Name())

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: leaseNamespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: holderIdentity,
		},
	}

	becameLeader := make(chan struct{})
	lostLeaderCh := make(chan struct{})

	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: leaseDuration,
		RenewDeadline: renewDeadline,
		RetryPeriod:   retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnNewLeader: func(identity string) {
				t.Logf("The lease is currently held by another test %s. Waiting...", identity)
			},
			OnStartedLeading: func(_ context.Context) {
				// Signal that we've acquired the lease
				close(becameLeader)
			},
			OnStoppedLeading: func() {
				t.Logf("Lost or released the lease")
				close(lostLeaderCh)
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create leader elector: %v", err)
	}

	// Run the leader election in a goroutine, so we can block until acquiring the lease
	ctx, cancel := context.WithCancel(t.Context())
	go elector.Run(ctx)

	// Wait until we become leader OR the test context ends
	select {
	case <-becameLeader:
		t.Logf("Acquired the lease as %s", holderIdentity)
	case <-t.Context().Done():
		// If the test was canceled before we could acquire the lease
		cancel()
		t.Fatalf("Test was canceled before acquiring lease: %v", t.Context().Err())
	}

	// We hold the lease. Register a cleanup to release it when the test is fully done.
	t.Cleanup(func() {
		t.Log("Releasing cluster-wide lock...")
		// Stop renewing the lease:
		cancel()
		// Wait until the OnStoppedLeading callback fires, ensuring the lock is fully released.
		<-lostLeaderCh
		t.Log("Cluster-wide lock released.")
	})
}
