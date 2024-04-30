# TODO: Review all changes to this file, remove bloat
# .Net test image

This image is used for testing the auto-instrumentation of .Net application through the OpenTelemetry Operator.

This image is pushed to https://quay.io/repository/splunko11ytest/dotnet_test.

The container performs two separate functions:
* It runs a .Net HTTP server on port 3000 of the container host.
* It runs HTTP requests against the server every second.

Running this container inside a Kubernetes cluster under observation of the operator therefore creates traces.

## Develop

Login to quay.io and push with `make push`
Make sure for new image repositories you make the repository public
- Arm based machines can have issues running dockerx build commands with this test image due to QEMU and .NET support, see: https://github.com/dotnet/dotnet-docker/issues/3832
- On an M2 Mac running Docker Desktop 25.0.0, was able to get the dockerx build to run after setting `docker buildx create --use --name multi-arch-builder`

## Creating a test and golden file

In functional_test_v2.yaml add the following lines

	// .NET test app
	stream, err = os.ReadFile(filepath.Join(testDir, "dotnet", "deployment.yaml"))
	require.NoError(t, err)
	deployment, _, err = decode(stream, nil, nil)
	require.NoError(t, err)
	_, err = deployments.Create(context.Background(), deployment.(*appsv1.Deployment), metav1.CreateOptions{})
	if err != nil {
		_, err2 := deployments.Update(context.Background(), deployment.(*appsv1.Deployment), metav1.UpdateOptions{})
		assert.NoError(t, err2)
		if err2 != nil {
			require.NoError(t, err)
		}
	}

---
	_ = deployments.Delete(context.Background(), "dotnet-test", metav1.DeleteOptions{
		GracePeriodSeconds: &waitTime,
	})
---
	t.Run(".NET traces captured", testDotNetTraces)
---

func testDotNetTraces(t *testing.T) {
tracesConsumer := setupOnce(t).tracesConsumer

	var expectedTraces ptrace.Traces
	expectedTracesFile := filepath.Join(testDir, expectedValuesDir, "expected_java_traces.yaml")
	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)

	waitForTraces(t, 10, tracesConsumer)
	var selectedTrace *ptrace.Traces

	require.Eventually(t, func() bool {
		for i := len(tracesConsumer.AllTraces()) - 1; i > 0; i-- {
			trace := tracesConsumer.AllTraces()[i]
			if val, ok := trace.ResourceSpans().At(0).Resource().Attributes().Get("telemetry.sdk.language"); ok && strings.Contains(val.Str(), "dotnet") {
				if expectedTraces.SpanCount() == trace.SpanCount() {
					selectedTrace = &trace
					break
				}
			}
		}
		return selectedTrace != nil
	}, 3*time.Minute, 5*time.Second)
	golden.WriteTraces(t, "write_expected_dotnet_traces.yaml", *selectedTrace)

	//require.NotNil(t, selectedTrace)
	//
	//maskScopeVersion(*selectedTrace)
	//maskScopeVersion(expectedTraces)
	//
	//err = ptracetest.CompareTraces(expectedTraces, *selectedTrace,
	//	ptracetest.IgnoreResourceAttributeValue("os.description"),
	//	ptracetest.IgnoreResourceAttributeValue("process.pid"),
	//	ptracetest.IgnoreResourceAttributeValue("container.id"),
	//	ptracetest.IgnoreResourceAttributeValue("k8s.deployment.name"),
	//	ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
	//	ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
	//	ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
	//	ptracetest.IgnoreResourceAttributeValue("k8s.replicaset.name"),
	//	ptracetest.IgnoreResourceAttributeValue("os.version"),
	//	ptracetest.IgnoreResourceAttributeValue("host.arch"),
	//	ptracetest.IgnoreResourceAttributeValue("telemetry.sdk.version"),
	//	ptracetest.IgnoreResourceAttributeValue("telemetry.auto.version"),
	//	ptracetest.IgnoreResourceAttributeValue("splunk.distro.version"),
	//	ptracetest.IgnoreResourceAttributeValue("splunk.zc.method"),
	//	ptracetest.IgnoreSpanAttributeValue("net.sock.peer.port"),
	//	ptracetest.IgnoreSpanAttributeValue("thread.id"),
	//	ptracetest.IgnoreSpanAttributeValue("thread.name"),
	//	ptracetest.IgnoreSpanAttributeValue("os.version"),
	//	ptracetest.IgnoreTraceID(),
	//	ptracetest.IgnoreSpanID(),
	//	ptracetest.IgnoreStartTimestamp(),
	//	ptracetest.IgnoreEndTimestamp(),
	//	ptracetest.IgnoreResourceSpansOrder(),
	//	ptracetest.IgnoreScopeSpansOrder(),
	//)
	//
	//require.NoError(t, err)
}

### Draft: Github Workflow To publish test image

name: "Publish DotNet Auto-Instrumentation"

on:
push:
paths:
- 'functional_tests/testdata/dotnet/'
- 'functional_tests/testdata/dotnet/Dockerfile'
#    branches:
#      - main
pull_request:
paths:
- 'functional_tests/testdata/dotnet/'
- 'functional_tests/testdata/dotnet/Dockerfile'
workflow_dispatch:

concurrency:
group: ${{ github.workflow }}-${{ github.head_ref || github.run_id }}
cancel-in-progress: true

jobs:
publish:
runs-on: ubuntu-22.04

    steps:
      - uses: actions/checkout@v4

      - name: Read version
        # TODO: Choose versioning process
        # run: echo "VERSION=$(cat autoinstrumentation/dotnet/version.txt)" >> $GITHUB_ENV
        run: echo "VERSION=latest" >> $GITHUB_ENV

      - name: Docker meta
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: |
            jvsplk/dotnet_test
          tags: |
            type=match,pattern=v(.*),group=1,value=v${{ env.VERSION }}

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Cache Docker layers
        uses: actions/cache@v3
        with:
          path: /tmp/.buildx-cache
          key: ${{ runner.os }}-buildx-${{ github.sha }}
          restore-keys: |
            ${{ runner.os }}-buildx-

      - name: Log into Docker.io
        uses: docker/login-action@v3
        if: ${{ github.event_name == 'push' }}
        with:
          # Note: Using personal jvsplk repo for these values for now
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}

#      - name: Login to GitHub Package Registry
#        uses: docker/login-action@v3
#        if: ${{ github.event_name == 'push' }}
#        with:
#          registry: ghcr.io
#          username: ${{ github.repository_owner }}
#          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context:  functional_tests/testdata/dotnet
          platforms: linux/amd64
          # push: ${{ github.event_name == 'push' }}
          push: true
          build-args: version=${{ env.VERSION }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=local,src=/tmp/.buildx-cache
          cache-to: type=local,dest=/tmp/.buildx-cache
