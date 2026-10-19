// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSendHTTPRequestWaitsForExpectedStatus(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if attempts.Add(1) == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte("no healthy upstream"))
					return
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte("workload response"))
			}))
			t.Cleanup(server.Close)

			sendHTTPRequest(t, server.Client(), server.URL, "httpbin.example.com", "httpbin.example.com:80", "/status", status)
			require.EqualValues(t, 2, attempts.Load(), "the ingress error must be retried before accepting the workload response")
		})
	}
}
