# Example of chart configuration

## How to install the collector on Talos Linux (experimental)

This example configures experimental support for Talos Linux 1.14.6 with Kubernetes 1.36.1.

Talos has a read-only root filesystem and does not ship
`/usr/lib/os-release`. The preset omits that host path and the Docker-specific
`/var/lib/docker/containers` path. Container logs are read from `/var/log/pods`.

Set `clusterName` explicitly; it is not auto-discovered on Talos.

Before installing, create a namespace that allows the agent's host networking,
host ports, and host paths. See [Talos Linux configuration](../../docs/advanced-configuration.md#talos-linux-experimental)
for the required Pod Security label, installation commands, and limitations.
