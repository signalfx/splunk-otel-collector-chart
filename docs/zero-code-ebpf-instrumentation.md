# Zero-Code Instrumentation with eBPF

[**O**penTelemetry e**B**PF **I**nstrumentation] (OBI) is a lightweight and efficient way to collect telemetry data using eBPF for user-space applications.
It provides many feature for applications using a zero-code instrumentation approach:

- **Context Propagation**: Propagate trace context across services automatically
- **Wide language support**: Java, .NET, Go, Python, Ruby, Node.js, C, C++, and Rust
- **Lightweight**: No code changes required, no libraries to install, no restarts needed
- **Efficient instrumentation**: Traces and metrics are captured by eBPF probes with minimal overhead
- **Kubernetes-native**: Provides configuration-free auto-instrumentation for Kubernetes applications
- **Visibility into encrypted communications**: Capture transactions over TLS/SSL without decryption
- [**Protocol support**]: HTTP/S, gRPC, SQL, Redis, MongoDB, and more
- **Low cardinality metrics**: Prometheus-compatible metrics with low cardinality for cost reduction
- **Network observability**: Capture network flows between services
- **Database traces**: Capture database queries and connections

> [!TIP]
> OBI provides broad observability for most services within a cluster.
>
> Use the language specific [auto-instrumentation] get deeper insights into applications where supported (Java, NodeJs, Python, .NET).
> OBI will auto-detect if auto-instrumentation is present and avoid duplicating traces.
>
> For more detailed observability, you can also combine OBI with application-level instrumentation using OpenTelemetry SDKs.

[**O**penTelemetry e**B**PF **I**nstrumentation]: https://opentelemetry.io/docs/zero-code/obi/
[**Protocol support**]: https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/main/devdocs/features.md
[auto-instrumentation]: ./auto-instrumentation-introduction.md

## Getting Started

OBI can be enabled by setting the `obi.enabled` value to `true` during installation or upgrades.

```bash
helm install my-splunk-otel-collector \
  splunk-otel-collector-chart/splunk-otel-collector \
  --set="obi.enabled=true"
  # All other options as needed ...
```

### Configuration Options

For basic usage, no additional configuration is required.

Additional configuration options are available to customize OBI features.
Refer to the [OBI chart's documentation] for more details.

[OBI chart's documentation]: https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-ebpf-instrumentation

### Verification

After installation, verify OBI pods are up and running:

```bash
# Check if eBPF instrumentation pods are running
kubectl get pods -l app.kubernetes.io/name=obi

# Check pod logs for any warnings or errors
kubectl logs -l app.kubernetes.io/name=obi -f
```

## Platform Requirements

> [!WARNING]:
> OBI only works on Linux nodes. Windows is not supported.

- Kubernetes 1.24+
- Helm 3.9+
- Linux kernel version 5.8 or later (or 4.18 for Redhat Enterprise Linux)
- x86_64 or arm64 processor
- Runtime support for eBPF (most modern Linux distributions)

## Security and Capabilities

OBI requires elevated privileges and specific Linux kernel capabilities to function.
For complete security considerations and configuration options, refer to the [OBI Security Documentation].

[OBI Security Documentation]: https://opentelemetry.io/docs/zero-code/obi/security/

### Default Permissions

By default, OBI is configured with the following elevated permissions:

- **Root user** (runAsUser: 0) for kernel access
- **Privileged mode** (privileged: true) for unrestricted access
- **Linux capabilities** for eBPF operations:
  - `CAP_BPF`: Core eBPF functionality
  - `CAP_NET_RAW`: Network packet capture
  - `CAP_NET_ADMIN`: Network filter programs
  - `CAP_PERFMON`: Performance monitoring
  - `CAP_DAC_READ_SEARCH`: Kernel introspection
  - `CAP_CHECKPOINT_RESTORE`: Process introspection
  - `CAP_SYS_PTRACE`: Executable module access

### Reducing Permissions

If your cluster policies require it, you can reduce permissions by:

1. **Modifying Linux capabilities** - Remove unnecessary capabilities from the `securityContext.capabilities` list
2. **Modifying container permissions** - Consult the [security documentation]

[security documentation]: https://opentelemetry.io/docs/zero-code/obi/security/

> [!NOTE]:
> Reducing permissions may limit OBI features.

## Troubleshooting

### Pods not running on Windows nodes

This is expected.
The `nodeSelector: kubernetes.io/os: linux` prevents scheduling on Windows.

### Missing capability warnings

If you see warnings like "Required system capabilities not present", either:

1. Ensure nodes have required kernel support
2. Adjust `kernel.perf_event_paranoid` sysctl on nodes (for [AKS/EKS])
3. Modify the `securityContext.capabilities` to use `CAP_SYS_ADMIN` instead of granular capabilities

[AKS/EKS]: https://opentelemetry.io/docs/zero-code/obi/security/#deploy-on-akseks

### Pod fails to start

1. Check security policies (Pod Security Standards, Network Policies)
2. Ensure adequate CPU/memory resources (default: 100m CPU, 100Mi memory)

## Further Reading

- [OpenTelemetry eBPF Documentation](https://opentelemetry.io/docs/zero-code/obi/)
- [Security & Capabilities](https://opentelemetry.io/docs/zero-code/obi/security/)
- [OBI GitHub Repository](https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation)
- [Linux Capabilities](https://man7.org/linux/man-pages/man7/capabilities.7.html)
