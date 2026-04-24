# Example: Collector on EKS with AWS Batch nodes

This example shows how to configure the Splunk OpenTelemetry Collector to
collect logs and metrics from EKS nodes managed by
[AWS Batch on EKS](https://docs.aws.amazon.com/batch/latest/userguide/jobs_eks.html).

## Background

AWS Batch taints its managed EKS nodes with `batch.amazonaws.com/batch-node` to
prevent general workloads from scheduling there. The Collector agent daemonset
must explicitly tolerate this taint to run on those nodes.

The chart's top-level `tolerations` value is a list. Helm replaces lists
entirely during upgrades, so your values file must include **both** the chart's
built-in default tolerations and the new AWS Batch entries. Omitting the
defaults would stop the agent from scheduling on control-plane and infra nodes.

## Usage

```bash
helm install my-splunk-otel-collector \
  --values eks-batch-nodes-values.norender.yaml \
  splunk-otel-collector-chart/splunk-otel-collector
```

Replace the `CHANGEME` placeholders before running.

## See also

- [Advanced configuration - EKS: Running on AWS Batch nodes](../../docs/advanced-configuration.md#eks-running-on-aws-batch-nodes)
- [Run a DaemonSet on AWS Batch managed nodes](https://docs.aws.amazon.com/batch/latest/userguide/daemonset-on-batch-eks-nodes.html)
- [Kubernetes taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)
