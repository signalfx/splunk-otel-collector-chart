// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	logsToMetricsTestNamespace     = "logs-to-metrics-test"
	logsToMetricsContainerName     = "logs-to-metrics-fixture"
	logsToMetricsMismatchContainer = "logs-to-metrics-label-mismatch"
	logsToMetricsValuesFile        = "logs_to_metrics_values.yaml.tmpl"
	logsToMetricsTestdataDir       = "testdata"
	logsToMetricsManifestsDir      = "testdata/logs_to_metrics_testobjects"
	logsToMetricsErrorMetric       = "app.log.error.count"
	logsToMetricsThrottledMetric   = "app.request.throttled.count"
	logsToMetricsTransactionMetric = "business.transaction.value"

	logsToMetricsErrorLog               = `{"message":"LOGS_TO_METRICS_ERROR_MARKER","app.level":"ERROR","app.error_code":"payment_declined"}`
	logsToMetricsInvalidErrorTypeLog    = `{"message":"LOGS_TO_METRICS_INVALID_ERROR_TYPE_MARKER","severity_text":"ERROR","app.error_code":"must_not_replace_invalid_canonical","error.type":{"code":"payment_declined","request_id":"abc-123"}}`
	logsToMetricsLabelMismatchLog       = `{"message":"LOGS_TO_METRICS_LABEL_MISMATCH_MARKER","app.level":"ERROR","app.error_code":"label_mismatch"}`
	logsToMetricsThrottledNoStatusLog   = `{"message":"LOGS_TO_METRICS_THROTTLED_NO_STATUS_MARKER","request.throttled":true,"http.request.method":"DELETE"}`
	logsToMetricsThrottledWithStatusLog = `{"message":"LOGS_TO_METRICS_THROTTLED_WITH_STATUS_MARKER","request.throttled":true,"http.request.method":"POST","http.response.status_code":429}`
	logsToMetricsTransactionLog         = `{"message":"LOGS_TO_METRICS_TRANSACTION_MARKER","event.outcome":"success","business.transaction.value":49.95,"business.transaction.type":"checkout","business.transaction.unit":"USD"}`
)

// Test_LogsToMetricsRuntime verifies the enabled chart with a running Collector. It confirms
// the file_log fan-out does not mutate source logs and that matching flat JSON fields produce
// the expected count and sum metrics.
func Test_LogsToMetricsRuntime(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	config, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		t.Log("Running teardown before setup as TEARDOWN_BEFORE_SETUP is set to true")
		logsToMetricsTeardown(t, k8sClient)
	}

	logsSink := internal.SetupHECLogsSink(t)
	metricsSink := internal.SetupHECMetricsSink(t)

	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		logsToMetricsDeployWorkloadAndCollector(t, testKubeConfig, clientset, k8sClient)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	t.Run("SourceLogsAreDeliveredUnchanged", func(t *testing.T) {
		require.EventuallyWithT(t, func(tt *assert.CollectT) {
			bodies := collectBodiesFromContainer(logsSink, logsToMetricsContainerName)
			assert.Contains(tt, bodies, logsToMetricsErrorLog)
			assert.Contains(tt, bodies, logsToMetricsInvalidErrorTypeLog)
			assert.Contains(tt, bodies, logsToMetricsThrottledNoStatusLog)
			assert.Contains(tt, bodies, logsToMetricsThrottledWithStatusLog)
			assert.Contains(tt, bodies, logsToMetricsTransactionLog)
			mismatchBodies := collectBodiesFromContainer(logsSink, logsToMetricsMismatchContainer)
			assert.Contains(tt, mismatchBodies, logsToMetricsLabelMismatchLog)
		}, 3*time.Minute, 5*time.Second)
	})

	t.Run("MatchingLogsGenerateExpectedMetrics", func(t *testing.T) {
		require.EventuallyWithT(t, func(tt *assert.CollectT) {
			errorValue, metricAttrs, errorFound := findNumberDatapointWithAttributes(metricsSink, logsToMetricsErrorMetric, map[string]string{
				"error.type":   "payment_declined",
				"log.severity": "ERROR",
			})
			if assert.True(tt, errorFound, "expected %s data point", logsToMetricsErrorMetric) {
				assert.InDelta(tt, 1, errorValue, 0.0001)
				assert.NotEmpty(tt, metricAttrs["k8s.pod.name"])
				assert.NotEmpty(tt, metricAttrs["k8s.pod.uid"])
				assert.NotEmpty(tt, metricAttrs["k8s.node.name"])
				assert.NotEmpty(tt, metricAttrs["container.id"])
				assert.NotContains(tt, metricAttrs, "splunk.logs_to_metrics")
				assert.NotContains(tt, metricAttrs, "splunk.logs_to_metrics.scope.pod_label.logs-to-metrics-test")
			}

			invalidTypeValue, invalidTypeFound := findNumberDatapoint(metricsSink, logsToMetricsErrorMetric, map[string]string{
				"error.type":   "unknown",
				"log.severity": "ERROR",
			})
			if assert.True(tt, invalidTypeFound, "expected malformed error.type to use the default dimension") {
				assert.InDelta(tt, 1, invalidTypeValue, 0.0001)
			}

			transactionValue, transactionFound := findNumberDatapoint(metricsSink, logsToMetricsTransactionMetric, map[string]string{
				"business.transaction.type": "checkout",
				"business.transaction.unit": "USD",
			})
			if assert.True(tt, transactionFound, "expected %s data point", logsToMetricsTransactionMetric) {
				assert.InDelta(tt, 49.95, transactionValue, 0.0001)
			}

			throttledValue, throttledFound := findNumberDatapoint(metricsSink, logsToMetricsThrottledMetric, map[string]string{
				"http.request.method":       "POST",
				"http.response.status_code": "429",
			})
			if assert.True(tt, throttledFound, "expected %s data point", logsToMetricsThrottledMetric) {
				assert.InDelta(tt, 1, throttledValue, 0.0001)
			}
		}, 3*time.Minute, 5*time.Second)

		assert.Never(t, func() bool {
			_, found := findNumberDatapoint(metricsSink, logsToMetricsThrottledMetric, map[string]string{
				"http.request.method": "DELETE",
			})
			if found {
				return true
			}
			_, found = findNumberDatapoint(metricsSink, logsToMetricsErrorMetric, map[string]string{
				"error.type": "label_mismatch",
			})
			return found
		}, 15*time.Second, time.Second, "invalid throttled and pod-label-mismatched records must not generate metrics")
	})
}

func findNumberDatapoint(sink *consumertest.MetricsSink, metricName string, expectedAttrs map[string]string) (float64, bool) {
	value, _, found := findNumberDatapointWithAttributes(sink, metricName, expectedAttrs)
	return value, found
}

func findNumberDatapointWithAttributes(sink *consumertest.MetricsSink, metricName string, expectedAttrs map[string]string) (float64, map[string]any, bool) {
	for _, metrics := range sink.AllMetrics() {
		for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
			scopeMetrics := metrics.ResourceMetrics().At(i).ScopeMetrics()
			for j := 0; j < scopeMetrics.Len(); j++ {
				metricSlice := scopeMetrics.At(j).Metrics()
				for k := 0; k < metricSlice.Len(); k++ {
					metric := metricSlice.At(k)
					if metric.Name() != metricName {
						continue
					}
					switch metric.Type() {
					case pmetric.MetricTypeGauge:
						if value, attrs, found := findNumberDatapointInSlice(metric.Gauge().DataPoints(), expectedAttrs); found {
							return value, attrs, true
						}
					case pmetric.MetricTypeSum:
						if value, attrs, found := findNumberDatapointInSlice(metric.Sum().DataPoints(), expectedAttrs); found {
							return value, attrs, true
						}
					case pmetric.MetricTypeEmpty, pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeSummary:
					}
				}
			}
		}
	}
	return 0, nil, false
}

func findNumberDatapointInSlice(datapoints pmetric.NumberDataPointSlice, expectedAttrs map[string]string) (float64, map[string]any, bool) {
	for i := 0; i < datapoints.Len(); i++ {
		datapoint := datapoints.At(i)
		if !attributesMatch(datapoint.Attributes(), expectedAttrs) {
			continue
		}
		switch datapoint.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			return float64(datapoint.IntValue()), datapoint.Attributes().AsRaw(), true
		case pmetric.NumberDataPointValueTypeDouble:
			return datapoint.DoubleValue(), datapoint.Attributes().AsRaw(), true
		}
	}
	return 0, nil, false
}

func attributesMatch(attributes pcommon.Map, expected map[string]string) bool {
	for key, expectedValue := range expected {
		actual, exists := attributes.Get(key)
		if !exists || actual.Type() != pcommon.ValueTypeStr || actual.Str() != expectedValue {
			return false
		}
	}
	return true
}

func logsToMetricsDeployWorkloadAndCollector(t *testing.T, testKubeConfig string, clientset *kubernetes.Clientset, k8sClient *k8stest.K8sClient) {
	t.Helper()

	valuesFile, err := filepath.Abs(filepath.Join(logsToMetricsTestdataDir, logsToMetricsValuesFile))
	require.NoError(t, err)

	hostEp := internal.HostEndpoint(t)
	require.NotEmpty(t, hostEp, "host endpoint not found")

	replacements := map[string]any{
		"LogHecEndpoint":    internal.HostPortHTTP(hostEp, internal.HECLogsReceiverPort),
		"MetricHecEndpoint": internal.HostPortHTTP(hostEp, internal.HECMetricsReceiverPort) + "/services/collector",
	}
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 0, internal.GetDefaultChartOptions())

	agentSelector := internal.AgentLabelSelector + ",release=" + internal.DefaultChartReleaseName
	internal.CheckPodsReady(t, clientset, internal.DefaultNamespace, agentSelector, 3*time.Minute, 5*time.Second)
	internal.CreateNamespace(t, clientset, logsToMetricsTestNamespace)
	internal.WaitForDefaultServiceAccount(t, clientset, logsToMetricsTestNamespace)

	createdObjs, err := k8stest.CreateObjects(k8sClient, logsToMetricsManifestsDir)
	require.NoError(t, err)
	require.NotEmpty(t, createdObjs)
	internal.CheckPodsReady(t, clientset, logsToMetricsTestNamespace, "logs-to-metrics-fixture=true", 2*time.Minute, 0)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		logsToMetricsTeardown(t, k8sClient)
	})
}

func logsToMetricsTeardown(t *testing.T, k8sClient *k8stest.K8sClient) {
	t.Helper()
	testKubeConfig := os.Getenv("KUBECONFIG")
	internal.ChartUninstall(t, testKubeConfig)
	internal.DeleteObject(t, k8sClient, `
apiVersion: v1
kind: Namespace
metadata:
  name: logs-to-metrics-test
`)
}
