# Example of chart configuration

## How to install the collector on Talos Linux (experimental)

This example configures experimental support for Kubernetes on Talos Linux.

Talos mounts its root filesystem as a read-only squashfs and does not ship
`/usr/lib/os-release`. The preset omits that host path and the Docker-specific
`/var/lib/docker/containers` path. Container logs are read from `/var/log/pods`.

Unlike `gke` and `eks`, the cluster name is not auto-discovered on Talos, so
`clusterName` must be set.

Before installing, create a namespace that allows the agent's host networking,
host ports, and host paths. See [Talos Linux configuration](../../docs/advanced-configuration.md#talos-linux-experimental)
for the required Pod Security label, installation commands, and limitations.
