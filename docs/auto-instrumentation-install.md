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

- **Operator Deployment (Required)**
  - `operatorcrds.install`: Set to `true` to install the CRDs required by the operator.
    - **Required**: Must be set unless CRDs are pre-installed manually.
  - `operator.enabled`: Set to `true` to enable deploying the operator.
    - **Required**: This configuration is necessary for the operator's deployment within your cluster.

- **TLS Certificate Management (Required)**
  - **Using cert-manager (Recommended)**
    - `certmanager.enabled`: Enable cert-manager by setting to `true`.
      - **Check Before Enabling**: Ensure cert-manager is not already installed to avoid multiple instances.
      - **Recommended**: Cert-manager simplifies the management of TLS certificates, automating issuance and renewal.

  - **Alternative Methods**
    - **Automatically Generate a Self-Signed Certificate with Helm**
      - `operator.admissionWebhooks.autoGenerateCert.enabled`: Set to `true` to enable Helm to automatically create a self-signed certificate.
        - **Use Case**: Suitable when cert-manager is not installed or preferred.
    - **Provide Your Own Certificate**
      - Ensure both `operator.admissionWebhooks.certManager.enabled` and `operator.admissionWebhooks.autoGenerateCert.enabled` are set to `false`.
      - `operator.admissionWebhooks.cert_file`: Path to your PEM-encoded certificate.
      - `operator.admissionWebhooks.key_file`: Path to your PEM-encoded private key.
      - `operator.admissionWebhooks.ca_file`: Path to your PEM-encoded CA certificate.
        - **Use Case**: Ideal for integrating existing certificates or custom certificate management processes.

- **Deployment Environment (Required)**
  - **Via `values.yaml` (Recommended)**
    - `environment`: Required configuration to set the deployment environment attribute in exported traces.

  - **Alternative Methods**
    - **Instrumentation Spec**
      - `operator.instrumentation.spec.env`: Use with the `OTEL_RESOURCE_ATTRIBUTES` environment variable to specify the deployment environment.

- **Auto-instrumentation Configuration Overrides (Optional)**
  - **[Default Instrumentation](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/examples/enable-operator-and-auto-instrumentation/rendered_manifests/operator/instrumentation.yaml) Object Deployment**
    - Automatically deploys with `operator.enabled=true`.
    - Supports AlwaysOn Profiling when `splunkObservability.profilingEnabled=true`.
  - **Customizing Instrumentation**
    - `operator.instrumentation.spec`: Override values under this parameter to customize the deployed opentelemetry.io/v1alpha1 Instrumentation object.
      - **Examples**
        - [Custom environment span tags](../examples/enable-operator-and-auto-instrumentation/instrumentation/instrumentation_add_custom_environment_span_tag.yaml)
        - [trace sampler](../examples/enable-operator-and-auto-instrumentation/instrumentation/instrumentation-add-trace-sampler.yaml)
        - [partially enable profiling](../examples/enable-operator-and-auto-instrumentation/instrumentation/instrumentation-enable-profiling-partially.yaml).

```bash
# Check if cert-manager is already installed, don't deploy a second cert-manager.
kubectl get pods -l app=cert-manager --all-namespaces

# If cert-manager is not deployed, make sure to add certmanager.enabled=true to the list of values to set
helm install splunk-otel-collector -f ./my_values.yaml --set operatorcrds.install=true,operator.enabled=true,environment=dev splunk-otel-collector-chart/splunk-otel-collector
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

The OpenTelemetry operator relies on
[Custom Resource Definitions (CRDs)](https://github.com/signalfx/splunk-otel-collector-chart/tree/main/helm-charts/splunk-otel-collector/charts/opentelemetry-operator-crds)
to manage auto-instrumentation configurations in Kubernetes.
Ensure the required CRDs are deployed before the operator by configuring `operatorcrds.install=true`.

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

In the table below current instrumentation libraries are listed, if they are supported, and how compatible they are
with Splunk customer content.
_The native OpenTelemetry instrumentation libraries are owned and maintained by the OpenTelemetry Community, Splunk
provides best effort support with issues related to native OpenTelemetry instrumentation libraries._

| Instrumentation Library | Distribution  | Status      | Supported        | Splunk Content Compatability | Code Repo                                                                            | Image Repo                                                                     |
|-------------------------|---------------|-------------|------------------|------------------------------|--------------------------------------------------------------------------------------|--------------------------------------------------------------------------------|
| java                    | Splunk        | Available   | Yes              | Completely                   | [Link](github.com/signalfx/splunk-otel-java)                                         | ghcr.io/signalfx/splunk-otel-java/splunk-otel-java                             |
| dotnet                  | Splunk        | Coming Soon |                  |                              | [Link](github.com/signalfx/splunk-otel-dotnet)                                       |                                                                                |
| nodejs                  | Splunk        | Available   | Yes              | Completely                   | [Link](github.com/signalfx/splunk-otel-nodejs)                                       | ghcr.io/signalfx/splunk-otel-java/splunk-otel-js                               |
| python                  | Splunk        | Coming Soon |                  |                              | [Link](github.com/signalfx/splunk-otel-python)                                       |                                                                                |
| java                    | OpenTelemetry | Available   | Yes              | Mostly                       | [Link](https://github.com/open-telemetry/opentelemetry-java-instrumentation)         | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-java         |
| dotnet                  | OpenTelemetry | Available   | Yes              | Mostly                       | [Link](https://github.com/open-telemetry/opentelemetry-dotnet-instrumentation)       | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-dotnet       |
| nodejs                  | OpenTelemetry | Available   | Yes              | Mostly                       | [Link](https://github.com/open-telemetry/opentelemetry-nodejs-instrumentation)       | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-nodes        |
| python                  | OpenTelemetry | Available   | Needs Validation |                              | [Link](https://github.com/open-telemetry/opentelemetry-java-instrumentation)         | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-java         |
| apache-httpd            | OpenTelemetry | Available   | Needs Validation |                              | [Link](https://github.com/open-telemetry/opentelemetry-apache-httpd-instrumentation) | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-apache-httpd |
| nginx                   | OpenTelemetry | Available   | Needs Validation |                              | [Link](https://github.com/open-telemetry/opentelemetry-apache-httpd-instrumentation) | ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-apache-httpd |

### CRD Management

When deploying the operator, the required Custom Resource Definitions (CRDs) must be deployed beforehand.

#### Recommended Approach: Automated CRD Deployment

Set the Helm chart value `operatorcrds.install=true` to allow the chart to handle CRD installation automatically.
_This option deploys the CRDs using a local subchart, available at [opentelemetry-operator-crds](https://github.com/signalfx/splunk-otel-collector-chart/tree/main/helm-charts/splunk-otel-collector/charts/opentelemetry-operator-crds)._
_Please note, helm will not update or delete these CRDs after initial install as noted in their [documentation](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations)._

#### Alternative Approach: Manual CRD Deployment

If you prefer to manage CRD deployment manually, apply the CRDs using the commands below before installing the Helm chart:

```bash
curl -sL https://raw.githubusercontent.com/splunk-otel-collector/charts/opentelemetry-operator-crds/crds/opentelemetry.io_opentelemetrycollectors.yaml | kubectl apply -f -
curl -sL https://raw.githubusercontent.com/splunk-otel-collector/charts/opentelemetry-operator-crds/crds/opentelemetry.io_opampbridges.yaml | kubectl apply -f -
curl -sL https://raw.githubusercontent.com/splunk-otel-collector/charts/opentelemetry-operator-crds/crds/opentelemetry.io_instrumentations.yaml | kubectl apply -f -
```

You can also use below helm template command to get the CRD yamls from the helm chart. This method can be helpful in keeping CRDs in-sync with the version bundled with our helm chart.

```bash
helm template splunk-otel-collector-chart/splunk-otel-collector --include-crds \
--set="splunkObservability.realm=us0,splunkObservability.accessToken=xxxxxx,clusterName=my-cluster,operatorcrds.install=true" \
| yq e '. | select(.kind == "CustomResourceDefinition")' \
| kubectl apply -f -
```

#### CRD Updates

With Helm v3.0 and later, CRDs created by this chart are not updated automatically. To update CRDs, you must apply the updated CRD definitions manually.
Refer to the [Helm Documentation on CRDs](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/) for more details.

#### CRD Cleanup

When uninstalling this chart, the OpenTelemetry CRDs are not removed automatically. To delete them manually, use the following commands:

```bash
kubectl delete crd opentelemetrycollectors.opentelemetry.io
kubectl delete crd opampbridges.opentelemetry.io
kubectl delete crd instrumentations.opentelemetry.io
```
You can use below combination of helm and kubectl command to delete CRDs.

```bash
helm template splunk-otel-collector-chart/splunk-otel-collector --include-crds \
--set="splunkObservability.realm=us0,splunkObservability.accessToken=xxxxxx,clusterName=my-cluster,operatorcrds.install=true" \
| yq e '. | select(.kind == "CustomResourceDefinition")' \
| kubectl delete --dry-run=client -f -
```

### Documentation Resources

- https://developers.redhat.com/devnation/tech-talks/using-opentelemetry-on-kubernetes
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/docs/api.md#instrumentation
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md#opentelemetry-auto-instrumentation-injection
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md#use-customized-or-vendor-instrumentation

### Troubleshooting the Operator and Cert Manager

#### Check the logs for failures

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

#### Operator Issues

##### Networking and Firewall Requirements

Ensure the Mutating Webhook used by the operator for pod auto-instrumentation is not hindered by network policies or firewall rules. Key points to ensure:

- **Webhook Accessibility**: The webhook must freely communicate with the cluster IP and the Kubernetes API server. Ensure network policies or firewall rules permit operator-related services to interact with these endpoints.
- **Required Ports**: Policies should explicitly allow traffic to the necessary ports for seamless operation.

Use the following command to identify the IP addresses and ports that need to be accessible:

```bash
kubectl get svc -n {operator_namespace}
# Example output indicating necessary IP and port configurations:
# NAME                                          TYPE       CLUSTER-IP    EXTERNAL-IP  PORT(S)                                       AGE
# kubernetes                                    ClusterIP  10.0.0.1      <none>       443/TCP                                       10d
# splunk-splunk-otel-collector-agent            ClusterIP  10.0.176.113  <none>       8006/TCP,14250/TCP,14268/TCP,...              3d17h
# splunk-splunk-otel-collector-operator         ClusterIP  10.0.254.125  <none>       8443/TCP,8080/TCP                             3d17h
# splunk-splunk-otel-collector-operator-webhook ClusterIP  10.0.222.223  <none>       443/TCP                                       3d17h
```

- **Configuration Action**: Adjust your network policies and firewall settings based on the service endpoints and ports listed by the command. This ensures the webhook and operator services can properly communicate within the cluster.

#### Cert-Manager Issues

If the operator seems to be hanging, it could be due to the cert-manager not auto-creating the required certificate. To troubleshoot:

- Check the health and logs of the cert-manager pods for potential issues.
- Consider restarting the cert-manager pods.
- Ensure that your cluster has only one instance of cert-manager, which should include `certmanager`, `certmanager-cainjector`, and `certmanager-webhook`.

For additional guidance, refer to the official cert-manager documentation:
- [Troubleshooting Guide](https://cert-manager.io/docs/troubleshooting/)
- [Uninstallation Guide](https://cert-manager.io/v1.2-docs/installation/uninstall/kubernetes/)

##### Validate Certificates

Ensure that the certificate, which the cert-manager creates and the operator utilizes, is available.

```bash
kubectl get certificates
# NAME                                          READY   SECRET                                                           AGE
# splunk-otel-collector-operator-serving-cert   True    splunk-otel-collector-operator-controller-manager-service-cert   5m
```

##### Using a Self-Signed Certificate for the Webhook

The operator supports various methods for managing TLS certificates for the webhook. Below are the options available through the operator, with a brief description for each. For detailed configurations and specific use cases, please refer to the operator’s
[official Helm chart documentation](https://github.com/open-telemetry/opentelemetry-helm-charts/blob/main/charts/opentelemetry-operator/values.yaml)

**Note**: While using a self-signed certificate offers a quicker and simpler setup, it has limitations, such as not being trusted by default by clients.
This may be acceptable for testing purposes or internal environments. For complete configurations and additional guidance, please refer to the provided link to the Helm chart documentation.
