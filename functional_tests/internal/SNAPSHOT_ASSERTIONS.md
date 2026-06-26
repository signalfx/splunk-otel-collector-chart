# pmetricassert snapshots

Use `internal.AssertMetricsSnapshot` for functional metric tests that only need
metric identity, not values or timestamps. Volatile attributes are passed as
keys and written as `<key>/exists: true`; patterned attributes can be written
as `<key>/regex: <pattern>`.

```go
assertionFile := filepath.Join(testDir, expectedValuesDir, "expected_my_metrics_assertion.yaml")
internal.AssertMetricsSnapshot(t, metricsSink, "target.metric", assertionFile,
    3*time.Minute, 10*time.Second,
    internal.WithVolatileAttributes(existsAttrs...),
    internal.WithRegexAttributes(regexAttrs))
```

Start with `internal.CommonK8sMetricAssertionExistsAttrs` and
`internal.CommonK8sMetricAssertionRegexAttrs`, then extend them near the test for
test-specific attributes.

If the old comparison used `pmetrictest.IgnoreSubsequentDataPoints`, pass those
metric names through `internal.WithFirstDatapointOnly(...)`.

If the old comparison used `pmetrictest.IgnoreScopeVersion`, pass
`internal.WithIgnoreScopeVersion()`.

To create or refresh a snapshot from an existing golden, add an entry to
`internal/assertiongen/gen_test.go`, then run:

```sh
cd functional_tests && GENERATE_ASSERTION=true go test ./internal/assertiongen -run TestGenerateAssertions -v
```

To refresh from a live functional run after an assertion mismatch:

```sh
cd functional_tests && UPDATE_EXPECTED_RESULTS=true go test ./functional -run 'Test_Functions/<subtest>' -count=1 -v
```

If a cluster-specific attribute appears, add it to that test's exists or regex
attrs before refreshing so the snapshot does not pin a generated value.
