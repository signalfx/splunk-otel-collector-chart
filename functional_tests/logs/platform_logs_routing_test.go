// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"k8s.io/client-go/kubernetes"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	applicationHECPort            = 8093
	securityHECPort               = 8094
	incomingResourceIndex         = "resource-index"
	incomingResourceSource        = "resource-source"
	incomingResourceSourcetype    = "resource:sourcetype"
	incomingLogIndex              = "log-record-index"
	incomingLogSource             = "log-record-source"
	incomingLogSourcetype         = "log-record:sourcetype"
	globalIndex                   = "global-index"
	globalSource                  = "global-source"
	globalSourcetype              = "global:sourcetype"
	applicationSource             = "application-source"
	applicationSourcetype         = "application:sourcetype"
	applicationRoutingMarker      = "application-marker"
	securityRoutingMarker         = "security-retry-marker"
	defaultRoutingMarker          = "default-marker"
	missingRoutingMarker          = "missing-attribute-marker"
	k8sApplicationRoutingMarker   = "K8S_APPLICATION_ROUTE_MARKER"
	k8sSecurityRoutingMarker      = "K8S_SECURITY_ROUTE_MARKER"
	k8sPodDefaultRoutingMarker    = "K8S_POD_DEFAULT_ROUTE_MARKER"
	k8sNamespaceRoutingMarker     = "K8S_NAMESPACE_DEFAULT_ROUTE_MARKER"
	k8sPodIndex                   = "pod-index"
	k8sPodSourcetype              = "pod:sourcetype"
	k8sNamespaceIndex             = "namespace-index"
	platformLogsRoutingNamespace  = "platform-logs-routing-precedence"
	platformLogsRoutingRunConfig  = "platform-logs-routing-run"
	platformLogsRoutingManifests  = "testdata/platform_logs_routing_testobjects"
	platformLogsRoutingAttribute  = "k8s.pod.labels.product"
	platformLogsRoutingValuesFile = "platform_logs_routing_values.yaml.tmpl"
)

func Test_PlatformLogsRouting(t *testing.T) {
	kubeconfig, ok := os.LookupEnv("KUBECONFIG")
	require.True(t, ok, "the environment variable KUBECONFIG must be set")

	k8sClient, err := k8stest.NewK8sClient(kubeconfig)
	require.NoError(t, err)
	clientset, err := internal.GetKubeClient(kubeconfig)
	require.NoError(t, err)
	otlpClient := &http.Client{Timeout: 5 * time.Second}
	t.Cleanup(otlpClient.CloseIdleConnections)

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		internal.ChartUninstall(t, kubeconfig)
	}

	for _, tc := range []struct {
		name           string
		gatewayEnabled bool
	}{
		{name: "direct-agent", gatewayEnabled: false},
		{name: "gateway", gatewayEnabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if os.Getenv("SKIP_SETUP") == "true" {
				t.Skip("SKIP_SETUP is not supported between routing modes")
			}
			// Remove leftovers from an interrupted run before creating this mode's
			// workloads. The shared delete helper waits for namespace termination.
			platformLogsRoutingDeletePrecedenceWorkloads(t, k8sClient)

			defaultSink := internal.SetupHECLogsSink(t)
			applicationSink := internal.SetupHECLogsSinkOnPort(t, applicationHECPort)

			platformLogsRoutingInstallChart(t, kubeconfig, tc.gatewayEnabled)
			t.Cleanup(func() {
				if os.Getenv("SKIP_TEARDOWN") != "true" {
					internal.ChartUninstall(t, kubeconfig)
				}
			})
			runID := platformLogsRoutingDeployPrecedenceWorkloads(t, clientset, k8sClient)
			applicationMarker := platformLogsRoutingRunMarker(runID, applicationRoutingMarker)
			securityMarker := platformLogsRoutingRunMarker(runID, securityRoutingMarker)
			defaultMarker := platformLogsRoutingRunMarker(runID, defaultRoutingMarker)
			missingMarker := platformLogsRoutingRunMarker(runID, missingRoutingMarker)
			k8sApplicationMarker := platformLogsRoutingRunMarker(runID, k8sApplicationRoutingMarker)
			k8sSecurityMarker := platformLogsRoutingRunMarker(runID, k8sSecurityRoutingMarker)
			k8sPodDefaultMarker := platformLogsRoutingRunMarker(runID, k8sPodDefaultRoutingMarker)
			k8sNamespaceMarker := platformLogsRoutingRunMarker(runID, k8sNamespaceRoutingMarker)

			if os.Getenv("SKIP_TESTS") == "true" {
				t.Log("Skipping assertions as SKIP_TESTS is set")
				return
			}

			// Keep the security HEC endpoint unavailable first. Its exporter must
			// retry without preventing healthy routes from making progress while
			// the bounded security queue still has capacity.
			sendPlatformLogsRoutingOTLPLog(t, otlpClient, "security", securityMarker, true, true)
			sendPlatformLogsRoutingOTLPLog(t, otlpClient, "application", applicationMarker, true, true)
			sendPlatformLogsRoutingOTLPLog(t, otlpClient, "unmatched", defaultMarker, true, false)
			sendPlatformLogsRoutingOTLPLog(t, otlpClient, "", missingMarker, false, false)

			// The rows make the precedence contract visible. The Kubernetes rows use
			// real annotated container logs, not synthetic attributes named after annotations.
			for _, expectation := range []platformLogsRoutingExpectation{
				{
					sink: applicationSink, marker: applicationMarker, destination: "application",
					index: "application", source: applicationSource, sourcetype: applicationSourcetype,
				},
				{
					sink: defaultSink, marker: defaultMarker, destination: "default",
					index: incomingResourceIndex, source: incomingResourceSource, sourcetype: globalSourcetype,
				},
				{
					sink: defaultSink, marker: missingMarker, destination: "default",
					index: globalIndex, source: globalSource, sourcetype: globalSourcetype,
				},
				{
					sink: applicationSink, marker: k8sApplicationMarker, destination: "application",
					index: "application", source: applicationSource, sourcetype: applicationSourcetype,
				},
				{
					sink: defaultSink, marker: k8sPodDefaultMarker, destination: "default",
					index: k8sPodIndex, sourceContains: "/default-pod-precedence/0.log", sourcetype: k8sPodSourcetype,
				},
				{
					sink: defaultSink, marker: k8sNamespaceMarker, destination: "default",
					index: k8sNamespaceIndex, sourceContains: "/namespace-precedence/0.log", sourcetype: globalSourcetype,
				},
			} {
				waitForPlatformLogsRoutingExpectation(t, expectation)
			}

			securitySink := internal.SetupHECLogsSinkOnPort(t, securityHECPort)
			for _, expectation := range []platformLogsRoutingExpectation{
				{
					sink: securitySink, marker: securityMarker, destination: "security",
					index: "security", source: incomingLogSource, sourcetype: incomingLogSourcetype,
				},
				{
					sink: securitySink, marker: k8sSecurityMarker, destination: "security",
					index: "security", sourceContains: "/security-precedence/0.log", sourcetype: k8sPodSourcetype,
				},
			} {
				waitForPlatformLogsRoutingExpectation(t, expectation)
			}

			assertPlatformLogsRoutingMarkerIsolation(t, []platformLogsRoutingIsolationExpectation{
				{
					destination: "default", sink: defaultSink,
					excluded: []string{applicationMarker, securityMarker, k8sApplicationMarker, k8sSecurityMarker},
				},
				{
					destination: "application", sink: applicationSink,
					excluded: []string{defaultMarker, missingMarker, securityMarker, k8sPodDefaultMarker, k8sNamespaceMarker, k8sSecurityMarker},
				},
				{
					destination: "security", sink: securitySink,
					excluded: []string{defaultMarker, missingMarker, applicationMarker, k8sPodDefaultMarker, k8sNamespaceMarker, k8sApplicationMarker},
				},
			})
		})
	}
}

func platformLogsRoutingDeployPrecedenceWorkloads(t *testing.T, clientset *kubernetes.Clientset, k8sClient *k8stest.K8sClient) string {
	t.Helper()
	internal.CreateNamespace(t, clientset, platformLogsRoutingNamespace)
	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") != "true" {
			platformLogsRoutingDeletePrecedenceWorkloads(t, k8sClient)
		}
	})
	internal.AnnotateNamespace(t, clientset, platformLogsRoutingNamespace, "splunk.com/index", k8sNamespaceIndex)
	internal.WaitForDefaultServiceAccount(t, clientset, platformLogsRoutingNamespace)
	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	_, err := k8stest.CreateObject(k8sClient, []byte(fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  run-id: %q
`, platformLogsRoutingRunConfig, platformLogsRoutingNamespace, runID)))
	require.NoError(t, err)

	createdObjects, err := k8stest.CreateObjects(k8sClient, platformLogsRoutingManifests)
	require.NoError(t, err)
	require.Len(t, createdObjects, 4)
	internal.CheckPodsReady(t, clientset, platformLogsRoutingNamespace, "test=platform-logs-routing-precedence", 2*time.Minute, 0)
	return runID
}

func platformLogsRoutingRunMarker(runID, marker string) string {
	return runID + ":" + marker
}

func platformLogsRoutingDeletePrecedenceWorkloads(t *testing.T, k8sClient *k8stest.K8sClient) {
	t.Helper()
	internal.DeleteObject(t, k8sClient, fmt.Sprintf(`
apiVersion: v1
kind: Namespace
metadata:
  name: %s
`, platformLogsRoutingNamespace))
}

func platformLogsRoutingInstallChart(t *testing.T, kubeconfig string, gatewayEnabled bool) {
	t.Helper()
	host := internal.HostEndpoint(t)
	require.NotEmpty(t, host)

	valuesFile, err := filepath.Abs(filepath.Join("testdata", platformLogsRoutingValuesFile))
	require.NoError(t, err)
	internal.ChartInstallOrUpgrade(t, kubeconfig, valuesFile, map[string]any{
		"GatewayEnabled": gatewayEnabled,
		"DefaultURL":     internal.HostPortHTTP(host, internal.HECLogsReceiverPort),
		"ApplicationURL": internal.HostPortHTTP(host, applicationHECPort),
		"SecurityURL":    internal.HostPortHTTP(host, securityHECPort),
	}, 0, internal.GetDefaultChartOptions())
}

func sendPlatformLogsRoutingOTLPLog(t *testing.T, client *http.Client, product, marker string, includeResourceMetadata, includeLogMetadata bool) {
	t.Helper()
	resourceAttributes := []string{
		fmt.Sprintf(`{"key": "routing.test.marker", "value": {"stringValue": %q}}`, marker),
	}
	if product != "" {
		resourceAttributes = append(resourceAttributes, fmt.Sprintf(
			`{"key": %q, "value": {"stringValue": %q}}`,
			platformLogsRoutingAttribute,
			product,
		))
	}
	if includeResourceMetadata {
		resourceAttributes = append(resourceAttributes,
			fmt.Sprintf(`{"key": "com.splunk.index", "value": {"stringValue": %q}}`, incomingResourceIndex),
			fmt.Sprintf(`{"key": "com.splunk.source", "value": {"stringValue": %q}}`, incomingResourceSource),
			fmt.Sprintf(`{"key": "com.splunk.sourcetype", "value": {"stringValue": %q}}`, incomingResourceSourcetype),
		)
	}
	logAttributes := ""
	if includeLogMetadata {
		logAttributes = fmt.Sprintf(`"attributes": [
		{"key": "com.splunk.index", "value": {"stringValue": %q}},
		{"key": "com.splunk.source", "value": {"stringValue": %q}},
		{"key": "com.splunk.sourcetype", "value": {"stringValue": %q}}
	  ],`, incomingLogIndex, incomingLogSource, incomingLogSourcetype)
	}
	payload := fmt.Sprintf(`{
	  "resourceLogs": [{
	    "resource": {"attributes": [
	      %s
	    ]},
	    "scopeLogs": [{
	      "scope": {"name": "logs-routing-functional-test"},
	      "logRecords": [{%s "body": {"stringValue": %q}}]
	    }]
	  }]
	}`, strings.Join(resourceAttributes, ",\n      "), logAttributes, marker)

	url := "http://" + internal.HostPort("127.0.0.1", 43180) + "/v1/logs"
	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewBufferString(payload))
		if err != nil {
			return false
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode >= 200 && resp.StatusCode < 300
	}, time.Minute, time.Second, "failed to send OTLP log marker %q", marker)
}

type platformLogsRoutingExpectation struct {
	sink           *consumertest.LogsSink
	marker         string
	destination    string
	index          string
	source         string
	sourceContains string
	sourcetype     string
}

func waitForPlatformLogsRoutingExpectation(t *testing.T, expectation platformLogsRoutingExpectation) {
	t.Helper()
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		attrs, found := findAttributesForLog(
			expectation.sink,
			expectation.marker,
			"routing.destination",
			"com.splunk.index",
			"com.splunk.source",
			"com.splunk.sourcetype",
		)
		if !assert.True(tt, found, "marker %q did not reach its HEC sink", expectation.marker) {
			return
		}
		assert.Equal(tt, expectation.destination, attrs["routing.destination"])
		assert.Equal(tt, expectation.index, attrs["com.splunk.index"])
		if expectation.sourceContains != "" {
			assert.Contains(tt, attrs["com.splunk.source"], expectation.sourceContains)
		} else {
			assert.Equal(tt, expectation.source, attrs["com.splunk.source"])
		}
		assert.Equal(tt, expectation.sourcetype, attrs["com.splunk.sourcetype"])
	}, 3*time.Minute, time.Second)
}

type platformLogsRoutingIsolationExpectation struct {
	destination string
	sink        *consumertest.LogsSink
	excluded    []string
}

func assertPlatformLogsRoutingMarkerIsolation(t *testing.T, expectations []platformLogsRoutingIsolationExpectation) {
	t.Helper()
	require.Never(t, func() bool {
		for _, expectation := range expectations {
			for _, marker := range expectation.excluded {
				if platformLogsRoutingSinkHasMarker(expectation.sink, marker) {
					t.Logf("marker %q reached the wrong %s HEC sink", marker, expectation.destination)
					return true
				}
			}
		}
		return false
	}, 2*time.Second, 100*time.Millisecond, "an excluded marker reached the wrong HEC sink")
}

func platformLogsRoutingSinkHasMarker(sink *consumertest.LogsSink, marker string) bool {
	_, found := findAttributesForLog(sink, marker)
	return found
}
