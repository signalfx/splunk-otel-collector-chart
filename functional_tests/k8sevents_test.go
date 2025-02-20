// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build k8events

package functional_tests

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"
	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	apiPort               = 8881
	splunkHecReceiverPort = 8089
)

var setupRun = sync.Once{}
var eventsLogsConsumer *consumertest.LogsSink

// Env vars to control the test behavior
// TEARDOWN_BEFORE_SETUP: if set to true, the test will run teardown before setup
// SKIP_SETUP: if set to true, the test will skip setup
// SKIP_TEARDOWN: if set to true, the test will skip teardown
// SKIP_TESTS: if set to true, the test will skip the test
// UPDATE_EXPECTED_RESULTS: if set to true, the test will update the expected results
// KUBECONFIG: the path to the kubeconfig file

func Test_K8SEvents(t *testing.T) {
	eventsLogsConsumer := setup(t)
	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	selectedLogs := selectLogs(t, "k8s.namespace.name", "k8sevents-test", eventsLogsConsumer, func(body string) string {
		re := regexp.MustCompile(`Successfully pulled image "busybox:latest" in \d+ms \(\d+ms including waiting\)`)
		return re.ReplaceAllString(body, `Successfully pulled image "busybox:latest" in <time> (<time> including waiting)`)
	})

	removeFlakyLogRecordAttr(selectedLogs, "container.id") // flaky - k8sattribute processor might not have received the container id when the "started container" event is ingested

	expectedLogsFile := "testdata_k8sevents/expected_k8sevents.yaml"
	expectedLogs, err := golden.ReadLogs(expectedLogsFile)
	require.NoError(t, err, "failed to read expected logs from file")

	err = plogtest.CompareLogs(expectedLogs, selectedLogs,
		plogtest.IgnoreTimestamp(),
		plogtest.IgnoreObservedTimestamp(),
		plogtest.IgnoreResourceAttributeValue("host.name"),
		plogtest.IgnoreLogRecordAttributeValue("k8s.object.uid"),
		plogtest.IgnoreLogRecordAttributeValue("k8s.pod.uid"),
		plogtest.IgnoreLogRecordAttributeValue("k8s.object.resource_version"),
		plogtest.IgnoreResourceLogsOrder(),
		plogtest.IgnoreScopeLogsOrder(),
		plogtest.IgnoreLogRecordsOrder(),
	)
	if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
		writeNewExpectedLogsResult(t, expectedLogsFile, &selectedLogs)
	}
	require.NoError(t, err)
}

func setup(t *testing.T) *consumertest.LogsSink {
	setupRun.Do(func() {
		if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
			t.Log("Running teardown before setup as TEARDOWN_BEFORE_SETUP is set to true")
			testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
			require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
			k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
			require.NoError(t, err)
			teardown(t, k8sClient)
		}

		internal.CreateApiServer(t, apiPort)

		eventsLogsConsumer = setupHECLogsReceiver(t, splunkHecReceiverPort)

		if os.Getenv("SKIP_SETUP") == "true" {
			t.Log("Skipping setup as SKIP_SETUP is set to true")
			return
		}
		deployWorkloadAndCollector(t)
	})

	return eventsLogsConsumer
}

func deployWorkloadAndCollector(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	chartPath := filepath.Join("..", "helm-charts", "splunk-otel-collector")
	chart, err := loader.Load(chartPath)
	require.NoError(t, err)

	valuesBytes, err := os.ReadFile(filepath.Join("testdata_k8sevents", "k8sevents_values.yaml.tmpl"))
	require.NoError(t, err)

	hostEp := hostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "host endpoint not found")
	}

	// Deploy collector
	replacements := struct {
		ApiURL string
		LogURL string
	}{
		fmt.Sprintf("http://%s:%d", hostEp, apiPort),
		fmt.Sprintf("http://%s:%d", hostEp, splunkHecReceiverPort),
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

	config, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	checkPodsReady(t, clientset, "default", "component=otel-k8s-cluster-receiver", 3*time.Minute)
	time.Sleep(30 * time.Second)

	// Deploy the workload
	createNamespace(t, clientset, "k8sevents-test")
	createdObjs, err := k8stest.CreateObjects(k8sClient, "testdata_k8sevents/testobjects")
	require.NoError(t, err)
	require.NotEmpty(t, createdObjs)

	checkPodsReady(t, clientset, "k8sevents-test", "app=k8sevents-test", 2*time.Minute)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		teardown(t, k8sClient)
	})
}

func teardown(t *testing.T, k8sClient *k8stest.K8sClient) {
	actionConfig := new(action.Configuration)
	testKubeConfig := os.Getenv("KUBECONFIG")
	require.NoError(t, actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), t.Logf))
	uninstall := action.NewUninstall(actionConfig)
	_, err := uninstall.Run("sock")
	if err != nil {
		t.Logf("error during helm uninstall: %v\n", err)
	}

	deleteObject(t, k8sClient, `
apiVersion: v1
kind: Namespace
metadata:
  name: k8sevents-test
`)
}

func setupHECLogsReceiver(t *testing.T, port int) *consumertest.LogsSink {
	f := splunkhecreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*splunkhecreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", splunkHecReceiverPort)

	receiver := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(), cfg, receiver)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, rcvr.Shutdown(context.Background()))
	})

	return receiver
}

func deleteObject(t *testing.T, k8sClient *k8stest.K8sClient, objYAML string) {
	obj := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(objYAML), obj)
	require.NoError(t, err)
	k8stest.DeleteObject(k8sClient, obj)
}

func selectLogs(t *testing.T, attributeName, attributeValue string, logSink *consumertest.LogsSink, modifyBodyFunc func(string) string) plog.Logs {
	selectedLogs := plog.NewLogs()
	// collapse logs across resource logs into a single one to reduce flakiness in test runs
	for h := 0; h < len(logSink.AllLogs()); h++ {
		logs := logSink.AllLogs()[h]
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			resourceLogs := logs.ResourceLogs().At(i)
			var existingResLog plog.ResourceLogs
			foundResource := false
			for j := 0; j < selectedLogs.ResourceLogs().Len(); j++ {
				if compareAttributes(resourceLogs.Resource().Attributes(), selectedLogs.ResourceLogs().At(j).Resource().Attributes()) {
					existingResLog = selectedLogs.ResourceLogs().At(j)
					foundResource = true
					break
				}
			}
			if !foundResource {
				existingResLog = selectedLogs.ResourceLogs().AppendEmpty()
				resourceLogs.Resource().Attributes().CopyTo(existingResLog.Resource().Attributes())
			}
			for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
				scopeLogs := resourceLogs.ScopeLogs().At(j)
				var existingScopeLog plog.ScopeLogs
				foundScope := false
				for k := 0; k < existingResLog.ScopeLogs().Len(); k++ {
					if compareAttributes(scopeLogs.Scope().Attributes(), existingResLog.ScopeLogs().At(k).Scope().Attributes()) {
						existingScopeLog = existingResLog.ScopeLogs().At(k)
						foundScope = true
						break
					}
				}
				if !foundScope {
					existingScopeLog = existingResLog.ScopeLogs().AppendEmpty()
					scopeLogs.Scope().Attributes().CopyTo(existingScopeLog.Scope().Attributes())
				}
				for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
					logRecord := scopeLogs.LogRecords().At(k)
					attributes := logRecord.Attributes()
					if attr, ok := attributes.Get(attributeName); ok && attr.Str() == attributeValue {
						modifiedBody := modifyBodyFunc(logRecord.Body().Str())
						logRecord.Body().SetStr(modifiedBody)
						logRecord.CopyTo(existingScopeLog.LogRecords().AppendEmpty())
					}
				}
			}
		}
	}

	return selectedLogs
}

func compareAttributes(attr1, attr2 pcommon.Map) bool {
	if len(attr1.AsRaw()) != len(attr2.AsRaw()) {
		return false
	}
	for k1, v1 := range attr1.AsRaw() {
		if v2, ok := attr2.AsRaw()[k1]; !ok || v1 != v2 {
			return false
		}
	}
	return true
}

func removeFlakyLogRecordAttr(logs plog.Logs, attributeName string) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)
				logRecord.Attributes().Remove(attributeName)
			}
		}
	}
}
