# OpenTelemetry Collector CRDs

This chart contains the CRDs for _*installation*_ only right now for the opentelemetry-operator. This allows the Splunk OpenTelemetry Collector chart to work on install. You can see more discussion about this [here](https://github.com/open-telemetry/opentelemetry-helm-charts/issues/677) and [here](https://github.com/open-telemetry/opentelemetry-helm-charts/pull/1203).

This approach is inspired by the [opentelemetry-kube-stack chart](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-kube-stack) approach.

> [!NOTE]
> The splunk-otel-collector chart only supports and tests functionality related to the auto-instrumentation that requires the Instrumentation CRD.
> Other CRDs, such as OpenTelemetryCollector, OpAMPBridge and TargetAllocator, are included solely to allow the Operator to start up and are not currently supported or tested.

# Upgrade Notes

Helm does NOT automatically update CRDs during helm upgrades.

## CRD Sources

The CRDs in this chart are fetched from the [opentelemetry-operator repository](https://github.com/open-telemetry/opentelemetry-operator/tree/main). The OPERATOR_APP_VERSION corresponds to the opentelemetry-operator tag/appVersion bundled in the splunk-otel-collector chart.
```
# CRDs are sourced from the tag matching our operator version
wget https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/v${OPERATOR_APP_VERSION}/config/crd/bases/opentelemetry.io_opentelemetrycollectors.yaml
wget https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/v${OPERATOR_APP_VERSION}/config/crd/bases/opentelemetry.io_opampbridges.yaml
wget https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/v${OPERATOR_APP_VERSION}/config/crd/bases/opentelemetry.io_instrumentations.yaml
wget https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/v${OPERATOR_APP_VERSION}/config/crd/bases/opentelemetry.io_targetallocators.yaml
```

## Manual CRD Update Process
Helm deliberately skips updating CRDs during helm upgrade operations as CRD changes can be destructive. This is a [well-documented limitation of Helm](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations).

Follow these steps to safely update your CRDs:

#### 1. Extract Current CRDs from Your Chart

Before extracting the CRDs, ensure your Helm repository is up-to-date. To fetch the latest version of the splunk-otel-collector chart run:

```bash
# Update your Helm repository
helm repo update splunk-otel-collector-chart
```

You can extract the CRDs bundled in the latest version of the chart or specify a particular version using the `--version` flag:

```bash
# Dump the CRDs to a temp file from the latest chart version
helm template splunk-otel-collector-chart/splunk-otel-collector --include-crds \
--set="splunkObservability.realm=us0,splunkObservability.accessToken=xxxxxx,clusterName=my-cluster,operatorcrds.install=true" \
| yq e '. | select(.kind == "CustomResourceDefinition")' > /tmp/new-crds.yaml

# OR get the CRDs from a specific chart version
helm template splunk-otel-collector-chart/splunk-otel-collector --include-crds \
--version=<CHART_VERSION> \
--set="splunkObservability.realm=us0,splunkObservability.accessToken=xxxxxx,clusterName=my-cluster,operatorcrds.install=true" \
| yq e '. | select(.kind == "CustomResourceDefinition")' > /tmp/new-crds.yaml
```

Replace `<CHART_VERSION>` with the desired version of the chart.

#### 2. Compare with Currently Installed CRDs
```bash
# See what would change with kubectl diff
kubectl diff -f /tmp/new-crds.yaml
```

#### 3. Apply Updated CRDs
After reviewing changes, apply the new CRDs.
```bash
# Apply the updated CRDs
kubectl apply -f /tmp/new-crds.yaml
```

#### 4. Verify the Update
```bash
# Check the CRDs and their versions
kubectl get crds -o jsonpath='{range .items[?(@.spec.group=="opentelemetry.io")]}{.metadata.name}{" - "}{.spec.versions[*].name}{"\n"}{end}'
```

#### 5. Continure with splunk-otel-collector Upgrade
Once the CRDs are updated, you can proceed with the upgrade of the splunk-otel-collector chart.
