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
  - **Automatically Generate a Self-Signed Certificate with Helm (Default)**
    - `operator.admissionWebhooks.autoGenerateCert.enabled`: Set to `true` to enable Helm to automatically create a self-signed certificate.

  - **Alternative Methods**

    - **Using cert-manager**
      -  Use an already installed certmanager by setting `operator.admissionWebhooks.certManager.enabled` to `true`.
        -  **Use Case**: Ideal for environments already leveraging `certmanager` for certificate management.
      -  _NOTE_ - The option to install `certmanager` with our chart is deprecated and will be removed in future releases.

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
helm install splunk-otel-collector -f ./my_values.yaml --set operatorcrds.install=true,operator.enabled=true,environment=dev splunk-otel-collector-chart/splunk-otel-collector
```

### 2. Verify all the OpenTelemetry resources (collector, operator, webhook, instrumentation) are deployed successfully

```bash
kubectl get pods
# NAME                                                            READY   STATUS
# splunk-otel-collector-agent-lfthw                               2/2     Running
# splunk-otel-collector-k8s-cluster-receiver-856f5fbcf9-pqkwg     1/1     Running
# splunk-otel-collector-opentelemetry-operator-56c4ddb4db-zcjgh   2/2     Running

kubectl get validatingwebhookconfiguration
# NAME                                      WEBHOOKS   AGE
# splunk-otel-collector-opentelemetry-operator-admission   3          14m

kubectl get mutatingwebhookconfiguration
# NAME                                      WEBHOOKS   AGE
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
  - The OpenTelemetry Operator Docker image only supports Linux and cannot run on Windows nodes.
- Sub-components
  - Admission and mutating webhooks, including ValidatingWebhookConfiguration and MutatingWebhookConfiguration.

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
curl -sL https://raw.githubusercontent.com/signalfx/splunk-otel-collector-chart/main/helm-charts/splunk-otel-collector/charts/opentelemetry-operator-crds/crds/opentelemetry.io_opentelemetrycollectors.yaml | kubectl apply -f -
curl -sL https://raw.githubusercontent.com/signalfx/splunk-otel-collector-chart/main/helm-charts/splunk-otel-collector/charts/opentelemetry-operator-crds/crds/opentelemetry.io_opampbridges.yaml | kubectl apply -f -
curl -sL https://raw.githubusercontent.com/signalfx/splunk-otel-collector-chart/main/helm-charts/splunk-otel-collector/charts/opentelemetry-operator-crds/crds/opentelemetry.io_instrumentations.yaml | kubectl apply -f -
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

### TLS Certificate Requirement for Kubernetes Operator Webhooks

In Kubernetes, the API server communicates with operator webhook components over HTTPS, which requires a valid TLS certificate that the API server trusts. The operator supports several methods for configuring the required certificate, each with different levels of complexity and security.

---

#### 1. **Using a Self-Signed Certificate Generated by the Chart**

This is the default and simplest method for generating a TLS certificate. It automatically creates a self-signed certificate for the webhook, making it suitable for internal environments or testing purposes. However, it may not be trusted by clients outside your cluster.

**Note**: The following settings reflect the default values starting in **v1.20.0** of this chart. You only need to update them if using a **previous chart version** or if additional customization is required.

```yaml
operator:
  admissionWebhooks:
    autoGenerateCert:
      enabled: true
      certPeriodDays: 3650
    certManager:
      enabled: false
```

- Setting `operator.admissionWebhooks.certManager.enabled` to `false` and `operator.admissionWebhooks.autoGenerateCert.enabled` to `true` ensures that Helm generates a self-signed TLS certificate.
- Helm generates a self-signed certificate that is valid for 10 years (3650 days) and stores it in a secret for the Operator webhook. The certificate's validity period can be adjusted using `operator.admissionWebhooks.autoGenerateCert.certPeriodDays`.
- The certificate is **automatically regenerated** on every Helm upgrade. To disable this behavior, set `operator.admissionWebhooks.autoGenerateCert.recreate` to `false`.

---

#### 2. **Using a cert-manager Certificate**

Using `cert-manager` offers more control over certificate management and is more suitable for production environments. However, due to Helm’s install/upgrade order of operations, cert-manager CRDs and certificates cannot be installed within the same Helm operation. To work around this limitation, you can choose one of the following options:

##### Option 1: **Pre-deploy cert-manager**

If `cert-manager` is already deployed in your cluster, you can configure the operator to use it without enabling certificate generation by Helm.

**Configuration:**
```yaml
operator:
  admissionWebhooks:
    certManager:
      enabled: true
```

##### Option 2: **Deploy cert-manager and the operator together (Deprecated)**

If you need to install `cert-manager` along with the operator, use a Helm post-install or post-upgrade hook to ensure that the certificate is created after cert-manager CRDs are installed.

**Configuration:**
```yaml
operator:
  admissionWebhooks:
    certManager:
      enabled: true
      certificateAnnotations:
        "helm.sh/hook": post-install,post-upgrade
        "helm.sh/hook-weight": "1"
      issuerAnnotations:
        "helm.sh/hook": post-install,post-upgrade
        "helm.sh/hook-weight": "1"
certmanager:
  enabled: true
  installCRDs: true
```

This method is useful when installing `cert-manager` as a subchart or as part of a larger Helm chart installation.

---

#### 3. **Using a Custom Externally Generated Certificate**

For full control, you can use an externally generated certificate. This is suitable if you already have a certificate issued by a trusted CA or have specific security requirements.

**Configuration:**
- Set both `operator.admissionWebhooks.certManager.enabled` and `operator.admissionWebhooks.autoGenerateCert.enabled` to `false`.
- Provide the paths to your certificate (`certFile`), private key (`keyFile`), and CA certificate (`caFile`) in the values.

**Example:**
```yaml
operator:
  admissionWebhooks:
    certManager:
      enabled: false
    autoGenerateCert:
      enabled: false
    certFile: /path/to/cert.crt
    keyFile: /path/to/cert.key
    caFile: /path/to/ca.crt
```

This method allows you to use a certificate that is trusted by external systems, such as certificates issued by a corporate CA.

---

For more advanced use cases, refer to the [official Helm chart documentation](https://github.com/open-telemetry/opentelemetry-helm-charts/blob/main/charts/opentelemetry-operator/values.yaml) for detailed configuration options and scenarios.

### Troubleshooting the Operator

#### General Debugging Steps

- Check the logs for the operator to identify any issues:
  ```bash
  kubectl logs -l app.kubernetes.io/name=operator
  ```
- The operator webhooks must communicate with the Kubernetes API server. Errors related to webhook usage can often be found in the API server logs:
  - For self-managed clusters, check logs directly:
    ```bash
    kubectl logs -n <api-server-namespace> -l component=kube-apiserver
    ```
  - For managed clusters, follow the platform-specific steps to enable and view API server logs:
    - [AKS: Monitor Logs](https://learn.microsoft.com/en-us/azure/aks/monitor-aks?tabs=cilium)
    - [EKS: Enable or Disable Control Plane Logs](https://docs.aws.amazon.com/eks/latest/userguide/control-plane-logs.html)
    - [GKE: View Logs](https://cloud.google.com/kubernetes-engine/docs/how-to/view-logs)
    - [OpenShift: Logging and Monitoring](https://docs.openshift.com/container-platform/latest/logging/cluster-logging.html)
- If using certmanager for TLS certificates, check its logs for issues:
  ```bash
  kubectl logs -l app=certmanager
  kubectl logs -l app=cainjector
  kubectl logs -l app=webhook
  ```
- **Verify Webhook Configurations**:
  Check `MutatingWebhookConfiguration` and `ValidatingWebhookConfiguration`:
  ```bash
  kubectl get mutatingwebhookconfiguration
  kubectl get validatingwebhookconfiguration
  ```
- **Inspect Network Policies**:
  Ensure there are no network policies blocking communication between the namespace where the chart
  is deployed and the namespace where the Kubernetes control plane resides.
  ```bash
  kubectl get networkpolicy -n <chart-namespace>
  kubectl get networkpolicy -n <control-plane-namespace>
  ```
- **Review Events**:
  Look for recent error or failure events in the chart namespace:
  ```bash
  kubectl get events -n <namespace>
  ```
#### Checking Operator <-> API Server Connectivity steps
Test Operator to API Server Connection
1. Create a `busybox` pod in the Operator's namespace:
   ```bash
   kubectl run busybox-test --rm -it --restart=Never -n <operator-namespace> --image=busybox -- /bin/sh
   ```
2. Enter the `busybox` pod:
   ```bash
   kubectl exec -it busybox-test -n <operator-namespace> -- /bin/sh
   ```
3. Attempt to contact the API Server:
   ```bash
   wget --spider https://kubernetes.default.svc
   ```
4. If the connection fails, investigate:
  - Network policies in the Operator's namespace.
  - Service account permissions.

Test API Server to Operator Webhook Connection
1. Create a `busybox` pod in the API Server's namespace:
   ```bash
   kubectl run busybox-test --rm -it --restart=Never -n <api-server-namespace> --image=busybox -- /bin/sh
   ```
2. Enter the `busybox` pod:
   ```bash
   kubectl exec -it busybox-test -n <api-server-namespace> -- /bin/sh
   ```
3. Attempt to contact the Operator's webhook:
   ```bash
   wget --spider http://<operator-webhook-service>.<operator-namespace>.svc.cluster.local
   ```
4. If the connection fails, investigate:
  - The `Service` and `Endpoints` for the Operator webhook.
  - Network policies in the Operator's namespace.

### Known Issues

**Custom Network Policies or Security Layers**
- **Cause:** Tools like Calico, Cilium, or custom firewalls may block communication between the API
  server and the operator webhook.
- **Resolution:**
  - Before reaching out to Splunk Support, consult with your infrastructure or platform
  team who set up your cluster. They may have implemented custom network policies or security layers
  that could be affecting communication.
  - If you are using networking or security solutions from a third-party Kubernetes solution provider,
    be aware that these may include configurations or custom CRDs that can impact this operator's
    functionality. Since these configurations vary widely per provider, we cannot provide specific
    guidance for every product here. We recommend reviewing the providers configurations, CRD definitions,
    and deployed CRD instances in your cluster to identify any settings related to networking or
    security that might interfere with communication between the operator and the Kubernetes API server.
     ```bash
     kubectl get crds
     kubectl get <crd-name> --all-namespaces
     kubectl get <crd-name> -n <namespace> -o yaml
     ```

**[EKS/Cilium] API Server Error: "No endpoints available for service 'splunk-otel-collector-operator-webhook'"**
- **Cause:** This is a general known issue in setups where the Kubernetes control plane cannot communicate
  with admission webhooks, such as the operator's webhook, in other namespaces. This occurs because
  the customer has deployed a custom networking solution (e.g., Cilium in overlay mode) that restricts
  the expected communication between the control plane and webhooks that are not a part of the control
  plane. The issue is not caused by the operator itself but by the limitations of the custom networking configuration.
- **Resolution:**
  - **Solution 1: Enable ENI Mode in Cilium**
    - Update the AWS Cilium setup to use ENI mode. This configuration allows the control plane to communicate
      with webhooks in other namespaces. Refer to the [Cilium ENI Documentation](https://docs.cilium.io/en/stable/gettingstarted/eni/).
  - **Solution 2: Run the Operator in Host Network Mode**
    - Modify the `splunk-otel-collector-chart` Helm chart values to enable host network mode for the operator:
      ```yaml
      operator:
        hostNetwork: true
      ```
    - Apply the updated Helm chart configuration and redeploy the operator.
    - **Note:** While this workaround resolves the issue, running the operator in host network mode is
      considered a less secure practice and thus the 1st solution would be more favorable for security.

- **Related Links:**
  - [Cilium Issue #21959 How to use an admission webhook with Cilium?](https://github.com/cilium/cilium/issues/21959)
  - [OpenTelemetry Operator Issue #2260 Webhook "address is not allowed" when creating an Instrumentation on EKS](https://github.com/open-telemetry/opentelemetry-operator/issues/2260)
  - [Cilium Issue  #30111 EKS Cilium in Overlay with ALB and webhooks: Address is not allowed](https://github.com/cilium/cilium/issues/30111)

### Documentation Resources

- https://developers.redhat.com/devnation/tech-talks/using-opentelemetry-on-kubernetes
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/docs/api.md#instrumentation
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md#opentelemetry-auto-instrumentation-injection
- https://github.com/open-telemetry/opentelemetry-operator/blob/main/README.md#use-customized-or-vendor-instrumentation
