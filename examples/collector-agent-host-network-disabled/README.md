# Example of chart configuration

## Deploy the Splunk Collector in agent mode and disable using the host network

- This configuration will install the collector as an agent deployment only.
- Disabling this value will affect monitoring of some control plane components.
- Enabling the agent service is recommended and also done in this example. Kubernetes services will be deployed to use transmit collector related data.
- Disregard for Windows (unsupported by k8s).
