## Running locally

It's recommended to use [minikube](https://github.com/kubernetes/minikube) with
[calico networking](https://docs.projectcalico.org/getting-started/kubernetes/) to run splunk-otel-collector locally.

If you run it on Windows or MacOS, use a linux VM driver, e.g. virtualbox.
In that case use the following arguments to start minikube cluster:

```bash
minikube start --cni calico --vm-driver=virtualbox
```
