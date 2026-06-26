// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

// Package assertiongen converts committed metric goldens to pmetricassert snapshots.
package assertiongen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/stretchr/testify/require"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

type migration struct {
	name               string
	dir                string
	golden             string
	assertion          string
	exists             []string
	regex              map[string]string
	firstDatapointOnly []string
	ignoreScopeVersion bool
}

var migrations = []migration{
	{
		name:      "cluster_receiver",
		dir:       filepath.Join("functional", "testdata", "expected_kind_values"),
		golden:    "expected_cluster_receiver.yaml",
		assertion: "expected_cluster_receiver_assertion.yaml",
		exists: internal.ExtendMetricAssertionAttrs(
			internal.CommonK8sMetricAssertionExistsAttrs,
			"k8s.container.status.last_terminated_reason",
		),
		regex: internal.CommonK8sMetricAssertionRegexAttrs,
		firstDatapointOnly: []string{
			"k8s.container.ready",
			"k8s.container.restarts",
			"k8s.pod.phase",
		},
		ignoreScopeVersion: true,
	},
}

func (m migration) paths() (goldenPath, assertionPath string) {
	base := filepath.Join("..", "..", m.dir)
	return filepath.Join(base, m.golden), filepath.Join(base, m.assertion)
}

func TestGenerateAssertions(t *testing.T) {
	if os.Getenv("GENERATE_ASSERTION") != "true" {
		t.Skip("set GENERATE_ASSERTION=true to regenerate assertion snapshots")
	}
	for _, m := range migrations {
		t.Run(m.name, func(t *testing.T) {
			goldenPath, assertionPath := m.paths()
			metrics, err := golden.ReadMetrics(goldenPath)
			require.NoError(t, err, "failed to read golden %s", goldenPath)
			require.NoError(t, internal.WriteMetricsAssertion(t, assertionPath, metrics, m.options()...))
			t.Logf("wrote assertion snapshot to %s", assertionPath)
		})
	}
}

func TestAssertionsMatchGolden(t *testing.T) {
	for _, m := range migrations {
		t.Run(m.name, func(t *testing.T) {
			goldenPath, assertionPath := m.paths()
			_, err := os.Stat(assertionPath)
			require.NoError(t, err,
				"assertion snapshot %s is missing; run `cd functional_tests && GENERATE_ASSERTION=true go test ./internal/assertiongen -run TestGenerateAssertions -v`",
				assertionPath)
			metrics, err := golden.ReadMetrics(goldenPath)
			require.NoError(t, err, "failed to read golden %s", goldenPath)
			require.NoError(t, internal.CompareMetricsAssertion(assertionPath, metrics, m.options()...))
		})
	}
}

func (m migration) options() []internal.MetricsAssertionOption {
	opts := []internal.MetricsAssertionOption{
		internal.WithVolatileAttributes(m.exists...),
		internal.WithRegexAttributes(m.regex),
		internal.WithFirstDatapointOnly(m.firstDatapointOnly...),
	}
	if m.ignoreScopeVersion {
		opts = append(opts, internal.WithIgnoreScopeVersion())
	}
	return opts
}
