# pmetricassert snapshots

Use `internal.AssertMetricsSnapshot` for functional metric tests that only need
metric identity, not values or timestamps. Volatile attributes are passed as
keys and written as `<key>/exists: true`; patterned attributes can be written
as `<key>/regex: <pattern>`. Scope versions can be matched by pattern with
`WithScopeVersionRegex`.

```go
assertionFile := filepath.Join(testDir, expectedValuesDir, "expected_my_metrics_assertion.yaml")
internal.AssertMetricsSnapshot(t, metricsSink, "target.metric", assertionFile,
    3*time.Minute, 10*time.Second,
    internal.WithVolatileAttributes(existsAttrs...),
    internal.WithRegexAttributes(regexAttrs))
```

Use `internal.CommonK8sMetricAssertionRegexAttrs` for shared Kubernetes
attribute patterns and add test-specific attributes near the test.

For metrics with additional live series, use `datapoints/include` in the YAML
snapshot to require the listed series and allow others. Use `datapoints/count`
when only the number of series matters. The test compares each full batch to
the snapshot without removing datapoints first.

To refresh from a live functional run after an assertion mismatch:

```sh
cd functional_tests && UPDATE_EXPECTED_RESULTS=true go test ./functional -run 'Test_Functions/<subtest>' -count=1 -v
```

If a cluster-specific attribute appears, add it to that test's exists or regex
attrs before refreshing so the snapshot does not pin a generated value.
The upstream writer emits exact collections for new snapshots. When refreshing
an existing snapshot, the helper preserves its `datapoints/include` and
`datapoints/count` constraints, including selected exact attribute values. It
checks the refreshed assertion against the selected live batch before replacing
the file.

For checks that only require metric names across received batches, use
`internal.AssertMetricNames`. Its optional filter selects which live metrics
contribute names. The assertion file uses canonical gauge metrics so the test
does not constrain the live metric type, resource, scope, datapoints, or values.
