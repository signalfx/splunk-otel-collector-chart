# .Net test image

This image is used for testing the auto-instrumentation of .Net application through the OpenTelemetry Operator.

This image is built locally by the chart repository's Kind functional-test
setup.

The container performs two separate functions:
* It runs a .Net HTTP server on port 3000 of the container host.
* It runs HTTP requests against the server every second.

Running this container inside a Kubernetes cluster under observation of the operator therefore creates traces.

## Develop

Build this image directly with `make -C functional_tests/functional/testdata/dotnet build`.
Set `PLATFORM=linux/arm64` when building for an ARM64 local Kind cluster. If
the .NET build or runtime has ARM64 limitations, use the default
`PLATFORM=linux/amd64` and run it with Docker's emulation support.

### Debugging .NET

These env vars can be set to help debug .NET instrumentation

```yaml
env:
  - name: OTEL_LOG_LEVEL
    value: DEBUG
  - name: OTEL_DOTNET_AUTO_TRACES_CONSOLE_EXPORTER_ENABLED
    value: "true"
```

### Current issues and workarounds

#### Rule Engine Failure - OTL-2843

An env var may be needed to bypass an error thrown by auto-instrumentation.

Error:

```
[Error] Error in StartupHook initialization: LoaderFolderLocation: /otel-auto-instrumentation-dotnet/net
Exception: Rule Engine Failure: One or more rules failed validation. Automatic Instrumentation won't be loaded.
System.Exception: Rule Engine Failure: One or more rules failed validation. Automatic Instrumentation won't be loaded.
    at StartupHook.Initialize() in /_/src/OpenTelemetry.AutoInstrumentation.StartupHook/StartupHook.cs:line 34
```

Env var:

```yaml
env:
  - name: OTEL_DOTNET_AUTO_RULE_ENGINE_ENABLED
    value: "false"
```
