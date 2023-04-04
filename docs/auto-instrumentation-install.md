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

## Getting started with auto-instrumentation

### 1. If the cert-manager available in the cluster, deploy it

The cert-manager adds certificates and certificate issuers as resource types in Kubernetes clusters, and simplifies the
process of obtaining, renewing and using those certificates. You can use the following make commands to deploy the
cert-manager.

```
make cert-manager

# If make is not availabe, you can use these commands.
kubectl get pods -l app=cert-manager --all-namespaces
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.10.0/cert-manager.yaml
```

### 2. Deploy the Helm Chart with the Operator enabled

You can pass `--set operator.enabled=true` when deploying the chart to enable the operator. You can also use commands
like in the following example.

```
VALUES_FILE=examples/enable-operator-and-auto-instrumentation/enable-operator-and-auto-instrumentation-values.yaml
helm install splunk-otel-collector \
-f $VALUES_FILE \
-n monitoring \
helm-charts/splunk-otel-collector
```

### 3. Deploy the opentelemetry.io/v1alpha1 Instrumentation

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
  dotnet:
    env:
      - name: SPLUNK_OTEL_AGENT
        valueFrom:
          fieldRef:
            apiVersion: v1
            fieldPath: status.hostIP
      - name: OTEL_EXPORTER_OTLP_ENDPOINT
        value: http://$(SPLUNK_OTEL_AGENT):4317
      - name: OTEL_TRACES_EXPORTER
        value: otlp
```

</details>

```
# Install
kubectl apply -f splunk-instrumentation.yaml
# Check the current deployed values
kubectl get otelint -o yaml
```

### 4. Verify all the OpenTelemetry resources (collector, operator, webhook, instrumentation) are deployed successfully

<details open>
<summary>Expand for sample output to verify against</summary>

```
kubectl  get pods -n monitoring
# NAME                                                          READY
# STATUS    RESTARTS   AGE
# splunk-otel-collector-agent-9ccgn                             2/2     Running   0          3m
# splunk-otel-collector-agent-ft4xc                             2/2     Running   0          3m
# splunk-otel-collector-k8s-cluster-receiver-56f7c9cf5b-mgsbj   1/1     Running   0          3m
# splunk-otel-collector-operator-6dffc898df-5jjkp               2/2     Running   0          3m

kubectl get mutatingwebhookconfiguration.admissionregistration.k8s.io -n monitoring
# NAME                                      WEBHOOKS   AGE
# cert-manager-webhook                      1          8m
# splunk-otel-collector-operator-mutation   3          2m

kubectl get otelinst -n spring-petclinic
# NAME                          AGE   ENDPOINT
# splunk-instrumentation        3m   http://$(SPLUNK_OTEL_AGENT):4317
```

</details>

### 5. Instrument application by setting an annotation

An _inject instrumentation_ annotation can be added to the following.
- Namespace: All pods within that namespace will be instrumented.
- Pod Spec Objects: PodSpec objects that are available as part of Deployment,
  Statefulset, or other resources can be annotated.
  following values.
- "true" - inject and Instrumentation resource from the namespace.
- "my-instrumentation" - name of Instrumentation CR instance in the current
  namespace.
- "my-other-namespace/my-instrumentation" - name and namespace of.
  Instrumentation CR instance in another namespace.
- "false" - do not inject.

### 6. Check out the results at [Splunk Observability APM](https://app.us1.signalfx.com/#/apm)

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
| dotnet                  | OpenTelemetry | Available   | Needs Validation |                              | [Link](https://github.com/open-telemetry/opentelemetry-dotnet-instrumentation)       | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-dotnet       |
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
