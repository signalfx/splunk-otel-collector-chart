// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package k8sevents

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

var eventsLogsConsumer *consumertest.LogsSink

// Env vars to control the test behavior
// TEARDOWN_BEFORE_SETUP: if set to true, the test will run teardown before setup
// SKIP_SETUP: if set to true, the test will skip setup
// SKIP_TEARDOWN: if set to true, the test will skip teardown
// SKIP_TESTS: if set to true, the test will skip the test
// UPDATE_EXPECTED_RESULTS: if set to true, the test will update the expected results
// KUBECONFIG: the path to the kubeconfig file
func Test_K8SEvents(t *testing.T) {
	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		t.Log("Running teardown before setup as TEARDOWN_BEFORE_SETUP is set to true")
		testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
		require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
		k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
		require.NoError(t, err)
		teardown(t, k8sClient)
	}

	internal.SetupSignalFxAPIServer(t)

	eventsLogsConsumer = internal.SetupHECLogsSink(t)

	if os.Getenv("SKIP_SETUP") == "true" {
		t.Log("Skipping setup as SKIP_SETUP is set to true")
	} else {
		deployWorkloadAndCollector(t)
	}

	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	internal.WaitForLogs(t, 3, eventsLogsConsumer)

	t.Run("CheckK8SEventsLogs", func(t *testing.T) {
		actualLogs := selectResLogs("com.splunk.sourcetype", "kube:events", eventsLogsConsumer)
		k8sEventsLogs := selectLogs("k8s.namespace.name", "k8sevents-test", &actualLogs, func(body string) string {
			re := regexp.MustCompile(`Successfully pulled image "(busybox|alpine):latest" in .* \(.* including waiting\).*`)
			return re.ReplaceAllString(body, `Successfully pulled image "$1:latest" in <time> (<time> including waiting)`)
		})

		// the following attributes are added by the k8sattributes processor which might not be ready when the test runs
		removeFlakyLogRecordAttr(k8sEventsLogs, "container.id")
		removeFlakyLogRecordAttr(k8sEventsLogs, "container.image.name")
		removeFlakyLogRecordAttr(k8sEventsLogs, "container.image.tag")

		expectedEventsLogsFile := "testdata/expected_k8sevents.yaml"
		expectedEventsLogs, err := golden.ReadLogs(expectedEventsLogsFile)
		require.NoError(t, err, "failed to read expected events logs from file")

		err = plogtest.CompareLogs(expectedEventsLogs, k8sEventsLogs,
			plogtest.IgnoreTimestamp(),
			plogtest.IgnoreObservedTimestamp(),
			plogtest.IgnoreResourceAttributeValue("host.name"),
			plogtest.IgnoreLogRecordAttributeValue("k8s.object.uid"),
			plogtest.IgnoreLogRecordAttributeValue("k8s.statefulset.uid"),
			plogtest.IgnoreLogRecordAttributeValue("k8s.pod.uid"),
			plogtest.IgnoreLogRecordAttributeValue("k8s.object.resource_version"),
			plogtest.IgnoreResourceLogsOrder(),
			plogtest.IgnoreScopeLogsOrder(),
			plogtest.IgnoreLogRecordsOrder(),
		)
		if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
			internal.WriteNewExpectedLogsResult(t, expectedEventsLogsFile, &k8sEventsLogs)
		}
		require.NoError(t, err)
	})

	t.Run("CheckK8SObjectsLogs", func(t *testing.T) {
		k8sObjectsLogs := selectResLogs("com.splunk.sourcetype", "kube:object:*", eventsLogsConsumer)
		k8sObjectsLogs = updateLogRecordBody(k8sObjectsLogs, []string{"object", "metadata", "uid"}, "21d0b84b-f1ae-4ae4-959f-31d7581a272b")
		k8sObjectsLogs = updateLogRecordBody(k8sObjectsLogs, []string{"object", "metadata", "resourceVersion"}, "85980")
		k8sObjectsLogs = updateLogRecordBody(k8sObjectsLogs, []string{"object", "metadata", "creationTimestamp"}, "2025-03-04T01:59:10Z")
		k8sObjectsLogs = updateLogRecordBody(k8sObjectsLogs, []string{"object", "metadata", "managedFields", "0", "time"}, "2025-03-04T01:59:10Z")
		k8sObjectsLogs = updateLogRecordBody(k8sObjectsLogs, []string{"object", "metadata", "managedFields", "0", "manager"}, "k8sevents.test") // changes when the test name which runs k8s client changes

		// the following attributes are added by the k8sattributes processor which might not be ready when the test runs
		removeFlakyLogRecordAttr(k8sObjectsLogs, "container.image.name")
		removeFlakyLogRecordAttr(k8sObjectsLogs, "container.image.tag")
		removeFlakyLogRecordAttr(k8sObjectsLogs, "k8s.node.name")
		removeFlakyLogRecordAttr(k8sObjectsLogs, "k8s.pod.name")
		removeFlakyLogRecordAttr(k8sObjectsLogs, "k8s.pod.uid")

		expectedObjectsLogsFile := "testdata/expected_k8sobjects.yaml"
		expectedObjectsLogs, err := golden.ReadLogs(expectedObjectsLogsFile)
		require.NoError(t, err, "failed to read expected objects logs from file")

		err = plogtest.CompareLogs(expectedObjectsLogs, k8sObjectsLogs,
			plogtest.IgnoreTimestamp(),
			plogtest.IgnoreObservedTimestamp(),
			plogtest.IgnoreResourceAttributeValue("host.name"),
			plogtest.IgnoreLogRecordAttributeValue("k8s.object.uid"),
			plogtest.IgnoreLogRecordAttributeValue("k8s.object.resource_version"),
			plogtest.IgnoreResourceAttributeValue("com.splunk.index"), // this is flaky, the index can be the value from pod annotation due to the k8sattributes processor in the pipeline or main
			plogtest.IgnoreResourceLogsOrder(),
			plogtest.IgnoreScopeLogsOrder(),
			plogtest.IgnoreLogRecordsOrder(),
		)
		if err != nil && os.Getenv("UPDATE_EXPECTED_RESULTS") == "true" {
			internal.WriteNewExpectedLogsResult(t, expectedObjectsLogsFile, &k8sObjectsLogs)
		}
		require.NoError(t, err)
	})
}

func deployWorkloadAndCollector(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	valuesFile, err := filepath.Abs(filepath.Join("testdata", "k8sevents_values.yaml.tmpl"))
	require.NoError(t, err)
	hostEp := internal.HostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "host endpoint not found")
	}
	replacements := map[string]any{
		"ApiURL": fmt.Sprintf("http://%s:%d", hostEp, internal.SignalFxAPIPort),
		"LogURL": fmt.Sprintf("http://%s:%d", hostEp, internal.HECLogsReceiverPort),
	}
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements, 0)

	config, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	internal.CheckPodsReady(t, clientset, internal.Namespace, "component=otel-k8s-cluster-receiver", 3*time.Minute, 0)
	time.Sleep(30 * time.Second)

	// Deploy the workload
	internal.CreateNamespace(t, clientset, "k8sevents-test")
	internal.AnnotateNamespace(t, clientset, "k8sevents-test", "com.splunk.index", "index_from_namespace")
	createdObjs, err := k8stest.CreateObjects(k8sClient, "testdata/testobjects")
	require.NoError(t, err)
	require.NotEmpty(t, createdObjs)

	internal.CheckPodsReady(t, clientset, "k8sevents-test", "app=k8sevents-test", 2*time.Minute, 0)

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		teardown(t, k8sClient)
	})
}

func teardown(t *testing.T, k8sClient *k8stest.K8sClient) {
	testKubeConfig := os.Getenv("KUBECONFIG")
	internal.ChartUninstall(t, testKubeConfig)

	internal.DeleteObject(t, k8sClient, `
apiVersion: v1
kind: Namespace
metadata:
  name: k8sevents-test
`)
}

func selectResLogs(attributeName, attributeValue string, logSink *consumertest.LogsSink) plog.Logs {
	selectedLogs := plog.NewLogs()
	for _, logs := range logSink.AllLogs() {
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			resourceLogs := logs.ResourceLogs().At(i)
			attributes := resourceLogs.Resource().Attributes()
			if attr, ok := attributes.Get(attributeName); ok {
				if match, _ := regexp.MatchString(attributeValue, attr.Str()); match {
					resourceLogs.CopyTo(selectedLogs.ResourceLogs().AppendEmpty())
				}
			}
		}
	}
	return selectedLogs
}

func selectLogs(attributeName, attributeValue string, inLogs *plog.Logs, modifyBodyFunc func(string) string) plog.Logs {
	selectedLogs := plog.NewLogs()
	// collapse logs across resource logs into a single one to reduce flakiness in test runs
	for h := 0; h < inLogs.ResourceLogs().Len(); h++ {
		resourceLogs := inLogs.ResourceLogs().At(h)
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

func updateLogRecordBody(logs plog.Logs, path []string, newValue string) plog.Logs {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)
				body := logRecord.Body()
				if body.Type() == pcommon.ValueTypeMap {
					updateMap(body.Map(), path, newValue)
				}
			}
		}
	}
	return logs
}

func updateMap(m pcommon.Map, path []string, newValue string) {
	if len(path) == 0 {
		return
	}
	key := path[0]
	if len(path) == 1 {
		m.PutStr(key, newValue)
		return
	}
	if nestedValue, ok := m.Get(key); ok {
		switch nestedValue.Type() {
		case pcommon.ValueTypeMap:
			updateMap(nestedValue.Map(), path[1:], newValue)
		case pcommon.ValueTypeSlice:
			index, err := strconv.Atoi(path[1])
			if err != nil {
				fmt.Printf("updateMap: invalid index %s\n", path[1])
				return
			}
			if index < nestedValue.Slice().Len() {
				updateSlice(nestedValue.Slice(), path[2:], newValue, index)
			}
		default:
			fmt.Printf("updateMap: unexpected type %v\n", nestedValue.Type())
		}
	}
}

func updateSlice(s pcommon.Slice, path []string, newValue string, index int) {
	if len(path) == 0 {
		return
	}
	if len(path) == 1 {
		if s.At(index).Type() == pcommon.ValueTypeMap {
			s.At(index).Map().PutStr(path[0], newValue)
		}
		return
	}
	if s.At(index).Type() == pcommon.ValueTypeMap {
		updateMap(s.At(index).Map(), path[1:], newValue)
	}
}
