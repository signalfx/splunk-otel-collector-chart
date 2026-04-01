// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

// dynamicAttrPlaceholders maps attribute keys whose values vary between runs
// (node names, UIDs, etc.) to deterministic placeholders of similar size/shape
// so golden-file comparisons remain stable.
var dynamicAttrPlaceholders = map[string]string{
	"k8s.pod.uid":   "00000000-0000-0000-0000-000000000000",
	"k8s.pod.name":  "log-attr-test-0000000000-xxxxx",
	"k8s.node.name": "node-000",
	"host.name":     "node-000",
}

// podUIDRe matches a UUID embedded in file paths (the pod UID segment).
var podUIDRe = regexp.MustCompile(`[0-9a-f]{8}[-_][0-9a-f]{4}[-_][0-9a-f]{4}[-_][0-9a-f]{4}[-_][0-9a-f]{12}`)

// podNameRe matches a deployment-generated pod name (name-replicaset-suffix).
var podNameRe = regexp.MustCompile(`log-attr-test-[0-9a-z]+-[0-9a-z]+`)

func validateLogAttributes(t *testing.T, logsConsumer *consumertest.LogsSink) {
	internal.WaitForLogs(t, 5, logsConsumer)

	var found bool
	foundLog := plog.NewLogs()
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		for _, l := range logsConsumer.AllLogs() {
			for j := 0; j < l.ResourceLogs().Len(); j++ {
				rl := l.ResourceLogs().At(j)
				for k := 0; k < rl.ScopeLogs().Len(); k++ {
					sl := rl.ScopeLogs().At(k)
					for m := 0; m < sl.LogRecords().Len(); m++ {
						lr := sl.LogRecords().At(m)
						v, ok := lr.Attributes().Get("k8s.container.name")
						if !ok || v.AsString() != "log-attr-test" {
							continue
						}
						if strings.Contains(lr.Body().AsString(), "LOG_ATTR_VALIDATION_MARKER") {
							foundLog = plog.NewLogs()
							newRL := foundLog.ResourceLogs().AppendEmpty()
							rl.Resource().CopyTo(newRL.Resource())
							newSL := newRL.ScopeLogs().AppendEmpty()
							lr.CopyTo(newSL.LogRecords().AppendEmpty())
							found = true
						}
					}
				}
			}
		}
		assert.True(tt, found, "log from log-attr-test container not found")
	}, 3*time.Minute, 5*time.Second)

	normalizeLogData(&foundLog)

	t.Logf("Normalized log resource attributes: %s", internal.FormatAttributes(foundLog.ResourceLogs().At(0).Resource().Attributes()))
	t.Logf("Normalized log record attributes: %s", internal.FormatAttributes(foundLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()))

	expectedFile := filepath.Join(testDir, expectedValuesDir, "expected_container_log_attributes.yaml")
	internal.MaybeUpdateExpectedLogsResults(t, expectedFile, &foundLog)

	if _, statErr := os.Stat(expectedFile); os.IsNotExist(statErr) {
		t.Skipf("Expected log attributes file not found at %s; run with UPDATE_EXPECTED_RESULTS=true to generate", expectedFile)
		return
	}

	expected, err := golden.ReadLogs(expectedFile)
	require.NoError(t, err)

	actualResAttrs := foundLog.ResourceLogs().At(0).Resource().Attributes()
	expectedResAttrs := expected.ResourceLogs().At(0).Resource().Attributes()
	require.True(t, expectedResAttrs.Equal(actualResAttrs),
		"Log resource attributes mismatch.\nExpected: %s\nActual:   %s",
		internal.FormatAttributes(expectedResAttrs), internal.FormatAttributes(actualResAttrs))

	actualLogAttrs := foundLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()
	expectedLogAttrs := expected.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()
	require.True(t, expectedLogAttrs.Equal(actualLogAttrs),
		"Log record attributes mismatch.\nExpected: %s\nActual:   %s",
		internal.FormatAttributes(expectedLogAttrs), internal.FormatAttributes(actualLogAttrs))
}

// normalizeLogData replaces dynamic attribute values with deterministic
// placeholders so golden-file comparisons are stable across runs. Static/
// config-driven values (sourcetype, index, os.type, etc.) are kept as-is.
func normalizeLogData(logs *plog.Logs) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		normalizeDynamicAttrs(rl.Resource().Attributes())
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				lr.Body().SetStr("LOG_ATTR_VALIDATION_MARKER")
				lr.SetTimestamp(0)
				lr.SetObservedTimestamp(0)
				normalizeDynamicAttrs(lr.Attributes())
			}
		}
	}
}

// normalizeDynamicAttrs replaces values of dynamic/per-run keys with fixed
// placeholders.
func normalizeDynamicAttrs(m pcommon.Map) {
	m.Range(func(k string, v pcommon.Value) bool {
		if placeholder, ok := dynamicAttrPlaceholders[k]; ok {
			m.PutStr(k, placeholder)
			return true
		}
		if k == "com.splunk.source" {
			s := podUIDRe.ReplaceAllString(v.AsString(), "00000000-0000-0000-0000-000000000000")
			s = podNameRe.ReplaceAllString(s, "log-attr-test-0000000000-xxxxx")
			m.PutStr(k, s)
		}
		return true
	})
}
