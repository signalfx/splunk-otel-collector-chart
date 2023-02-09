# Example of chart configuration

## Logs collection configuration for CRI-O container runtime

Default logs collection is configured for Docker container runtime.
The following configuration should be set for CRI-O or containerd runtimes.

`criTimeFormat` can be used to configure logs collection for different log
formats, e.g. `criTimeFormat: "%Y-%m-%dT%H:%M:%S.%NZ"` for IBM IKS.
