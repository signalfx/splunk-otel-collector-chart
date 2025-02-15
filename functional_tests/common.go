// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional_tests

import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hostEndpoint(t *testing.T) string {
	if host, ok := os.LookupEnv("HOST_ENDPOINT"); ok {
		return host
	}
	if runtime.GOOS == "darwin" {
		return "host.docker.internal"
	}

	client, err := docker.NewClientWithOpts(docker.FromEnv)
	require.NoError(t, err)
	client.NegotiateAPIVersion(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	network, err := client.NetworkInspect(ctx, "kind", types.NetworkInspectOptions{})
	require.NoError(t, err)
	for _, ipam := range network.IPAM.Config {
		if ipam.Gateway != "" {
			return ipam.Gateway
		}
	}
	require.Fail(t, "failed to find host endpoint")
	return ""
}

func waitForTraces(t *testing.T, entriesNum int, tc *consumertest.TracesSink) {
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(tc.AllTraces()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d traces in %d minutes", entriesNum,
		len(tc.AllTraces()), timeoutMinutes)
}

func waitForLogs(t *testing.T, entriesNum int, lc *consumertest.LogsSink) {
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(lc.AllLogs()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d logs in %d minutes", entriesNum,
		len(lc.AllLogs()), timeoutMinutes)
}

func waitForMetrics(t *testing.T, entriesNum int, mc *consumertest.MetricsSink) {
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics in %d minutes", entriesNum,
		len(mc.AllMetrics()), timeoutMinutes)
}

func checkNoEventsReceived(t *testing.T, lc *consumertest.LogsSink) {
	require.True(t, len(lc.AllLogs()) == 0,
		"received %d logs, expected 0 logs", len(lc.AllLogs()))
}

func checkNoMetricsReceived(t *testing.T, lc *consumertest.MetricsSink) {
	require.True(t, len(lc.AllMetrics()) == 0,
		"received %d metrics, expected 0 metrics", len(lc.AllMetrics()))
}

func resetMetricsSink(t *testing.T, mc *consumertest.MetricsSink) {
	mc.Reset()
	t.Logf("Metrics sink reset, current metrics: %d", len(mc.AllMetrics()))
}

func resetLogsSink(t *testing.T, lc *consumertest.LogsSink) {
	lc.Reset()
	t.Logf("Logs sink reset, current logs: %d", len(lc.AllLogs()))
}

func writeNewExpectedTracesResult(t *testing.T, file string, trace *ptrace.Traces) {
	require.NoError(t, os.MkdirAll("results", 0755))
	require.NoError(t, golden.WriteTraces(t, filepath.Join("results", filepath.Base(file)), *trace))
}

func writeNewExpectedMetricsResult(t *testing.T, file string, metric *pmetric.Metrics) {
	require.NoError(t, os.MkdirAll("results", 0755))
	require.NoError(t, golden.WriteMetrics(t, filepath.Join("results", filepath.Base(file)), *metric))
}

func setupSignalfxReceiver(t *testing.T, port int) *consumertest.MetricsSink {
	mc := new(consumertest.MetricsSink)
	f := signalfxreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*signalfxreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", port)

	rcvr, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(), cfg, mc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating metrics receiver")
	t.Cleanup(func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	})

	return mc
}
