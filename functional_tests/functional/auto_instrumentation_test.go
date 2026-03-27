// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

func maskScopeVersion(traces ptrace.Traces) {
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			ss.Scope().SetVersion("")
		}
	}
}

func maskSpanParentID(traces ptrace.Traces) {
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				span.SetParentSpanID(pcommon.NewSpanIDEmpty())
			}
		}
	}
}

func testNodeJSTraces(t *testing.T) {
	tracesConsumer := globalSinks.tracesConsumer

	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join(testDir, expectedValuesDir, "expected_nodejs_traces.yaml")
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)
	internal.ClearTraceSchemaURLs(expectedTraces)

	internal.WaitForTraces(t, 10, tracesConsumer)

	var selectedTrace *ptrace.Traces

	require.Eventually(t, func() bool {
		for i := len(tracesConsumer.AllTraces()) - 1; i > 0; i-- {
			trace := tracesConsumer.AllTraces()[i]
			if val, ok := trace.ResourceSpans().At(0).Resource().Attributes().Get("telemetry.sdk.language"); ok && strings.Contains(val.Str(), "nodejs") {
				if expectedTraces.SpanCount() == trace.SpanCount() && expectedTraces.ResourceSpans().Len() == trace.ResourceSpans().Len() {
					selectedTrace = &trace
					break
				}
			}
		}
		return selectedTrace != nil
	}, 3*time.Minute, 5*time.Second)
	require.NotNil(t, selectedTrace)

	maskScopeVersion(*selectedTrace)
	maskScopeVersion(expectedTraces)
	internal.ClearTraceSchemaURLs(*selectedTrace)

	internal.MaybeWriteUpdateExpectedTracesResults(t, expectedTracesFile, selectedTrace)
	err = ptracetest.CompareTraces(expectedTraces, *selectedTrace,
		ptracetest.IgnoreResourceAttributeValue("container.id"),
		ptracetest.IgnoreResourceAttributeValue("host.arch"),
		ptracetest.IgnoreResourceAttributeValue("k8s.deployment.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.replicaset.name"),
		ptracetest.IgnoreResourceAttributeValue("os.version"),
		ptracetest.IgnoreResourceAttributeValue("process.pid"),
		ptracetest.IgnoreResourceAttributeValue("splunk.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("process.runtime.version"),
		ptracetest.IgnoreResourceAttributeValue("process.command"),
		ptracetest.IgnoreResourceAttributeValue("process.command_args"),
		ptracetest.IgnoreResourceAttributeValue("process.executable.path"),
		ptracetest.IgnoreResourceAttributeValue("process.owner"),
		ptracetest.IgnoreResourceAttributeValue("process.runtime.description"),
		ptracetest.IgnoreResourceAttributeValue("splunk.zc.method"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.sdk.version"),
		ptracetest.IgnoreResourceAttributeValue("service.instance.id"),
		ptracetest.IgnoreSpanAttributeValue("http.user_agent"),
		ptracetest.IgnoreSpanAttributeValue("net.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("network.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("os.version"),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
		ptracetest.IgnoreScopeSpanInstrumentationScopeVersion(),
	)
	require.NoError(t, err)
}

func testPythonTraces(t *testing.T) {
	tracesConsumer := globalSinks.tracesConsumer

	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join(testDir, expectedValuesDir, "expected_python_traces.yaml")
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)
	internal.ClearTraceSchemaURLs(expectedTraces)

	internal.WaitForTraces(t, 10, tracesConsumer)

	var selectedTrace *ptrace.Traces

	read := 0
	require.Eventually(t, func() bool {
		for i := len(tracesConsumer.AllTraces()) - 1; i > read; i-- {
			trace := tracesConsumer.AllTraces()[i]
			if val, ok := trace.ResourceSpans().At(0).Resource().Attributes().Get("telemetry.sdk.language"); ok && strings.Contains(val.Str(), "python") {
				if expectedTraces.SpanCount() == trace.SpanCount() && expectedTraces.ResourceSpans().Len() == trace.ResourceSpans().Len() {
					selectedTrace = &trace
					break
				}
			}
		}
		read = len(tracesConsumer.AllTraces()) - 1
		return selectedTrace != nil
	}, 1*time.Minute, 5*time.Second)
	require.NotNil(t, selectedTrace)

	maskScopeVersion(*selectedTrace)
	maskScopeVersion(expectedTraces)
	internal.ClearTraceSchemaURLs(*selectedTrace)

	internal.MaybeWriteUpdateExpectedTracesResults(t, expectedTracesFile, selectedTrace)
	err = ptracetest.CompareTraces(expectedTraces, *selectedTrace,
		ptracetest.IgnoreResourceAttributeValue("container.id"),
		ptracetest.IgnoreResourceAttributeValue("host.arch"),
		ptracetest.IgnoreResourceAttributeValue("k8s.deployment.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.replicaset.name"),
		ptracetest.IgnoreResourceAttributeValue("os.version"),
		ptracetest.IgnoreResourceAttributeValue("process.pid"),
		ptracetest.IgnoreResourceAttributeValue("splunk.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("process.runtime.version"),
		ptracetest.IgnoreResourceAttributeValue("process.command"),
		ptracetest.IgnoreResourceAttributeValue("process.command_args"),
		ptracetest.IgnoreResourceAttributeValue("process.executable.path"),
		ptracetest.IgnoreResourceAttributeValue("process.owner"),
		ptracetest.IgnoreResourceAttributeValue("process.runtime.description"),
		ptracetest.IgnoreResourceAttributeValue("splunk.zc.method"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.sdk.version"),
		ptracetest.IgnoreResourceAttributeValue("service.instance.id"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.auto.version"),
		ptracetest.IgnoreSpanAttributeValue("http.user_agent"),
		ptracetest.IgnoreSpanAttributeValue("net.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("network.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("os.version"),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
	)
	require.NoError(t, err)
}

func testJavaTraces(t *testing.T) {
	tracesConsumer := globalSinks.tracesConsumer

	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join(testDir, expectedValuesDir, "expected_java_traces.yaml")
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)
	internal.ClearTraceSchemaURLs(expectedTraces)

	internal.WaitForTraces(t, 10, tracesConsumer)

	var selectedTrace *ptrace.Traces

	require.Eventually(t, func() bool {
		for i := len(tracesConsumer.AllTraces()) - 1; i > 0; i-- {
			trace := tracesConsumer.AllTraces()[i]
			if val, ok := trace.ResourceSpans().At(0).Resource().Attributes().Get("telemetry.sdk.language"); ok && strings.Contains(val.Str(), "java") {
				if expectedTraces.SpanCount() == trace.SpanCount() && expectedTraces.ResourceSpans().Len() == trace.ResourceSpans().Len() {
					selectedTrace = &trace
					break
				}
			}
		}
		return selectedTrace != nil
	}, 3*time.Minute, 5*time.Second)
	require.NotNil(t, selectedTrace)

	maskScopeVersion(*selectedTrace)
	maskScopeVersion(expectedTraces)
	internal.ClearTraceSchemaURLs(*selectedTrace)

	internal.MaybeWriteUpdateExpectedTracesResults(t, expectedTracesFile, selectedTrace)
	err = ptracetest.CompareTraces(expectedTraces, *selectedTrace,
		ptracetest.IgnoreResourceAttributeValue("host.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.node.name"),
		ptracetest.IgnoreResourceAttributeValue("os.description"),
		ptracetest.IgnoreResourceAttributeValue("process.pid"),
		ptracetest.IgnoreResourceAttributeValue("container.id"),
		ptracetest.IgnoreResourceAttributeValue("k8s.deployment.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.replicaset.name"),
		ptracetest.IgnoreResourceAttributeValue("os.version"),
		ptracetest.IgnoreResourceAttributeValue("host.arch"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.sdk.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.auto.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("splunk.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("splunk.zc.method"),
		ptracetest.IgnoreResourceAttributeValue("service.instance.id"),
		ptracetest.IgnoreSpanAttributeValue("network.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("net.sock.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("thread.id"),
		ptracetest.IgnoreSpanAttributeValue("thread.name"),
		ptracetest.IgnoreSpanAttributeValue("os.version"),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
	)
	require.NoError(t, err)
}

func testDotNetTraces(t *testing.T) {
	tracesConsumer := globalSinks.tracesConsumer

	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join(testDir, expectedValuesDir, "expected_dotnet_traces.yaml")
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)
	internal.ClearTraceSchemaURLs(expectedTraces)

	internal.WaitForTraces(t, 30, tracesConsumer)
	var selectedTrace *ptrace.Traces

	require.Eventually(t, func() bool {
		for i := len(tracesConsumer.AllTraces()) - 1; i > 0; i-- {
			trace := tracesConsumer.AllTraces()[i]
			if val, ok := trace.ResourceSpans().At(0).Resource().Attributes().Get("telemetry.sdk.language"); ok && strings.Contains(val.Str(), "dotnet") {
				if expectedTraces.SpanCount() == trace.SpanCount() && expectedTraces.ResourceSpans().Len() == trace.ResourceSpans().Len() {
					selectedTrace = &trace
					break
				}
			}
		}
		return selectedTrace != nil
	}, 3*time.Minute, 5*time.Second)
	require.NotNil(t, selectedTrace)

	maskScopeVersion(*selectedTrace)
	maskScopeVersion(expectedTraces)
	maskSpanParentID(*selectedTrace)
	maskSpanParentID(expectedTraces)
	internal.ClearTraceSchemaURLs(*selectedTrace)

	internal.MaybeWriteUpdateExpectedTracesResults(t, expectedTracesFile, selectedTrace)
	err = ptracetest.CompareTraces(expectedTraces, *selectedTrace,
		ptracetest.IgnoreResourceAttributeValue("host.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.node.name"),
		ptracetest.IgnoreResourceAttributeValue("container.id"),
		ptracetest.IgnoreResourceAttributeValue("k8s.deployment.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.replicaset.name"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.sdk.version"),
		ptracetest.IgnoreResourceAttributeValue("telemetry.auto.version"),
		ptracetest.IgnoreResourceAttributeValue("splunk.distro.version"),
		ptracetest.IgnoreResourceAttributeValue("splunk.zc.method"),
		ptracetest.IgnoreResourceAttributeValue("service.instance.id"),
		ptracetest.IgnoreSpanAttributeValue("net.sock.peer.port"),
		ptracetest.IgnoreSpanAttributeValue("thread.id"),
		ptracetest.IgnoreSpanAttributeValue("thread.name"),
		ptracetest.IgnoreSpanAttributeValue("os.version"),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
	)
	require.NoError(t, err)
}

// Metrics tests — match by telemetry.sdk.language + service.name via the
// default metrics pipeline (signalfx exporter).

func testJavaMetrics(t *testing.T) {
	checkMetricsFromApp(t, globalSinks.agentMetricsConsumer, "java", "java-test", []string{
		"jvm.memory.used",
		"jvm.thread.count",
	})
}

func testNodeJSMetrics(t *testing.T) {
	checkMetricsFromApp(t, globalSinks.agentMetricsConsumer, "nodejs", "nodejs-test", []string{
		"process.runtime.nodejs.memory.heap.used",
		"process.runtime.nodejs.memory.rss",
	})
}

func testDotNetMetrics(t *testing.T) {
	checkMetricsFromApp(t, globalSinks.agentMetricsConsumer, "dotnet", "dotnet-test", []string{
		"process.runtime.dotnet.gc.collections.count",
	})
}

func testPythonMetrics(t *testing.T) {
	checkMetricsFromApp(t, globalSinks.agentMetricsConsumer, "python", "python-test", []string{
		"process.runtime.cpython.gc_count",
	})
}

// Profiling tests — look for com.splunk.sourcetype=otel.profiling* logs
// carrying the expected telemetry.sdk.language + service.name.

func testJavaProfiling(t *testing.T)   { checkProfilingFromApp(t, "java", "java-test") }
func testNodeJSProfiling(t *testing.T) { checkProfilingFromApp(t, "nodejs", "nodejs-test") }
func testDotNetProfiling(t *testing.T) { checkProfilingFromApp(t, "dotnet", "dotnet-test") }
func testPythonProfiling(t *testing.T) { checkProfilingFromApp(t, "python", "python-test") }

func checkProfilingFromApp(t *testing.T, sdkLanguage, serviceName string) {
	lc := globalSinks.logsConsumer
	label := sdkLanguage + "/" + serviceName
	require.Eventuallyf(t, func() bool {
		return hasProfilingFromApp(lc, sdkLanguage, serviceName)
	}, 3*time.Minute, 5*time.Second,
		"no profiling logs (otel.profiling*) for %s within timeout", label)
	t.Logf("Received profiling logs from %s", label)
}

// hasProfilingFromApp checks the logs sink for profiling entries matching the app.
// After the HEC round-trip, identity attrs may appear at the resource or log record level.
func hasProfilingFromApp(lc *consumertest.LogsSink, sdkLanguage, serviceName string) bool {
	matchIdentity := func(attrs pcommon.Map) bool {
		return hasAttrMatch(attrs, "telemetry.sdk.language", sdkLanguage) &&
			hasAttrMatch(attrs, "service.name", serviceName)
	}
	for _, logs := range lc.AllLogs() {
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			rl := logs.ResourceLogs().At(i)
			resAttrs := rl.Resource().Attributes()
			stVal, stOk := resAttrs.Get("com.splunk.sourcetype")
			if !stOk || !strings.HasPrefix(stVal.Str(), "otel.profiling") {
				continue
			}
			if matchIdentity(resAttrs) {
				return true
			}
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				for k := 0; k < rl.ScopeLogs().At(j).LogRecords().Len(); k++ {
					if matchIdentity(rl.ScopeLogs().At(j).LogRecords().At(k).Attributes()) {
						return true
					}
				}
			}
		}
	}
	return false
}
