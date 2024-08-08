// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build histogram

package functional_tests

import (
	"bytes"
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"text/template"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
)

const (
	otlpReceiverPort = 4317

	testValuesFile = "test_values.yaml.tmpl"
	testDir        = "testdata_histogram"
	valuesDir      = "values"
)

var setupRun = sync.Once{}

var histogramMetricsConsumer *consumertest.MetricsSink

func setupOnce(t *testing.T) *consumertest.MetricsSink {
	setupRun.Do(func() {
		histogramMetricsConsumer = setupOtlpReceiver(t, otlpReceiverPort)

		if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
			teardown(t)
		}
		// deploy the chart and applications.
		if os.Getenv("SKIP_SETUP") == "true" {
			t.Log("Skipping setup as SKIP_SETUP is set to true")
			return
		}
		deployChartsAndApps(t)
	})

	return histogramMetricsConsumer
}

func setupOtlpReceiver(t *testing.T, port int) *consumertest.MetricsSink {
	mc := new(consumertest.MetricsSink)
	f := signalfxreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*signalfxreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", port)

	rcvr, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating metrics receiver")
	t.Cleanup(func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	})

	return mc
}

func deployChartsAndApps(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	chartPath := filepath.Join("..", "helm-charts", "splunk-otel-collector")
	chart, err := loader.Load(chartPath)
	require.NoError(t, err)

	valuesBytes, err := os.ReadFile(filepath.Join(testDir, valuesDir, "test_values.yaml.tmpl"))
	require.NoError(t, err)

	hostEp := hostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}

	replacements := struct {
		IngestURL string
	}{
		fmt.Sprintf("http://%s:%d", hostEp, otlpReceiverPort),
	}
	tmpl, err := template.New("").Parse(string(valuesBytes))
	require.NoError(t, err)
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, replacements)
	require.NoError(t, err)
	var values map[string]interface{}
	err = yaml.Unmarshal(buf.Bytes(), &values)
	require.NoError(t, err)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format+"\n", v...)
	}); err != nil {
		require.NoError(t, err)
	}
	install := action.NewInstall(actionConfig)
	install.Namespace = "default"
	install.ReleaseName = "sock"
	_, err = install.Run(chart, values)
	if err != nil {
		t.Logf("error reported during helm install: %v\n", err)
		retryUpgrade := action.NewUpgrade(actionConfig)
		retryUpgrade.Namespace = "default"
		retryUpgrade.Install = true
		_, err = retryUpgrade.Run("sock", chart, values)
		require.NoError(t, err)
	}
}

func teardown(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format+"\n", v)
	}); err != nil {
		require.NoError(t, err)
	}
	uninstall := action.NewUninstall(actionConfig)
	uninstall.IgnoreNotFound = true
	uninstall.Wait = true
	_, _ = uninstall.Run("sock")
}

func Test_Histograms(t *testing.T) {
	_ = setupOnce(t)
	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	t.Run("histogram metrics captured", testHistogramMetrics)
}

func testHistogramMetrics(t *testing.T) {
	otlpMetricsSink := setupOnce(t)
	waitForMetrics(t, 5, otlpMetricsSink)

	var selected *pmetric.Metrics

	for h := len(otlpMetricsSink.AllMetrics()) - 1; h >= 0; h-- {
		m := otlpMetricsSink.AllMetrics()[h]
		foundCorrectSet := false
	OUTER:
		for i := 0; i < m.ResourceMetrics().Len(); i++ {
			for j := 0; j < m.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
				for k := 0; k < m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
					metricToConsider := m.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
					if metricToConsider.Type() == pmetric.MetricTypeHistogram {
						foundCorrectSet = true
						break OUTER
					}
				}
			}
		}
		if !foundCorrectSet {
			continue
		}
		selected = &m
	}

	require.NotNil(t, selected)

	expectedMetricsFile := filepath.Join(testDir, "expected_histogram_metrics.yaml")
	golden.WriteMetrics(t, filepath.Join(testDir, "expected_histogram_metrics.yaml"), *selected)
	expectedMetrics, err := golden.ReadMetrics(expectedMetricsFile)
	require.NoError(t, err)

	err = pmetrictest.CompareMetrics(expectedMetrics, otlpMetricsSink.AllMetrics()[len(otlpMetricsSink.AllMetrics())-1],
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(),
	)
	assert.NoError(t, err)
}
