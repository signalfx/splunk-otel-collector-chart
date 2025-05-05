// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const SignalFxAPIPort = 8881

func SetupSignalFxAPIServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	s := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", SignalFxAPIPort),
		Handler:           mux,
		ReadHeaderTimeout: 60 * time.Minute,
	}

	errCh := make(chan error)
	t.Cleanup(func() {
		err := s.Close()
		require.NoError(t, err)
		err = <-errCh
		require.NoError(t, err)
	})

	go func() {
		if err := s.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		errCh <- nil
	}()
}
