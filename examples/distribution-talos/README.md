# Example of chart configuration

## How to install the collector on Talos Linux

In this example we will show how to install the collector on this Kubernetes
distribution.

Talos mounts its root filesystem as a read-only squashfs and does not ship
`/usr/lib/os-release`, so that host path is not mounted on this distribution.
The `hostmetrics` receiver still reports OS details, which it reads from
`/hostfs/etc/os-release`.

Unlike `gke` and `eks`, the cluster name is not auto-discovered on Talos, so
`clusterName` must be set.
