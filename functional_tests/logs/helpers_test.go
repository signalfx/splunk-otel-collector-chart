// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"strings"

	"go.opentelemetry.io/collector/consumer/consumertest"
)

func findAttributesForLog(logsConsumer *consumertest.LogsSink, marker string, keys ...string) (map[string]string, bool) {
	for _, logs := range logsConsumer.AllLogs() {
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			rl := logs.ResourceLogs().At(i)
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				sl := rl.ScopeLogs().At(j)
				for k := 0; k < sl.LogRecords().Len(); k++ {
					lr := sl.LogRecords().At(k)
					if !strings.Contains(lr.Body().AsString(), marker) {
						continue
					}
					attrs := make(map[string]string, len(keys))
					for _, key := range keys {
						if value, exists := rl.Resource().Attributes().Get(key); exists {
							attrs[key] = value.AsString()
						}
						if value, exists := lr.Attributes().Get(key); exists {
							attrs[key] = value.AsString()
						}
					}
					return attrs, true
				}
			}
		}
	}
	return nil, false
}
