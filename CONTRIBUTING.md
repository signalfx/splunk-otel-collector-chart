## Building

When changing the helm chart the files under `rendered` need to be rebuilt with `make render`. It's strongly recommended to use [pre-commit](https://pre-commit.com/) which will do this automatically for each commit (as well as run some linting).

## Running locally

It's recommended to use [minikube](https://github.com/kubernetes/minikube) with
[calico networking](https://docs.projectcalico.org/getting-started/kubernetes/) to run splunk-otel-collector locally.

If you run it on Windows or MacOS, use a linux VM driver, e.g. virtualbox.
In that case use the following arguments to start minikube cluster:

```bash
minikube start --cni calico --vm-driver=virtualbox
```
