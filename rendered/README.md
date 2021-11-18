# Manifests

The [manifests](manifests) directory contains pre-rendered Kubernetes resource
manifests that can be applied using the `kubectl create` command. The directory contains manifests with all telemetry types enabled for the agent, which is the default when installing the Helm chart. These manifests should be configured for Splunk Observability Cloud only. 

- [Default configuration deployment](manifests/agent-only)
- [Default deployment with native OTel logs collection](manifests/otel-logs)
- [Metrics collection only](manifests/metrics-only)
- [Traces collection only](manifests/traces-only)
- [Fluent logs collection only](manifests/logs-only)

Search using "CHANGEME" to find the values that must be changed. The secret manifest must be updated to include the encoded version of the access token. Use `stringData` in the resource to do this automatically.
