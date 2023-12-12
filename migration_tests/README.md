# Migration tests

This folder contains a set of deployments used to show the migration steps taken by the Helm chart from a SCK deployment.

You can see how this test executes as part of Github actions in `.github/workflows/migration_tests.yaml`

## Local development

You can recreate the execution of the step on your machine with the following set up steps.

1. Check out the repository
1. Install kubectl, helm as per root README.
1. Set up a kind cluster with a few default utilities:
   ```
   export KUBECONFIG=/tmp/kube-config-splunk-otel-collector-chart-migration-testing
   export K8S_VERSION=v1.28.0
   kind create cluster --kubeconfig=$KUBECONFIG --config=.github/workflows/configs/kind-config.yaml --image=kindest/node:$K8S_VERSION --name=migration-tests
   kubectl get csr -o=jsonpath='{range.items[?(@.spec.signerName=="kubernetes.io/kubelet-serving")]}{.metadata.name}{" "}{end}' | xargs kubectl certificate approve
   make cert-manager
   ```
1. Deploy a collector as a service that can act as a log sink for HEC traffic, alongside with a deployment that outputs logs every second:
   ```
   kubectl apply -f migration_tests/collector_deployment.yaml
   ```
1. Deploy SCK
   ```
   helm repo add sck https://splunk.github.io/splunk-connect-for-kubernetes/
   helm install --wait sck -f migration_tests/sck_values.yaml sck/splunk-connect-for-kubernetes
   ```
1. Now that SCK is deployed, wait 10s to generate enough logs:
   ```
   sleep 10
   ```
1. Uninstall SCK:
   ```
   helm uninstall --wait sck
   ```
1. Collect logs collected so far:
   ```
   pod=$(kubectl get pods -A | grep logsink | awk '{print $2}')
   kubectl exec -it $pod -- cat /tmp/output.log | grep -Eo '"body":{"stringValue":"APP LOG LINE \d+' | awk '{print $4}' > sck_logs.log
   ```
1. Install SOCK:
   ```
   helm install --wait sock -f migration_tests/sock_values.yaml helm-charts/splunk-otel-collector
   ```
1. Wait additional 10s to produce more logs
   ```
   sleep 10
   ```
1. Check the collector deployment logs:
   ```
   pod=$(kubectl get pods -A | grep logsink | awk '{print $2}')
   kubectl exec -it $pod -- cat /tmp/output.log | grep -Eo '"body":{"stringValue":"APP LOG LINE \d+' | awk '{print $4}' > sock_logs.log
   ```
1. Check we have no duplicates:
   ```
   dupes=$(cat sock_logs.log | sort | uniq -d)
   if [[ -n $dupes ]]; then
     echo "Duplicates detected: $dupes"
   fi
   ```
