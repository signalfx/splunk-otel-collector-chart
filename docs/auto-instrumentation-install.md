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

<details>
<summary>Expand for extended Instrumentation configuration details</summary>

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
- For proper ingestion of trace telemetry data, the `deployment.environment` attribute must be present in the exported traces. There are two ways to set this attribute:
  - Use the optional `environment` configuration in `values.yaml`.
  - Use the Instrumentation spec (`operator.instrumentation.spec.env`) with the environment variable `OTEL_RESOURCE_ATTRIBUTES`.

</details>

```bash
# Check if cert-manager is already installed, don't deploy a second cert-manager.
kubectl get pods -l app=cert-manager --all-namespaces

# If cert-manager is not deployed, make sure to add certmanager.enabled=true to the list of values to set
helm install splunk-otel-collector -f ./my_values.yaml --set operator.enabled=true,environment=dev splunk-otel-collector-chart/splunk-otel-collector
```

### 2. Verify all the OpenTelemetry resources (collector, operator, webhook, instrumentation) are deployed successfully

<details>
<summary>Expand for sample output to verify against</summary>

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

</details>

### 3. Instrument application by setting an annotation

An _instrumentation.opentelemetry.io/inject-{instrumentation_library}_ annotation can be added to the following:
- Namespace: All pods within that namespace will be instrumented.
- Pod Spec Objects: PodSpec objects that are available as part of Deployment,
  Statefulset, or other resources can be annotated.
- Example annotations
  - `instrumentation.opentelemetry.io/inject-java: "true"`
  - `instrumentation.opentelemetry.io/inject-dotnet: "true"`
  - `instrumentation.opentelemetry.io/inject-nodejs: "true"`
  - `instrumentation.opentelemetry.io/inject-python: "true"`

The instrumentation annotations can have the following values:
- "true" - inject and Instrumentation resource from the namespace to use.
- "my-instrumentation" - name of Instrumentation CR instance in the current namespace to use.
- "my-other-namespace/my-instrumentation" - name and namespace of Instrumentation CR instance in another namespace to use.
- "false" - do not inject.

### 4. Check out the results at [Splunk Observability APM](https://app.us1.signalfx.com/#/apm)

The trace and metrics data should populate the APM dashboard.To better
visualize this example as a whole, we have also included an image
of what the APM dashboard should look like and a architecture diagram to show
how everything is set up.

## Learn by example

- [Java Spring Clinic Example](../examples/enable-operator-and-auto-instrumentation/README.md)

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

In the table below current instrumentation libraries are listed, if they are supported, and how compatible they are
with Splunk customer content.
_The native OpenTelemetry instrumentation libraries are owned and maintained by the OpenTelemetry Community, Splunk
provides best effort support with issues related to native OpenTelemetry instrumentation libraries._

| Instrumentation Library | Distribution  | Status      | Supported        | Splunk Content Compatability | Code Repo                                                                            | Image Repo                                                                     |
|-------------------------|---------------|-------------|------------------|------------------------------|--------------------------------------------------------------------------------------|--------------------------------------------------------------------------------|
| java                    | Splunk        | Available   | Yes              | Completely                   | [Link](github.com/signalfx/splunk-otel-java)                                         | ghcr.io/signalfx/splunk-otel-java/splunk-otel-java                             |
| dotnet                  | Splunk        | Coming Soon |                  |                              | [Link](github.com/signalfx/splunk-otel-dotnet)                                       |                                                                                |
| nodejs                  | Splunk        | Coming Soon |                  |                              | [Link](github.com/signalfx/splunk-otel-nodejs)                                       |                                                                                |
| python                  | Splunk        | Coming Soon |                  |                              | [Link](github.com/signalfx/splunk-otel-python)                                       |                                                                                |
| java                    | OpenTelemetry | Available   | Yes              | Mostly                       | [Link](https://github.com/open-telemetry/opentelemetry-java-instrumentation)         | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-java         |
| dotnet                  | OpenTelemetry | Available   | Yes              | Mostly                       | [Link](https://github.com/open-telemetry/opentelemetry-dotnet-instrumentation)       | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-dotnet       |
| nodejs                  | OpenTelemetry | Available   | Needs Validation |                              | [Link](https://github.com/open-telemetry/opentelemetry-nodejs-instrumentation)       | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-nodes        |
| python                  | OpenTelemetry | Available   | Needs Validation |                              | [Link](https://github.com/open-telemetry/opentelemetry-java-instrumentation)         | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-java         |
| apache-httpd            | OpenTelemetry | Available   | Needs Validation |                              | [Link](https://github.com/open-telemetry/opentelemetry-apache-httpd-instrumentation) | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-apache-httpd |

</details>

### Documentation Resources

- https://developers.redhat.com/devnation/tech-talks/using-opentelemetry-on-kubernetes
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/docs/api.md#instrumentation
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md#opentelemetry-auto-instrumentation-injection
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md#use-customized-or-vendor-instrumentation
