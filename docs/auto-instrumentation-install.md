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

```
# Check if cert-manager is already installed, don't deploy a second cert-manager.
kubectl get pods -l app=cert-manager --all-namespaces

# If cert-manager is not deployed.
helm install splunk-otel-collector -f ./my_values.yaml --set operator.enabled=true,certmanager.enabled=true -n monitoring helm-charts/splunk-otel-collector

# If cert-manager is already deployed.
helm install splunk-otel-collector -f ./my_values.yaml --set operator.enabled=true -n monitoring helm-charts/splunk-otel-collector
```

### 2. Deploy the opentelemetry.io/v1alpha1 Instrumentation

This Instrumentation object is a spec to configure what instrumentation libraries to use for instrumentation. An
Instrumentation object must be available to the target pod for auto-instrumentation to function. Here is an example
of a Instrumentation yaml file and how to install it.

<details open>
<summary>Expand for sample splunk-instrumentation.yaml</summary>

```yaml
# splunk-instrumentation.yaml
apiVersion: opentelemetry.io/v1alpha1
kind: Instrumentation
metadata:
  name: splunk-instrumentation
spec:
  exporter:
    endpoint: http://$(SPLUNK_OTEL_AGENT):4317
  propagators:
    - tracecontext
    - baggage
    - b3
  java:
    env:
      - name: SPLUNK_OTEL_AGENT
        valueFrom:
          fieldRef:
            apiVersion: v1
            fieldPath: status.hostIP
      - name: OTEL_EXPORTER_OTLP_ENDPOINT
        value: https://$(SPLUNK_OTEL_AGENT):4318
      - name: OTEL_TRACES_EXPORTER
        value: otlp
  dotnet:
    image: your-customized-auto-instrumentation-image:dotnet
    env:
      - name: OTEL_EXPORTER_OTLP_ENDPOINT
        value: http://{TARGET}:4317
  # nodjs:
  # python:
```

</details>

```
# Install
kubectl apply -f splunk-instrumentation.yaml
# Check the current deployed values
kubectl get otelinst -o yaml
```

### 3. Verify all the OpenTelemetry resources (collector, operator, webhook, instrumentation) are deployed successfully

<details open>
<summary>Expand for sample output to verify against</summary>

```
kubectl  get pods -n monitoring
# NAME                                                          READY
# NAMESPACE     NAME                                                            READY   STATUS
# monitoring    splunk-otel-collector-agent-lfthw                               2/2     Running
# monitoring    splunk-otel-collector-cert-manager-6b9fb8b95f-2lmv4             1/1     Running
# monitoring    splunk-otel-collector-cert-manager-cainjector-6d65b6d4c-khcrc   1/1     Running
# monitoring    splunk-otel-collector-cert-manager-webhook-87b7ffffc-xp4sr      1/1     Running
# monitoring    splunk-otel-collector-k8s-cluster-receiver-856f5fbcf9-pqkwg     1/1     Running
# monitoring    splunk-otel-collector-opentelemetry-operator-56c4ddb4db-zcjgh   2/2     Running

kubectl get mutatingwebhookconfiguration.admissionregistration.k8s.io -n monitoring
# NAME                                      WEBHOOKS   AGE
# splunk-otel-collector-cert-manager-webhook              1          14m
# splunk-otel-collector-opentelemetry-operator-mutation   3          14m

kubectl get otelinst -n spring-petclinic
# NAME                          AGE   ENDPOINT
# splunk-instrumentation        3m   http://$(SPLUNK_OTEL_AGENT):4317
```

</details>

### 4. Instrument application by setting an annotation

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

### 5. Check out the results at [Splunk Observability APM](https://app.us1.signalfx.com/#/apm)

The trace and metrics data should populate the APM dashboard.To better
visualize this example as a whole, we have also included a an image
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

<details open>
<summary>Splunk OTel Collector Chart</summary>

- Description
  - A Helm chart used to deploy the collector and related resources.
  - The Splunk OTel Collector Chart is responsible for deploying the Splunk OTel collector (agent and gateway mode) and
    the OpenTelemetry Operator.

</details>

<details open>
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

<details open>
<summary> Kubernetes Object - opentelemetry.io/v1alpha1 Instrumentation </summary>

- Description
  - A Kubernetes object used to configure auto-instrumentation settings for applications at the namespace or pod level.
- Documentation
  - https://github.com/open-telemetry/opentelemetry-operator/blob/main/docs/api.md#instrumentation

</details>

<details open>
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

<details open>

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
