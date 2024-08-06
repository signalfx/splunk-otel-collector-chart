# Example of chart configuration

## Deploy the Splunk Collector in agent mode and disable using the host network

- This configuration will install the collector as an agent deployment only.
- Disabling this value will affect monitoring of some control plane components.
- Enabling the agent service, as done in this example, is recommended. In this case Kubernetes services will be deployed to transmit collector related data.
- Disregard for Windows (unsupported by k8s).
