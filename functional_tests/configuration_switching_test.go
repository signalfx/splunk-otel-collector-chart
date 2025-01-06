// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build configuration_switching

package functional_tests

import (
	"bytes"
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	hecReceiverPort            = 8090
	hecMetricsReceiverPort     = 8091
	apiPort                    = 8881
	hecLogsObjectsReceiverPort = 8092
	testDir                    = "testdata_configuration_switching"
	valuesDir                  = "values"
)

var globalSinks *sinks

var setupRun = sync.Once{}

type sinks struct {
	logsConsumer        *consumertest.LogsSink
	hecMetricsConsumer  *consumertest.MetricsSink
	logsObjectsConsumer *consumertest.LogsSink
}

func setupOnce(t *testing.T) *sinks {
	setupRun.Do(func() {
		// create an API server
		internal.CreateApiServer(t, apiPort)
		// set ingest pipelines
		logs, metrics := setupHEC(t)
		globalSinks = &sinks{
			logsConsumer:        logs,
			hecMetricsConsumer:  metrics,
			logsObjectsConsumer: setupHECLogsObjects(t),
		}
		if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
			teardown(t)
		}
	})
	return globalSinks
}

func deployChartsAndApps(t *testing.T, valuesFileName string, repl map[string]interface{}) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	chartPath := filepath.Join("..", "helm-charts", "splunk-otel-collector")
	chart, err := loader.Load(chartPath)
	require.NoError(t, err)

	var valuesBytes []byte
	valuesBytes, err = os.ReadFile(filepath.Join(testDir, valuesDir, valuesFileName))
	require.NoError(t, err)

	hostEp := hostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}
	replacements := map[string]interface{}{
		"LogHecEndpoint":    fmt.Sprintf("http://%s:%d", hostEp, hecReceiverPort),
		"MetricHecEndpoint": fmt.Sprintf("http://%s:%d/services/collector", hostEp, hecMetricsReceiverPort),
	}
	for k, v := range repl {
		replacements[k] = v
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

	waitForAllDeploymentsToStart(t, client)
	t.Log("Deployments started")

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		t.Log("Cleaning up cluster")
		teardown(t)

	})

}
func teardown(t *testing.T) {
	t.Log("Running teardown")
	uninstallDeployment(t)
}

func waitForAllDeploymentsToStart(t *testing.T, client *kubernetes.Clientset) {
	require.Eventually(t, func() bool {
		di, err := client.AppsV1().Deployments("default").List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)
		for _, d := range di.Items {
			if d.Status.ReadyReplicas != d.Status.Replicas {
				var messages string
				for _, c := range d.Status.Conditions {
					messages += c.Message
					messages += "\n"
				}

				t.Logf("Deployment not ready: %s, %s", d.Name, messages)
				return false
			}
		}
		return true
	}, 5*time.Minute, 10*time.Second)
}

func Test_Functions(t *testing.T) {
	_ = setupOnce(t)
	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}

	t.Run("agent logs and metrics enabled or disabled", testAgentLogsAndMetrics)
	t.Run("logs and metrics index switch", testIndexSwitch)
	t.Run("cluster receiver enabled or disabled", testClusterReceiverEnabledOrDisabled)

}

func testAgentLogsAndMetrics(t *testing.T) {

	valuesFileName := "values_logs_and_metrics_switching.yaml.tmpl"
	hecMetricsConsumer := setupOnce(t).hecMetricsConsumer
	agentLogsConsumer := setupOnce(t).logsConsumer

	t.Run("check logs and metrics received when both are enabled", func(t *testing.T) {
		resetLogsSink(t, agentLogsConsumer)
		resetMetricsSink(t, hecMetricsConsumer)

		checkNoMetricsReceived(t, hecMetricsConsumer)
		checkNoEventsReceived(t, agentLogsConsumer)

		replacements := map[string]interface{}{
			"MetricsEnabled": true,
			"LogsEnabled":    true,
		}
		deployChartsAndApps(t, valuesFileName, replacements)

		waitForMetrics(t, 5, hecMetricsConsumer)
		waitForLogs(t, 5, agentLogsConsumer)
		uninstallDeployment(t)
	})

	t.Run("check metrics only enabled", func(t *testing.T) {
		resetLogsSink(t, agentLogsConsumer)
		resetMetricsSink(t, hecMetricsConsumer)

		checkNoMetricsReceived(t, hecMetricsConsumer)
		checkNoEventsReceived(t, agentLogsConsumer)

		replacements := map[string]interface{}{
			"MetricsEnabled": true,
			"LogsEnabled":    false,
		}
		deployChartsAndApps(t, valuesFileName, replacements)

		waitForMetrics(t, 5, hecMetricsConsumer)
		checkNoEventsReceived(t, agentLogsConsumer)
		uninstallDeployment(t)
	})

	t.Run("check logs only enabled", func(t *testing.T) {
		resetLogsSink(t, agentLogsConsumer)
		resetMetricsSink(t, hecMetricsConsumer)

		replacements := map[string]interface{}{
			"MetricsEnabled": false,
			"LogsEnabled":    true,
		}
		deployChartsAndApps(t, valuesFileName, replacements)

		waitForLogs(t, 5, agentLogsConsumer)
		uninstallDeployment(t)
		resetLogsSink(t, agentLogsConsumer)
		resetMetricsSink(t, hecMetricsConsumer)
	})
}

func testIndexSwitch(t *testing.T) {
	var metricsIndex string = "metricsIndex"
	var newMetricsIndex string = "newMetricsIndex"
	var logsIndex string = "main"
	var newLogsIndex string = "newLogsIndex"
	var nonDefaultSourcetype = "my-sourcetype"

	valuesFileName := "values_indexes_switching.yaml.tmpl"
	hecMetricsConsumer := setupOnce(t).hecMetricsConsumer
	checkNoMetricsReceived(t, hecMetricsConsumer)
	agentLogsConsumer := setupOnce(t).logsConsumer
	checkNoEventsReceived(t, agentLogsConsumer)

	t.Run("check logs and metrics index switching", func(t *testing.T) {
		replacements := map[string]interface{}{
			"MetricsIndex": metricsIndex,
			"LogsIndex":    logsIndex,
		}
		deployChartsAndApps(t, valuesFileName, replacements)

		waitForMetrics(t, 3, hecMetricsConsumer)
		waitForLogs(t, 3, agentLogsConsumer)

		var sourcetypes []string
		var indices []string
		logs := agentLogsConsumer.AllLogs()
		sourcetypes, indices = getLogsIndexAndSourceType(logs)
		assert.True(t, len(sourcetypes) > 1) // we are also receiving logs from other kind containers
		assert.Contains(t, sourcetypes, "kube:container:kindnet-cni")
		assert.True(t, len(indices) == 1)
		assert.True(t, indices[0] == logsIndex)

		var mIndices []string
		mIndices = getMetricsIndex(hecMetricsConsumer.AllMetrics())
		assert.True(t, len(mIndices) == 1)
		assert.True(t, mIndices[0] == metricsIndex)

		replacements = map[string]interface{}{
			"MetricsIndex":         newMetricsIndex,
			"LogsIndex":            newLogsIndex,
			"NonDefaultSourcetype": true,
			"Sourcetype":           nonDefaultSourcetype,
		}
		deployChartsAndApps(t, valuesFileName, replacements)
		resetLogsSink(t, agentLogsConsumer)
		resetMetricsSink(t, hecMetricsConsumer)

		waitForMetrics(t, 3, hecMetricsConsumer)
		waitForLogs(t, 3, agentLogsConsumer)
		logs = agentLogsConsumer.AllLogs()
		sourcetypes, indices = getLogsIndexAndSourceType(logs)
		t.Logf("Indices: %v", indices)
		assert.Contains(t, indices, newLogsIndex)
		assert.Contains(t, sourcetypes, nonDefaultSourcetype)
		assert.True(t, len(indices) == 1)
		assert.True(t, len(sourcetypes) == 1)
		mIndices = getMetricsIndex(hecMetricsConsumer.AllMetrics())
		assert.True(t, len(mIndices) == 1)
		assert.True(t, mIndices[0] == newMetricsIndex)
	})
	uninstallDeployment(t)
	resetLogsSink(t, agentLogsConsumer)
	resetMetricsSink(t, hecMetricsConsumer)
}

func testClusterReceiverEnabledOrDisabled(t *testing.T) {
	valuesFileName := "values_cluster_receiver_switching.yaml.tmpl"
	namespace := "default"
	logsObjectsConsumer := setupOnce(t).logsObjectsConsumer
	hostEp := hostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}
	logsObjectsHecEndpoint := fmt.Sprintf("http://%s:%d/services/collector", hostEp, hecLogsObjectsReceiverPort)

	t.Run("check cluster receiver enabled", func(t *testing.T) {
		replacements := map[string]interface{}{
			"ClusterReceiverEnabled": false,
			"LogObjectsHecEndpoint":  logsObjectsHecEndpoint,
		}
		deployChartsAndApps(t, valuesFileName, replacements)
		var pods *corev1.PodList
		pods = listPodsInNamespace(t, namespace)
		assert.True(t, len(pods.Items) == 1)
		assert.True(t, strings.HasPrefix(pods.Items[0].Name, "sock-splunk-otel-collector-agent"))
		checkNoEventsReceived(t, logsObjectsConsumer)

		t.Log("cluster receiver enabled")
		replacements = map[string]interface{}{
			"ClusterReceiverEnabled": true,
			"LogObjectsHecEndpoint":  logsObjectsHecEndpoint,
		}
		deployChartsAndApps(t, valuesFileName, replacements)
		resetLogsSink(t, logsObjectsConsumer)

		pods = listPodsInNamespace(t, namespace)
		assert.True(t, len(pods.Items) == 2)
		assert.True(t, checkPodExists(pods, "sock-splunk-otel-collector-agent"))
		assert.True(t, checkPodExists(pods, "sock-splunk-otel-collector-k8s-cluster-receiver"))
		waitForLogs(t, 5, logsObjectsConsumer)
	})
	uninstallDeployment(t)
	resetLogsSink(t, logsObjectsConsumer)
}

func checkPodExists(pods *corev1.PodList, podNamePrefix string) bool {
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, podNamePrefix) {
			return true
		}
	}
	return false
}

func listPodsInNamespace(t *testing.T, namespace string) *corev1.PodList {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)

	// Get the list of pods in the specified namespace
	pods, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	t.Logf("There are %d pods in the namespace %q\n", len(pods.Items), namespace)
	return pods
}

func waitForAllPodsToBeRemoved(t *testing.T, namespace string) {
	timeoutMinutes := 2
	require.Eventuallyf(t, func() bool {
		return len(listPodsInNamespace(t, namespace).Items) == 0
	}, time.Duration(timeoutMinutes)*time.Minute, 5*time.Second, "There are still %d pods in the namespace", len(listPodsInNamespace(t, namespace).Items))
}

func getLogsIndexAndSourceType(logs []plog.Logs) ([]string, []string) {
	var sourcetypes []string
	var indices []string

	for i := 0; i < len(logs); i++ {
		l := logs[i]
		for j := 0; j < l.ResourceLogs().Len(); j++ {
			rl := l.ResourceLogs().At(j)
			if value, ok := rl.Resource().Attributes().Get("com.splunk.sourcetype"); ok {
				sourcetype := value.AsString()
				// check if sourcetype is already in the list
				if !contains(sourcetypes, sourcetype) {
					sourcetypes = append(sourcetypes, sourcetype)
				}
			}
			if value, ok := rl.Resource().Attributes().Get("com.splunk.index"); ok {
				index := value.AsString()
				// check if index is already in the list
				if !contains(indices, index) {
					indices = append(indices, index)
				}
			}
		}
	}
	return sourcetypes, indices
}

// get metrics index from metrics
func getMetricsIndex(metrics []pmetric.Metrics) []string {
	var indices []string
	for i := 0; i < len(metrics); i++ {
		m := metrics[i]
		fmt.Printf("Metrics: %v", m.ResourceMetrics().At(0).Resource().Attributes())
		if value, ok := m.ResourceMetrics().At(0).Resource().Attributes().Get("com.splunk.index"); ok {
			index := value.AsString()
			if !contains(indices, index) {
				indices = append(indices, index)
			}
		}
	}
	return indices
}

func contains(list []string, newValue string) bool {
	for _, v := range list {
		if v == newValue {
			return true
		}
	}
	return false
}

func uninstallDeployment(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format+"\n", v...)
	}); err != nil {
		require.NoError(t, err)
	}

	uninstall := action.NewUninstall(actionConfig)
	uninstallResponse, err := uninstall.Run("sock")
	if err != nil {
		t.Logf("Failed to uninstall release: %v", err)
	}
	t.Logf("Uninstalled release: %v", uninstallResponse)
	waitForAllPodsToBeRemoved(t, "default")
}

func setupHEC(t *testing.T) (*consumertest.LogsSink, *consumertest.MetricsSink) {
	// the splunkhecreceiver does poorly at receiving logs and metrics. Use separate ports for now.
	f := splunkhecreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*splunkhecreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", hecReceiverPort)

	mCfg := f.CreateDefaultConfig().(*splunkhecreceiver.Config)
	mCfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", hecMetricsReceiverPort)

	lc := new(consumertest.LogsSink)
	mc := new(consumertest.MetricsSink)
	rcvr, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(), cfg, lc)
	mrcvr, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(), mCfg, mc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating logs receiver")
	t.Cleanup(func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	})

	require.NoError(t, mrcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating metrics receiver")
	t.Cleanup(func() {
		assert.NoError(t, mrcvr.Shutdown(context.Background()))
	})

	return lc, mc
}

func setupHECLogsObjects(t *testing.T) *consumertest.LogsSink {
	f := splunkhecreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*splunkhecreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", hecLogsObjectsReceiverPort)

	lc := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(), cfg, lc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating logs receiver")
	t.Cleanup(func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	})

	return lc
}
