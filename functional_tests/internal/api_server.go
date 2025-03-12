// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

const SignalFxAPIPort = 8881

func SetupSignalFxApiServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
	})

	_, cancelCtx := context.WithCancel(context.Background())
	s := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", SignalFxAPIPort),
		Handler: mux,
	}

	t.Cleanup(func() {
		cancelCtx()
	})

	go func() {
		if err := s.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			require.NoError(t, err)
		}
	}()
}
