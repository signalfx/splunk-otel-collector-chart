# Auto-instrumentation

The process of adding observability code to your application so that it produces telemetry data is known as
instrumentation. There are two main approaches to instrumentation in OpenTelemetry: auto-instrumentation and manual
instrumentation.

Auto-instrumentation

- Usually is the easiest way to setup instrumentation for applications
- OpentTelemetry leverages Kubernetes to implement auto-instrumentation in such a way that your applications can be
  instrumented not by updating the application source code but rather by utilizing Kubernetes resources.
- Supports a subset of instrumentation libraries compared to manual instrumentation

Manual Instrumentation

- Takes more setup effort compared to auto-instrumentation but offers more customizability
- Requires editing application source code to include pre-built OpenTelemetry instrumentation libraries
- Supports more instrumentation libraries and customization of the exported telemetry data compared to
  auto-instrumentation

In particular, auto-instrumentation is useful for applications that use widely popular frameworks and libraries, as
these frameworks often have pre-built instrumentation capabilities already available.

## Steps for setting up auto-instrumentation

### 1. Deploy the Helm Chart with the Operator enabled

Set `operator.enabled=true` when deploying the chart to enable deploying the operator as well.
If a cert-manager is not available in the cluster (or other TLS certificate source), then you'll need to deploy it
using `certmanager.enabled=true`. The cert-manager issues TLS certificates the operator requires. You can use the
commands below to run these steps.
- An [opentelemetry.io/v1alpha1 Instrumentation](https://github.com/open-telemetry/opentelemetry-operator/blob/main/docs/api.md#instrumentation)
object is used to configure the auto-instrumentation of your applications. To successfully enable instrumentation, the
target pod must have an Instrumentation object available.
- When `operator.enabled=true`, the helm chart deploys a
[default](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/examples/enable-operator-and-auto-instrumentation/rendered_manifests/operator/instrumentation.yaml)
opentelemetry.io/v1alpha1 Instrumentation object.
  - The default Instrumentation supports [AlwaysOn Profiling](https://docs.splunk.com/Observability/apm/profiling/intro-profiling.html) when `splunkObservability.profilingEnabled=true`.
    - These environment variables will be set when auto-instrumenting applications.
      - SPLUNK_PROFILER_ENABLED="true"
      - SPLUNK_PROFILER_MEMORY_ENABLED="true"
    - Example
      - [Enable always-on profiling](../examples/enable-operator-and-auto-instrumentation/instrumentation/instrumentation-enable-profiling.yaml)
  - Users can override the specifications of the default deployed instrumentation by setting override values under `operator.instrumentation.spec`.
    - Examples
      - [Add custom environment span tag](../examples/enable-operator-and-auto-instrumentation/instrumentation/instrumentation_add_custom_environment_span_tag.yaml)
      - [Add trace sampler](../examples/enable-operator-and-auto-instrumentation/instrumentation/instrumentation-add-trace-sampler.yaml)
      - [Enable always-on profiling partially](../examples/enable-operator-and-auto-instrumentation/instrumentation/instrumentation-enable-profiling-partially.yaml)
- To view a deployed Instrumentation, you can use the command: `kubectl get otelinst {instrumentation_name} -o yaml`.
- The `deployment.environment` attribute may be set in the exported traces to identify the [APM deployment environment](https://docs.splunk.com/observability/en/apm/set-up-apm/environments.html). There are two ways to set this attribute:
  - Use the optional `environment` configuration in `values.yaml`.
  - Use the Instrumentation spec (`operator.instrumentation.spec.env`) with the environment variable `OTEL_RESOURCE_ATTRIBUTES`.

```bash
# Check if cert-manager is already installed, don't deploy a second cert-manager.
kubectl get pods -l app=cert-manager --all-namespaces

# If cert-manager is not deployed, make sure to add certmanager.enabled=true to the list of values to set
helm install splunk-otel-collector -f ./my_values.yaml --set operator.enabled=true,environment=dev splunk-otel-collector-chart/splunk-otel-collector
```

### 2. Verify all the OpenTelemetry resources (collector, operator, webhook, instrumentation) are deployed successfully

```bash
kubectl get pods
# NAME                                                            READY   STATUS
# splunk-otel-collector-agent-lfthw                               2/2     Running
# splunk-otel-collector-cert-manager-6b9fb8b95f-2lmv4             1/1     Running
# splunk-otel-collector-cert-manager-cainjector-6d65b6d4c-khcrc   1/1     Running
# splunk-otel-collector-cert-manager-webhook-87b7ffffc-xp4sr      1/1     Running
# splunk-otel-collector-k8s-cluster-receiver-856f5fbcf9-pqkwg     1/1     Running
# splunk-otel-collector-opentelemetry-operator-56c4ddb4db-zcjgh   2/2     Running

kubectl get mutatingwebhookconfiguration.admissionregistration.k8s.io
# NAME                                      WEBHOOKS   AGE
# splunk-otel-collector-cert-manager-webhook              1          14m
# splunk-otel-collector-opentelemetry-operator-mutation   3          14m

kubectl get otelinst
# NAME                    AGE   ENDPOINT
# splunk-otel-collector   3s    http://$(SPLUNK_OTEL_AGENT):4317
```

### 3. Instrument application by setting an annotation

Enable instrumentation by adding the `instrumentation.opentelemetry.io/inject-{instrumentation_library}` annotation.
This can be applied to a namespace for all its pods or to individual PodSpec objects, available as part of
Deployment, Statefulset, and other resources.

**Annotation Values:**
- `"true"`: Inject the `Instrumentation` resource from the namespace.
- `"my-instrumentation"`: Use the `Instrumentation` custom resource (CR) instance in the current namespace.
- `"my-other-namespace/my-instrumentation"`: Use the `Instrumentation` CR instance from another namespace.
- `"false"`: Do not inject.

**Annotations for Different Libraries:**

**Java:**

```yaml
instrumentation.opentelemetry.io/inject-java: "true"
```

**NodeJS:**

```yaml
instrumentation.opentelemetry.io/inject-nodejs: "true"
```

**Python:**

```yaml
instrumentation.opentelemetry.io/inject-python: "true"
```

**.NET:**
.NET auto-instrumentation uses annotations to set the .NET [Runtime Identifiers](https://learn.microsoft.com/en-us/dotnet/core/rid-catalog).
Current RIDs: `linux-x64` (default) and `linux-musl-x64`.

```yaml
instrumentation.opentelemetry.io/inject-dotnet: "true"
instrumentation.opentelemetry.io/otel-dotnet-auto-runtime: "linux-x64"
instrumentation.opentelemetry.io/otel-dotnet-auto-runtime: "linux-musl-x64"
```

**Go:**
Go auto-instrumentation requires `OTEL_GO_AUTO_TARGET_EXE`. Set via annotation or the Instrumentation resource.

```yaml
instrumentation.opentelemetry.io/inject-go: "true"
instrumentation.opentelemetry.io/otel-go-auto-target-exe: "/path/to/container/executable"
```
_Note: Elevated permissions are automatically set for Go auto-instrumentation._

**Apache HTTPD:**

```yaml
instrumentation.opentelemetry.io/inject-apache-httpd: "true"
```

**Nginx:**

```yaml
instrumentation.opentelemetry.io/inject-nginx: "true"
```

**OpenTelemetry SDK:**

```yaml
instrumentation.opentelemetry.io/inject-sdk: "true"
```

#### Annotation Examples:

**Example 1:**

For a nodejs application, with helm chart installed as:

```bash
helm install splunk-otel-collector --values ~/src/values/my_values.yaml ./helm-charts/splunk-otel-collector --namespace monitoring
```

_Note: The default `Instrumentation` object name matches the helm release name. The default instrumentation name for this example is `splunk-otel-collector`._

If the current namespace is `monitoring`:
- Use any of the following annotations:
  - `"instrumentation.opentelemetry.io/inject-nodejs": "true"`
  - `"instrumentation.opentelemetry.io/inject-nodejs": "splunk-otel-collector"`
  - `"instrumentation.opentelemetry.io/inject-nodejs": "monitoring/splunk-otel-collector"`

If the current namespace is not `monitoring`, like `default` or `my-other-namespace`:
- Use the annotation:
  - `"instrumentation.opentelemetry.io/inject-nodejs": "monitoring/splunk-otel-collector"`

**Example 2:**

For a nodejs application, with helm chart installed as:

```bash
helm install otel-collector --values ~/src/values/my_values.yaml ./helm-charts/splunk-otel-collector --namespace o11y
```

_Note: The default `Instrumentation` object name matches the helm release name. The default instrumentation name for this example is `otel-collector`._

If the current namespace is `o11y`:
- Use any of the following annotations:
  - `"instrumentation.opentelemetry.io/inject-nodejs": "true"`
  - `"instrumentation.opentelemetry.io/inject-nodejs": "otel-collector"`
  - `"instrumentation.opentelemetry.io/inject-nodejs": "o11y/otel-collector"`

If the current namespace is not `o11y`, like `default` or `my-other-namespace`:
- Use the annotation:
  - `"instrumentation.opentelemetry.io/inject-nodejs": "o11y/otel-collector"`

#### Multi-container pods with single instrumentation:

By default, the first container in the pod spec is instrumented. Specify containers with the
`instrumentation.opentelemetry.io/container-names` annotation.

**Example:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment-with-multiple-containers
spec:
  selector:
    matchLabels:
      app: my-pod-with-multiple-containers
  replicas: 1
  template:
    metadata:
      labels:
        app: my-pod-with-multiple-containers
      annotations:
        instrumentation.opentelemetry.io/inject-java: "true"
        instrumentation.opentelemetry.io/container-names: "myapp,myapp2"
```

#### Multi-container pods with multiple instrumentations:

This is for when `operator.autoinstrumentation.multi-instrumentation` is enabled. Specify containers
for each language using specific annotations like `instrumentation.opentelemetry.io/java-container-names`.

**Example:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment-with-multi-containers-multi-instrumentations
spec:
  selector:
    matchLabels:
      app: my-pod-with-multi-containers-multi-instrumentations
  replicas: 1
  template:
    metadata:
      labels:
        app: my-pod-with-multi-containers-multi-instrumentations
      annotations:
        instrumentation.opentelemetry.io/inject-java: "true"
        instrumentation.opentelemetry.io/java-container-names: "myapp,myapp2"
        instrumentation.opentelemetry.io/inject-python: "true"
        instrumentation.opentelemetry.io/python-container-names: "myapp3"
```

**NOTES:**
- Go auto-instrumentation **does not** support multi-container pods.
- A container cannot be instrumented with multiple languages.
- The `instrumentation.opentelemetry.io/container-names` annotation will be disregarded if a language container name annotation is set.

### 4. Check out the results at [Splunk Observability APM](https://app.us1.signalfx.com/#/apm)

The trace and metrics data should populate the APM dashboard.To better
visualize this example as a whole, we have also included an image
of what the APM dashboard should look like and a architecture diagram to show
how everything is set up.

## Learn by example

- [OpenTelemetry Operator and Auto-Instrumentation Example Guide](../examples/enable-operator-and-auto-instrumentation/README.md)

## How does auto-instrumentation work?

OpenTelemetry offers auto-instrumentation by using an
[operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
in a Kubernetes environment. An operator is a method of packaging, deploying, and managing Kubernetes applications.
In the context of setting up observability in a Kubernetes environment, an operator simplifies the management of
application auto-instrumentation, making it easier to gain valuable insights into application performance.

With this Splunk OTel Collector chart, the
[OpenTelemetry Operator](https://github.com/open-telemetry/opentelemetry-operator#opentelemetry-auto-instrumentation-injection)
can be deployed (by configuring `operator.enabled=true`) to your cluster and start auto-instrumenting your applications.
The chart and operator are two separate applications, but when used together they enable powerful telemetry data
related features for users.

The OpenTelemetry operator implement a
[MutatingAdmissionWebhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#mutatingadmissionwebhook)
that allows the operator to modify pod specs when a pod is created or updated. MutatingAdmissionWebhooks are
essentially part of the cluster control-plane, they functionally work by intercepting and modifying requests to the
Kubernetes API server after the request is authorized but before being persisted. MutatingAdmissionWebhooks are required
to be served via HTTPS, we use the Linux Foundation [cert-manager](https://cert-manager.io/docs/installation/kubectl/)
application to generate proper certificates.

For our Observability use case, the webhook modifies a pod to inject auto-instrumentation libraries into
the application container.

What does this really look like in practice? Let's describe the Java use case for this. I've deployed the chart with
everything needed to set up auto-instrumentation and I want to instrument my Java application.

- The operator auto-instruments my the application by injecting the Splunk OTel Java library
  (Javaagent) into the application container via an OpenTelemetry init container that copies the Javaagent into a volume
  that is mounted to the application container.
- The operator configures the SDK by injecting environment variables into the application container. The
  JAVA_TOOL_OPTIONS environment variable is used to set the JVM to use the injected Javaagent.

Below is a breakdown of the main and related components involved in auto-instrumentation:

<details>
<summary>Splunk OTel Collector Chart</summary>

- Description
  - A Helm chart used to deploy the collector and related resources.
  - The Splunk OTel Collector Chart is responsible for deploying the Splunk OTel collector (agent and gateway mode) and
    the OpenTelemetry Operator.

</details>

<details>
<summary>OpenTelemetry Operator</summary>

- Description
  - The OpenTelemetry Operator is responsible for setting up auto-instrumentation for Kubernetes pods.
  - The auto-instrumented applications are configured to send data to either a collector agent, collector gateway, or
    Splunk backend ingestion endpoint.
  - Has the capability to a particular kind of OpenTelemetry
    native [collectors](https://github.com/open-telemetry/opentelemetry-collector),
    however using this capability to manage the collectors deployed by the Splunk OTel Collector Chart is not supported.
  - Optionally deployed as a subchart located
    at: https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-operator
  - _The OpenTelemetry Operator Chart is owned and maintained by the OpenTelemetry Community, Splunk provides best
    effort support with issues related OpenTelemetry Operator Chart._
- Sub-components
  - Mutating Admission Webhook
- Dependencies
  - Cert Manager

</details>

<details>
<summary> Kubernetes Object - opentelemetry.io/v1alpha1 Instrumentation </summary>

- Description
  - A Kubernetes object used to configure auto-instrumentation settings for applications at the namespace or pod level.
- Documentation
  - https://github.com/open-telemetry/opentelemetry-operator/blob/main/docs/api.md#instrumentation

</details>

<details>
<summary>OpenTelemetry SDK</summary>

- Description
  - The SDK is the implementation of the OpenTelemetry API.
  - It's responsible for processing the telemetry data generated by the application and sending it to the configured
    backend.
  - The SDK typically consists of exporters, processors, and other components that handle the data before it is sent to
    the backend.
  - _The OpenTelemetry Operator is owned and maintained by the OpenTelemetry Community, Splunk provides best effort
    support with issues related to the native OpenTelemetry Operator._
- Documentation
  - [OpenTelemetry Tracing SDK](https://opentelemetry.io/docs/reference/specification/trace/sdk/)

</details>

<details>

<summary>OpenTelemetry Instrumentation Libraries</summary>

- Description:
  - OpenTelemetry auto-instrumentation still relies on instrumentation libraries for specific frameworks, libraries, or
    components of your application.
  - These libraries generate telemetry data when your application uses the instrumented components.
  - Splunk, OpenTelemetry, and other vendors produce instrumentation libraries to.
- Documentation
  - https://opentelemetry.io/docs/instrumentation/

</details>

### Instrumentation Libraries

The table below lists the current instrumentation libraries, their availability, and their compatibility with Splunk customer content.
_Note: Native OpenTelemetry instrumentation libraries are owned and maintained by the OpenTelemetry Community. Splunk provides best effort support for issues related to these libraries._

| Instrumentation Language | Distribution   | Feature Gate Name                                     | Feature Gate Default Value | Status         | Splunk Content Compatibility | Source URL                                                  | Image Repository                                                               |
|--------------------------|----------------|-------------------------------------------------------|----------------------------|----------------|------------------------------|-------------------------------------------------------------|--------------------------------------------------------------------------------|
| Java                     | Splunk         | `operator.autoinstrumentation.java`                   | Enabled                    | Stable         | Fully Compatible             | https://github.com/signalfx/splunk-otel-java                | ghcr.io/signalfx/splunk-otel-java/splunk-otel-java                             |
| NodeJS                   | Splunk         | `operator.autoinstrumentation.nodejs`                 | Enabled                    | Stable         | Fully Compatible             | https://github.com/signalfx/splunk-otel-js                  | ghcr.io/signalfx/splunk-otel-java/splunk-otel-js                               |
| DotNet                   | Splunk         | `operator.autoinstrumentation.dotnet`                 | Enabled                    | Stable         | Fully Compatible             | https://github.com/signalfx/splunk-otel-dotnet              | ghcr.io/signalfx/splunk-otel-java/splunk-otel-dotnet                           |
| ApacheHttpD              | OpenTelemetry  | `operator.autoinstrumentation.apache-httpd`           | Disabled                   | Experimental   | Partially Compatible         | https://github.com/open-telemetry/opentelemetry-cpp-contrib | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-apache-httpd |
| Go                       | OpenTelemetry  | `operator.autoinstrumentation.go`                     | Disabled                   | Experimental   | Partially Compatible         | https://github.com/open-telemetry/opentelemetry-go          | ghcr.io/open-telemetry/opentelemetry-go-instrumentation/autoinstrumentation-go |
| Nginx                    | OpenTelemetry  | `operator.autoinstrumentation.nginx`                  | Disabled                   | Experimental   | Partially Compatible         | https://github.com/open-telemetry/opentelemetry-cpp-contrib | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-apache-httpd |
| Python                   | Splunk         | `operator.autoinstrumentation.python`                 | Disabled                   | Experimental   | Partially Compatible         | https://github.com/open-telemetry/opentelemetry-python      | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-python       |

### Documentation Resources

- https://developers.redhat.com/devnation/tech-talks/using-opentelemetry-on-kubernetes
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/docs/api.md#instrumentation
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md#opentelemetry-auto-instrumentation-injection
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md#use-customized-or-vendor-instrumentation

### Troubleshooting the Operator and Cert Manager

#### 1. Check the logs for failures

**Operator Logs:**

```bash
kubectl logs -l app.kubernetes.io/name=operator
```

**Cert-Manager Logs:**

```bash
kubectl logs -l app=certmanager
kubectl logs -l app=cainjector
kubectl logs -l app=webhook
```

#### 2. Cert-Manager Issues

If the operator seems to be hanging, it could be due to the cert-manager not auto-creating the required certificate. To troubleshoot:

- Check the health and logs of the cert-manager pods for potential issues.
- Consider restarting the cert-manager pods.
- Ensure that your cluster has only one instance of cert-manager, which should include `certmanager`, `certmanager-cainjector`, and `certmanager-webhook`.

For additional guidance, refer to the official cert-manager documentation:
- [Troubleshooting Guide](https://cert-manager.io/docs/troubleshooting/)
- [Uninstallation Guide](https://cert-manager.io/v1.2-docs/installation/uninstall/kubernetes/)

#### 3. Validate Certificates

Ensure that the certificate, which the cert-manager creates and the operator utilizes, is available.

```bash
kubectl get certificates
# NAME                                          READY   SECRET                                                           AGE
# splunk-otel-collector-operator-serving-cert   True    splunk-otel-collector-operator-controller-manager-service-cert   5m
```

#### 4. Using a Self-Signed Certificate for the Webhook

The operator supports various methods for managing TLS certificates for the webhook. Below are the options available through the operator, with a brief description for each. For detailed configurations and specific use cases, please refer to the operator’s
[official Helm chart documentation](https://github.com/open-telemetry/opentelemetry-helm-charts/blob/main/charts/opentelemetry-operator/values.yaml).

1. **(Default Functionality) Use certManager to Generate a Self-Signed Certificate:**
  - Ensure that `operator.admissionWebhooks.certManager` is enabled.
  - By default, the OpenTelemetry Operator will use a self-signer issuer.
  - This option takes precedence over other options when enabled.
  - Specific issuer references and annotations can be provided as needed.

2. **Use Helm to Automatically Generate a Self-Signed Certificate:**
  - Ensure that `operator.admissionWebhooks.certManager` is disabled and `operator.admissionWebhooks.autoGenerateCert` is enabled.
  - When these conditions are met, Helm will automatically create a self-signed certificate and secret for you.

3. **Use Your Own Self-Signed Certificate:**
  - Ensure that both `operator.admissionWebhooks.certManager` and `operator.admissionWebhooks.autoGenerateCert` are disabled.
  - Provide paths to your own PEM-encoded certificate, private key, and CA cert.

**Note**: While using a self-signed certificate offers a quicker and simpler setup, it has limitations, such as not being trusted by default by clients.
This may be acceptable for testing purposes or internal environments. For complete configurations and additional guidance, please refer to the provided link to the Helm chart documentation.
