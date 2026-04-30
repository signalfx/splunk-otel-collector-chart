// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runHelmTemplate(t *testing.T, extraArgs ...string) (string, error) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	repoRoot := filepath.Clean(filepath.Join(wd, ".."))
	args := append([]string{
		"template", "test-release", "helm-charts/splunk-otel-collector",
	}, extraArgs...)
	cmd := exec.Command("helm", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestOtlpLogIngestSchemaValidation(t *testing.T) {
	t.Run("fails when OTLP ingest is enabled but logs are disabled", func(t *testing.T) {
		output, err := runHelmTemplate(t,
			"--set", "clusterName=test",
			"--set", "splunkObservability.realm=",
			"--set", "splunkPlatform.endpoint=",
			"--set", "splunkPlatform.logsEnabled=false",
			"--set", "splunkPlatform.metricsEnabled=true",
			"--set", "splunkPlatform.metricsIndex=metrics",
			"--set", "splunkPlatform.otlpIngest.enabled=true",
			"--set", "splunkPlatform.otlpIngest.endpoint=otlp-logs.example.com:4317",
		)
		if err == nil {
			t.Fatalf("expected helm template to fail, got success\noutput:\n%s", output)
		}
		if !strings.Contains(output, "/splunkPlatform/logsEnabled") {
			t.Fatalf("expected output to mention logsEnabled constraint failure, got:\n%s", output)
		}
	})

	t.Run("passes with OTLP logs only and no platform endpoint/token", func(t *testing.T) {
		output, err := runHelmTemplate(t,
			"--set", "clusterName=test",
			"--set", "splunkObservability.realm=",
			"--set", "splunkPlatform.endpoint=",
			"--set", "splunkPlatform.token=",
			"--set", "splunkPlatform.logsEnabled=true",
			"--set", "splunkPlatform.metricsEnabled=false",
			"--set", "splunkPlatform.tracesEnabled=false",
			"--set", "splunkPlatform.otlpIngest.enabled=true",
			"--set", "splunkPlatform.otlpIngest.endpoint=otlp-logs.example.com:4317",
			"--set", "splunkPlatform.otlpIngest.protocol=grpc",
		)
		if err != nil {
			t.Fatalf("expected helm template to pass, got error: %v\noutput:\n%s", err, output)
		}
	})

	t.Run("passes with OTLP logs enabled even when endpoint is set and token is empty", func(t *testing.T) {
		output, err := runHelmTemplate(t,
			"--set", "clusterName=test",
			"--set", "splunkObservability.realm=",
			"--set", "splunkPlatform.endpoint=https://hec.example.com/services/collector/event",
			"--set", "splunkPlatform.token=",
			"--set", "splunkPlatform.logsEnabled=true",
			"--set", "splunkPlatform.metricsEnabled=false",
			"--set", "splunkPlatform.tracesEnabled=false",
			"--set", "splunkPlatform.otlpIngest.enabled=true",
			"--set", "splunkPlatform.otlpIngest.endpoint=otlp-logs.example.com:4317",
			"--set", "splunkPlatform.otlpIngest.protocol=grpc",
		)
		if err != nil {
			t.Fatalf("expected helm template to pass, got error: %v\noutput:\n%s", err, output)
		}
	})

	t.Run("fails without token when endpoint is set and metrics are enabled", func(t *testing.T) {
		output, err := runHelmTemplate(t,
			"--set", "clusterName=test",
			"--set", "splunkObservability.realm=",
			"--set", "splunkPlatform.endpoint=https://hec.example.com/services/collector/event",
			"--set", "splunkPlatform.token=",
			"--set", "splunkPlatform.logsEnabled=true",
			"--set", "splunkPlatform.metricsEnabled=true",
			"--set", "splunkPlatform.metricsIndex=metrics",
			"--set", "splunkPlatform.tracesEnabled=false",
			"--set", "splunkPlatform.otlpIngest.enabled=true",
			"--set", "splunkPlatform.otlpIngest.endpoint=otlp-logs.example.com:4317",
			"--set", "splunkPlatform.otlpIngest.protocol=grpc",
		)
		if err == nil {
			t.Fatalf("expected helm template to fail, got success\noutput:\n%s", output)
		}
		if !strings.Contains(output, "/splunkPlatform/token") {
			t.Fatalf("expected output to mention token requirement, got:\n%s", output)
		}
	})

	t.Run("fails when metrics are enabled without platform endpoint", func(t *testing.T) {
		output, err := runHelmTemplate(t,
			"--set", "clusterName=test",
			"--set", "splunkObservability.realm=",
			"--set", "splunkPlatform.endpoint=",
			"--set", "splunkPlatform.token=",
			"--set", "splunkPlatform.logsEnabled=true",
			"--set", "splunkPlatform.metricsEnabled=true",
			"--set", "splunkPlatform.metricsIndex=metrics",
			"--set", "splunkPlatform.tracesEnabled=false",
			"--set", "splunkPlatform.otlpIngest.enabled=true",
			"--set", "splunkPlatform.otlpIngest.endpoint=otlp-logs.example.com:4317",
			"--set", "splunkPlatform.otlpIngest.protocol=grpc",
		)
		if err == nil {
			t.Fatalf("expected helm template to fail when metrics are enabled without platform endpoint, got success\noutput:\n%s", output)
		}
		if !strings.Contains(output, "/splunkPlatform/endpoint") {
			t.Fatalf("expected output to mention endpoint constraint failure, got:\n%s", output)
		}
	})

	t.Run("fails when traces are enabled without platform endpoint", func(t *testing.T) {
		output, err := runHelmTemplate(t,
			"--set", "clusterName=test",
			"--set", "splunkObservability.realm=",
			"--set", "splunkPlatform.endpoint=",
			"--set", "splunkPlatform.token=",
			"--set", "splunkPlatform.logsEnabled=true",
			"--set", "splunkPlatform.metricsEnabled=false",
			"--set", "splunkPlatform.tracesEnabled=true",
			"--set", "splunkPlatform.tracesIndex=traces",
			"--set", "splunkPlatform.otlpIngest.enabled=true",
			"--set", "splunkPlatform.otlpIngest.endpoint=otlp-logs.example.com:4317",
			"--set", "splunkPlatform.otlpIngest.protocol=grpc",
		)
		if err == nil {
			t.Fatalf("expected helm template to fail when traces are enabled without platform endpoint, got success\noutput:\n%s", output)
		}
		if !strings.Contains(output, "/splunkPlatform/endpoint") {
			t.Fatalf("expected output to mention endpoint constraint failure, got:\n%s", output)
		}
	})

	t.Run("fails when OTLP ingest is enabled but otlpIngest.endpoint is empty", func(t *testing.T) {
		output, err := runHelmTemplate(t,
			"--set", "clusterName=test",
			"--set", "splunkObservability.realm=",
			"--set", "splunkPlatform.endpoint=",
			"--set", "splunkPlatform.token=",
			"--set", "splunkPlatform.logsEnabled=true",
			"--set", "splunkPlatform.metricsEnabled=false",
			"--set", "splunkPlatform.tracesEnabled=false",
			"--set", "splunkPlatform.otlpIngest.enabled=true",
			"--set", "splunkPlatform.otlpIngest.endpoint=",
		)
		if err == nil {
			t.Fatalf("expected helm template to fail when otlpIngest.endpoint is empty, got success\noutput:\n%s", output)
		}
		if !strings.Contains(output, "/splunkPlatform/otlpIngest/endpoint") {
			t.Fatalf("expected output to mention otlpIngest.endpoint constraint failure, got:\n%s", output)
		}
	})
}
