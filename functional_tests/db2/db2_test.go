// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package db2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	testDir   = "testdata"
	valuesDir = "values"

	// Rendered by the subchart from the release name; see assertDerivedEndpoint.
	db2ConfigMapSuffix = "-db2-config"

	// Matches the startupProbe budget in testdata/db2/deployment.yaml, plus slack for a
	// cold image pull. A ceiling, not an expectation.
	//
	// This plus the Helm, readiness and metric waits and the teardown must fit inside
	// TEST_TIMEOUT, which go test also charges cleanup against.
	db2ReadyTimeout = 15 * time.Minute

	// Dedicated so it collides with neither the OTLP sinks (4317/4318) nor the secureapp
	// logs sink (4320).
	dbMonSinkPort = 4321
)

// Test_Db2_Metrics_Through_Collector covers the contract between the two charts: a real
// Db2, a receiver deployed by the subchart, and metrics arriving at a sink through the
// Collector this release creates.
//
// db2.otlp.endpoint is left unset on purpose — the subchart deriving it from the parent
// release name is the behavior under test.
func Test_Db2_Metrics_Through_Collector(t *testing.T) {
	kubeconfig, ok := os.LookupEnv("KUBECONFIG")
	require.True(t, ok, "KUBECONFIG must be set")

	client := k8sClient(t, kubeconfig)

	if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
		internal.ChartUninstall(t, kubeconfig)
		// The fixture is created directly rather than by the chart, so uninstalling the
		// release does not remove it. Left behind by SKIP_TEARDOWN or an interrupted run,
		// it makes the next run fail on AlreadyExists.
		deleteDb2Fixture(t, client)
	}

	// Db2 first, so a failure reads as "Db2 did not come up" rather than "the wiring
	// broke". The receiver would retry, but that only lengthens the run.
	deployDb2(t, client)

	metricsSink := internal.SetupSignalfxReceiver(t, internal.SignalFxReceiverPort)

	// Database Monitoring events need a sink that serves /v3/event; the metrics sink is a
	// SignalFx receiver and cannot.
	dbmonSink := internal.SetupOTLPLogsSinkOnPort(t, dbMonSinkPort, "/v3/event")

	valuesFile, err := filepath.Abs(filepath.Join(testDir, valuesDir, "db2_values.yaml.tmpl"))
	require.NoError(t, err)

	// Registered before the install so a failure during it, or during the readiness
	// checks below, still tears the release down.
	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		internal.ChartUninstall(t, kubeconfig)
	})

	hostEp := internal.HostEndpoint(t)
	internal.ChartInstallOrUpgrade(t, kubeconfig, valuesFile, map[string]any{
		"IngestURL": "http://" + internal.HostPort(hostEp, internal.SignalFxReceiverPort),
		"ApiURL":    "http://" + internal.HostPort(hostEp, internal.SignalFxAPIPort),
		"DbMonURL":  "http://" + internal.HostPort(hostEp, dbMonSinkPort),
	}, 0, internal.GetDefaultChartOptions())

	// A crash here would mean the chart produced an unusable Deployment or a config the
	// receiver cannot parse.
	internal.CheckPodsReady(t, client, internal.DefaultNamespace,
		"app.kubernetes.io/name=db2", 5*time.Minute, 10*time.Second)

	// A Collector rejects an invalid routing condition or exporter at startup, so the
	// gateway reaching Ready is what shows the rendered wiring is loadable.
	internal.CheckPodsReady(t, client, internal.DefaultNamespace,
		"app=splunk-otel-collector,component=otel-collector", 5*time.Minute, 10*time.Second)

	assertDerivedEndpoint(t, client)
	assertDbMonRouting(t, client)

	// The receiver scrapes on an interval and the Collector batches, so allow several
	// cycles.
	internal.AssertMetricNames(t, metricsSink,
		filepath.Join(testDir, "expected_db2_metric_names.yaml"), 5*time.Minute, 5*time.Second)

	assertDb2Dimensions(t, metricsSink)
	assertDbMonEventsDelivered(t, dbmonSink)
}

// assertDb2Dimensions checks the attribution that survives the trip to the sink. The
// receiver can scrape several remote databases, so a metric is only useful if it still
// names the one it came from.
//
// Dimensions come from the data points rather than the resource: the chart exports through
// the signalfx exporter, which carries them per data point.
//
// server.address is asserted; host.name is only logged, because the agent metrics pipeline
// runs resource_detection with override: true and may replace it.
func assertDb2Dimensions(t *testing.T, sink *consumertest.MetricsSink) {
	t.Helper()

	const wantHost = "db2.default.svc.cluster.local"

	seen := map[string]map[string]struct{}{}
	var checked int

	for _, batch := range sink.AllMetrics() {
		for i := 0; i < batch.ResourceMetrics().Len(); i++ {
			rm := batch.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				for k := 0; k < sm.Metrics().Len(); k++ {
					m := sm.Metrics().At(k)
					if !strings.HasPrefix(m.Name(), "db2.") {
						continue
					}
					for _, attrs := range dataPointAttributes(m) {
						attrs.Range(func(key string, v pcommon.Value) bool {
							if seen[key] == nil {
								seen[key] = map[string]struct{}{}
							}
							seen[key][v.AsString()] = struct{}{}
							return true
						})
						checked++
					}
				}
			}
		}
	}

	require.NotZero(t, checked, "no Db2 data points reached the sink to inspect")
	// "host" is logged alongside "host.name" in case anything in the SignalFx path renames
	// it.
	for _, key := range []string{"server.address", "host.name", "host", "db.namespace", "server.port"} {
		t.Logf("dimension %s as received: %v", key, keysOf(seen[key]))
	}

	require.Contains(t, seen, "server.address",
		"Db2 data points reached the sink without a server.address dimension")
	require.Contains(t, seen["server.address"], wantHost,
		"server.address must still name the database the metrics came from")
}

// dataPointAttributes returns the attribute maps of a metric's data points, whichever shape
// the metric has.
func dataPointAttributes(m pmetric.Metric) []pcommon.Map {
	var out []pcommon.Map
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < m.Gauge().DataPoints().Len(); i++ {
			out = append(out, m.Gauge().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < m.Sum().DataPoints().Len(); i++ {
			out = append(out, m.Sum().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		for i := 0; i < m.Histogram().DataPoints().Len(); i++ {
			out = append(out, m.Histogram().DataPoints().At(i).Attributes())
		}
	}
	return out
}

func keysOf(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// assertDbMonRouting checks the gateway config the Collector loaded routes query events to
// their own exporter, and keeps them off the shared entity exporter.
func assertDbMonRouting(t *testing.T, client *kubernetes.Clientset) {
	t.Helper()

	cms, err := client.CoreV1().ConfigMaps(internal.DefaultNamespace).
		List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)

	var relay string
	for _, cm := range cms.Items {
		if strings.HasSuffix(cm.Name, "-otel-collector") {
			relay = cm.Data["relay"]
			break
		}
	}
	require.NotEmpty(t, relay, "no gateway ConfigMap was created")

	var cfg struct {
		Exporters  map[string]map[string]any `yaml:"exporters"`
		Connectors struct {
			RoutingLogs struct {
				Table []struct {
					Condition string   `yaml:"condition"`
					Pipelines []string `yaml:"pipelines"`
				} `yaml:"table"`
			} `yaml:"routing/logs"`
		} `yaml:"connectors"`
		Service struct {
			Pipelines map[string]struct {
				Receivers []string `yaml:"receivers"`
				Exporters []string `yaml:"exporters"`
			} `yaml:"pipelines"`
		} `yaml:"service"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(relay), &cfg),
		"the gateway relay config is not parseable YAML")

	dbmon, ok := cfg.Service.Pipelines["logs/dbmon"]
	require.True(t, ok, "dbMonitoringEnabled rendered no logs/dbmon pipeline")
	require.Contains(t, dbmon.Receivers, "routing/logs")
	require.Contains(t, dbmon.Exporters, "otlp_http/dbmon")

	headers, _ := cfg.Exporters["otlp_http/dbmon"]["headers"].(map[string]any)
	require.Equal(t, "dbmon", headers["X-Splunk-Instrumentation-Library"],
		"the dbmon exporter must carry the Database Monitoring library header")

	entities, ok := cfg.Exporters["otlp_http/entities"]
	require.True(t, ok, "no otlp_http/entities exporter in the gateway config")
	entityHeaders, _ := entities["headers"].(map[string]any)
	require.NotContains(t, entityHeaders, "X-Splunk-Instrumentation-Library",
		"the shared entity exporter must not carry the Database Monitoring header")

	// Query events are not entity-marked, so an entity route ahead of the Database
	// Monitoring one would leave them to the default route and the ordinary logs
	// destination.
	table := cfg.Connectors.RoutingLogs.Table
	dbmonAt, entitiesAt := -1, -1
	for i, r := range table {
		if slices.Contains(r.Pipelines, "logs/dbmon") && dbmonAt < 0 {
			dbmonAt = i
		}
		if slices.Contains(r.Pipelines, "logs/entities") && entitiesAt < 0 {
			entitiesAt = i
		}
	}
	require.GreaterOrEqual(t, dbmonAt, 0, "routing/logs has no route to logs/dbmon")
	if entitiesAt >= 0 {
		require.Less(t, dbmonAt, entitiesAt,
			"the Database Monitoring route must come before the entity route")
	}
}

// assertDbMonEventsDelivered waits for a Db2 query event to arrive at the Database
// Monitoring sink, which covers the whole path: the receiver emitted it, routing/logs sent
// it to logs/dbmon, and otlp_http/dbmon delivered it to /v3/event in a form the receiving
// end accepts.
func assertDbMonEventsDelivered(t *testing.T, sink *consumertest.LogsSink) {
	t.Helper()

	const wantScope = "github.com/signalfx/splunk-otel-db2"
	var scopes []string

	require.Eventually(t, func() bool {
		scopes = nil
		for _, batch := range sink.AllLogs() {
			for i := 0; i < batch.ResourceLogs().Len(); i++ {
				rl := batch.ResourceLogs().At(i)
				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					sl := rl.ScopeLogs().At(j)
					if sl.LogRecords().Len() == 0 {
						continue
					}
					scopes = append(scopes, sl.Scope().Name())
				}
			}
		}
		return slices.Contains(scopes, wantScope)
	}, 5*time.Minute, 10*time.Second,
		"no log record from scope %q reached the Database Monitoring sink; scopes seen: %v",
		wantScope, scopes)

	// Only the Db2 scope may be routed here; anything else is a routing fault.
	for _, got := range scopes {
		require.Equal(t, wantScope, got,
			"a record from scope %q reached the Database Monitoring sink", got)
	}
}

func deployDb2(t *testing.T, client *kubernetes.Clientset) {
	t.Helper()

	svc := decode[*corev1.Service](t, filepath.Join(testDir, "db2", "service.yaml"))
	services := client.CoreV1().Services(internal.DefaultNamespace)
	_, err := services.Create(t.Context(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	dep := decode[*appsv1.Deployment](t, filepath.Join(testDir, "db2", "deployment.yaml"))
	deployments := client.AppsV1().Deployments(internal.DefaultNamespace)
	_, err = deployments.Create(t.Context(), dep, metav1.CreateOptions{})
	require.NoError(t, err)

	// Registered before the readiness wait so it also runs when that wait is what failed.
	// The run archive is not always retrievable afterwards, so the reason the fixture did
	// not start has to reach the test output itself.
	t.Cleanup(func() {
		if t.Failed() {
			dumpDb2Fixture(t, client)
		}
	})

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			return
		}
		deleteDb2Fixture(t, client)
	})

	// The startupProbe opens a real connection, so this returns when SAMPLE is usable
	// rather than when the port is merely listening.
	internal.CheckPodsReady(t, client, internal.DefaultNamespace, "app=db2",
		db2ReadyTimeout, 15*time.Second)
}

// dumpDb2Fixture reports why the Db2 Pod is not usable: its phase and container states,
// the Events the scheduler and kubelet recorded, and the tail of its log.
func dumpDb2Fixture(t *testing.T, client *kubernetes.Clientset) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 60*time.Second)
	defer cancel()

	pods, err := client.CoreV1().Pods(internal.DefaultNamespace).
		List(ctx, metav1.ListOptions{LabelSelector: "app=db2"})
	if err != nil {
		t.Logf("db2 diagnostics: listing pods failed: %v", err)
		return
	}
	if len(pods.Items) == 0 {
		t.Log("db2 diagnostics: no pod matched app=db2")
	}

	for _, pod := range pods.Items {
		t.Logf("db2 diagnostics: pod %s phase=%s node=%s", pod.Name, pod.Status.Phase, pod.Spec.NodeName)
		for _, c := range pod.Status.Conditions {
			t.Logf("  condition %s=%s %s %s", c.Type, c.Status, c.Reason, c.Message)
		}
		for _, cs := range pod.Status.ContainerStatuses {
			t.Logf("  container %s ready=%t restarts=%d state=%+v",
				cs.Name, cs.Ready, cs.RestartCount, cs.State)
		}

		events, eventsErr := client.CoreV1().Events(internal.DefaultNamespace).List(ctx, metav1.ListOptions{
			FieldSelector: "involvedObject.name=" + pod.Name,
		})
		if eventsErr != nil {
			t.Logf("  events unavailable: %v", eventsErr)
		}
		for _, e := range events.Items {
			t.Logf("  event %s/%s x%d: %s", e.Type, e.Reason, e.Count, e.Message)
		}

		tail := int64(60)
		logs, logsErr := client.CoreV1().Pods(internal.DefaultNamespace).
			GetLogs(pod.Name, &corev1.PodLogOptions{TailLines: &tail}).DoRaw(ctx)
		if logsErr != nil {
			t.Logf("  logs unavailable: %v", logsErr)
			continue
		}
		t.Logf("  last %d log lines:\n%s", tail, string(logs))
	}
}

// deleteDb2Fixture removes the Service and Deployment created by deployDb2. Shared by the
// pre-test teardown and the cleanup path so both remove the same objects.
func deleteDb2Fixture(t *testing.T, client *kubernetes.Clientset) {
	t.Helper()

	grace := int64(0)
	// t.Context is canceled before cleanup runs.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 30*time.Second)
	defer cancel()

	_ = client.AppsV1().Deployments(internal.DefaultNamespace).
		Delete(ctx, "db2", metav1.DeleteOptions{GracePeriodSeconds: &grace})
	_ = client.CoreV1().Services(internal.DefaultNamespace).
		Delete(ctx, "db2", metav1.DeleteOptions{})
}

// decode reads a single-document manifest and asserts its kind, so a mistake in testdata
// fails here rather than as a nil later.
func decode[T any](t *testing.T, path string) T {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(raw, nil, nil)
	require.NoError(t, err)
	typed, ok := obj.(T)
	require.True(t, ok, "%s decoded to %T, not the expected type", path, obj)
	return typed
}

// assertDerivedEndpoint checks the derived address against the API, not as a string: a
// string comparison passes just as happily against a Service that does not exist.
func assertDerivedEndpoint(t *testing.T, client *kubernetes.Clientset) {
	t.Helper()

	cms, err := client.CoreV1().ConfigMaps(internal.DefaultNamespace).
		List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)

	var config string
	for _, cm := range cms.Items {
		if strings.HasSuffix(cm.Name, db2ConfigMapSuffix) {
			config = cm.Data["config.yaml"]
			break
		}
	}
	require.NotEmpty(t, config, "no ConfigMap ending in %q was created", db2ConfigMapSuffix)

	host, port := otlpTarget(t, config)

	svc, err := client.CoreV1().Services(internal.DefaultNamespace).
		Get(t.Context(), host, metav1.GetOptions{})
	require.NoError(t, err,
		"the subchart derived %q, but this release created no such Service - the receiver "+
			"would export into a name that does not resolve", host)

	var ports []int32
	for _, p := range svc.Spec.Ports {
		ports = append(ports, p.Port)
	}
	require.Contains(t, ports, port,
		"Service %q exists but does not expose port %d", host, port)
}

// otlpTarget reduces the rendered exporter endpoint to a Service name and port.
func otlpTarget(t *testing.T, config string) (string, int32) {
	t.Helper()

	const key = "endpoint:"
	idx := strings.Index(config, key)
	require.NotEqual(t, -1, idx, "no exporter endpoint in the rendered config:\n%s", config)

	line := strings.TrimSpace(config[idx+len(key):])
	if nl := strings.IndexByte(line, '\n'); nl != -1 {
		line = line[:nl]
	}
	endpoint := strings.Trim(strings.TrimSpace(line), `"`)
	endpoint = strings.TrimPrefix(endpoint, "http://")

	hostPort := strings.SplitN(endpoint, ":", 2)
	require.Len(t, hostPort, 2, "endpoint %q has no port", endpoint)

	var port int32
	_, err := fmt.Sscanf(hostPort[1], "%d", &port)
	require.NoError(t, err, "endpoint %q has a non-numeric port", endpoint)

	// Derived as a cluster FQDN; the Service object is named by its first label.
	return strings.SplitN(hostPort[0], ".", 2)[0], port
}

func k8sClient(t *testing.T, kubeconfig string) *kubernetes.Clientset {
	t.Helper()
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)
	return client
}
