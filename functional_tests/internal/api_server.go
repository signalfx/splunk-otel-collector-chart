// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const SignalFxAPIPort = 8881

// SetupSignalFxAPIServer starts a local API endpoint that accepts all requests.
func SetupSignalFxAPIServer(t *testing.T) {
	t.Helper()
	SetupSignalFxAPIServerWithHandler(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
}

// SetupSignalFxAPIServerWithHandler starts the API endpoint with a custom handler.
func SetupSignalFxAPIServerWithHandler(t *testing.T, handler http.Handler) {
	t.Helper()
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", SignalFxAPIPort))
	require.NoError(t, err)

	s := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: time.Minute,
	}
	errCh := make(chan error, 1)
	t.Cleanup(func() {
		require.NoError(t, s.Close())
		require.ErrorIs(t, <-errCh, http.ErrServerClosed)
	})
	go func() {
		errCh <- s.Serve(listener)
	}()
}
