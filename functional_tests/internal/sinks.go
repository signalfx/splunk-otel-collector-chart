// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

const (
	HECLogsReceiverPort    = 8090
	HECMetricsReceiverPort = 8091
	HECObjectsReceiverPort = 8092
	OTLPGRPCReceiverPort   = 4317
	OTLPHTTPReceiverPort   = 4318
	SignalFxReceiverPort   = 9943
)

func SetupHECLogsSink(t *testing.T) *consumertest.LogsSink {
	return setupHECLogsSink(t, HECLogsReceiverPort)
}

func SetupHECObjectsSink(t *testing.T) *consumertest.LogsSink {
	return setupHECLogsSink(t, HECObjectsReceiverPort)
}

func setupHECLogsSink(t *testing.T, port int) *consumertest.LogsSink {
	f := splunkhecreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*splunkhecreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", port)

	lc := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(f.Type()), cfg, lc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating logs receiver")
	t.Cleanup(func() {
		require.NoError(t, rcvr.Shutdown(t.Context()))
	})

	return lc
}

func SetupHECMetricsSink(t *testing.T) *consumertest.MetricsSink {
	// the splunkhecreceiver does poorly at receiving logs and metrics. Use separate ports for now.
	f := splunkhecreceiver.NewFactory()
	mCfg := f.CreateDefaultConfig().(*splunkhecreceiver.Config)
	mCfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", HECMetricsReceiverPort)

	mc := new(consumertest.MetricsSink)
	mrcvr, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(f.Type()), mCfg, mc)
	require.NoError(t, err)

	require.NoError(t, mrcvr.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating metrics receiver")
	t.Cleanup(func() {
		require.NoError(t, mrcvr.Shutdown(t.Context()))
	})

	return mc
}

func SetupOTLPTracesSink(t *testing.T) *consumertest.TracesSink {
	tc := new(consumertest.TracesSink)
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	cfg.GRPC = configoptional.Some(configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  fmt.Sprintf("0.0.0.0:%d", OTLPGRPCReceiverPort),
			Transport: "tcp",
		},
	})

	cfg.HTTP = configoptional.Some(otlpreceiver.HTTPConfig{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: fmt.Sprintf("0.0.0.0:%d", OTLPHTTPReceiverPort),
		},
		TracesURLPath: "/v1/traces",
	})

	rcvr, err := f.CreateTraces(t.Context(), receivertest.NewNopSettings(f.Type()), cfg, tc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating traces receiver")
	t.Cleanup(func() {
		require.NoError(t, rcvr.Shutdown(t.Context()))
	})

	return tc
}

type mockHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (h *mockHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

func SetupOTLPTracesSinkWithToken(t *testing.T, token string) *consumertest.TracesSink {
	tc := new(consumertest.TracesSink)
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.GRPC = configoptional.Some(configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  fmt.Sprintf("0.0.0.0:%d", OTLPGRPCReceiverPort),
			Transport: "tcp",
		},
	})

	baFactory := bearertokenauthextension.NewFactory()
	baCfg := baFactory.CreateDefaultConfig().(*bearertokenauthextension.Config)
	baCfg.BearerToken = configopaque.String(token)
	baCfg.Header = "X-Sf-Token"
	baCfg.Scheme = ""
	baExt, err := baFactory.Create(t.Context(), extensiontest.NewNopSettings(baFactory.Type()), baCfg)
	require.NoError(t, err)

	host := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewIDWithName("bearertokenauth", "passthroughValidation"): baExt,
		},
	}

	cfg.HTTP = configoptional.Some(otlpreceiver.HTTPConfig{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: fmt.Sprintf("0.0.0.0:%d", OTLPHTTPReceiverPort),
			Auth: configoptional.Some(confighttp.AuthConfig{
				Config: configauth.Config{
					AuthenticatorID: component.MustNewIDWithName("bearertokenauth", "passthroughValidation"),
				},
			}),
		},
		TracesURLPath: "/v2/trace/otlp",
	})

	rcvr, err := f.CreateTraces(t.Context(), receivertest.NewNopSettings(f.Type()), cfg, tc)
	require.NoError(t, err)
	require.NoError(t, rcvr.Start(t.Context(), host))

	t.Cleanup(func() {
		require.NoError(t, rcvr.Shutdown(t.Context()))
	})

	return tc
}

func SetupOTLPLogsSink(t *testing.T) *consumertest.LogsSink {
	ls := new(consumertest.LogsSink)
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.GRPC = configoptional.Some(configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  fmt.Sprintf("0.0.0.0:%d", OTLPGRPCReceiverPort),
			Transport: "tcp",
		},
	})
	cfg.HTTP = configoptional.Some(otlpreceiver.HTTPConfig{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: fmt.Sprintf("0.0.0.0:%d", OTLPHTTPReceiverPort),
		},
		LogsURLPath: "/v1/logs",
	})

	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(f.Type()), cfg, ls)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating logs receiver")
	t.Cleanup(func() {
		require.NoError(t, rcvr.Shutdown(t.Context()))
	})

	return ls
}

func SetupSignalfxReceiver(t *testing.T, port int) *consumertest.MetricsSink {
	mc := new(consumertest.MetricsSink)
	f := signalfxreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*signalfxreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", port)

	rcvr, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(f.Type()), cfg, mc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating metrics receiver")
	t.Cleanup(func() {
		require.NoError(t, rcvr.Shutdown(t.Context()))
	})

	return mc
}
