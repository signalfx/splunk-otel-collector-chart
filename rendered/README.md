# Manifests

The [manifests](manifests) directory contains pre-rendered Kubernetes resource
manifests that can be applied with `kubectl create`. Different sets contain
different features enabled. For now, configured for Slunk Observability only.

- [metrics-only](manifests/metrics-only)
- [traces-only](manifests/traces-only)
- [logs-only](manifests/logs-only)
- [agent-only](manifests/agent-only)

    contains manifests with all telemetry types enabled for the agent (the default when installing Helm chart).

Values that must be changed can be found by searching for `CHANGEME`. The secret manifest must be updated to include the encoded version of the access token. Use `stringData` in the resource to do this automatically.
