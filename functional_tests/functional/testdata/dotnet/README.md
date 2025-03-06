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
- Arm based machines can have issues running docker/dockerx build commands that use QEMU virtualization and have .NET support, see: https://github.com/dotnet/dotnet-docker/issues/3832
- On an arm M2 Mac running Docker Desktop 25.0.0, was able to get the docker/dockerx build for 1 arch images but not multi arch images.
  - New versions of the OS with Docker may fix this issue.
  - Dockerx related command:  `docker buildx create --use --name multi-arch-builder`

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
