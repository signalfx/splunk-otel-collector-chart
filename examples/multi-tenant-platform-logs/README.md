# Route Splunk Platform logs by tenant

> [!WARNING]
> Multi-destination log routing is experimental.

This example derives a `product` route from K8s pod and namespace labels,
preferring the pod label. Unmatched logs use the default destination.

For pod-label-only routing, use `attribute: k8s.pod.labels.product` and remove
the derivation processor.

It includes Secret and mounted-file credentials, route processors, metadata
precedence, and a persistent `security` queue for direct agent export. Explicit
route indexes override `splunk.com/index` annotations; omitted metadata remains
unchanged.

Create the `application-hec` and `security-hec` Secrets and replace the example
values before installation. The mounted token uses mode `0440` with the default
agent `fsGroup: 999`; preserve read access if you override the pod security
context.

Queue isolation ends when a failed route's queue fills, and route-local
drop-on-overflow is not supported. Gateway queues are memory-only.
Cluster-receiver events and objects continue to use the default destination.

See [multi-tenant log routing](../../docs/advanced-configuration.md#route-logs-to-multiple-splunk-platform-destinations)
for inheritance and gateway behavior.

The committed [rendered agent ConfigMap](rendered_manifests/configmap-agent.yaml)
is the reviewable generated output; `make unittest && make render` verifies it.
