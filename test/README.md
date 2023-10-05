# Splunk Platform Functional Test Environment Setup

## Prerequsite
* Python version must be > 3.x
* Kubectl = v1.15.2
* Minikube = v1.20.0
* Helm = 3.3.x
* libseccomp2 and cri-o (optional)

## Setup local environment

#### Start Minikube
    # Set environment variable for container runtime. Options are: docker, cri-o and containerd
      export CONTAINER_RUNTIME=docker

    # Start minikube
      minikube start --driver=docker --container-runtime=$CONTAINER_RUNTIME --cpus 3 --memory 8192 --kubernetes-version=v1.15.2 --no-vtx-check

#### Install Splunk on minikube
    # Use ci_scripts/k8s-splunk.yml file to deploy splunk on minikube
    kubectl apply -f ci_scripts/k8s-splunk.yml

    # Run following command to check if Splunk is ready. User should see "Ansible playbook complete, will begin streaming splunkd_stderr.log"
    kubectl logs splunk -f

    # To be abel to interact with Splunk pod from local workstation, you need to forward local ports to the ports on the Splunk Pod
    # Start a new terminal concole, run following command and keep it running in the background
    kubectl port-forward pods/splunk 8089

    # Start a new terminal concole, forward local port 8000 to the port on Splunk pod (for debugging)
    kubectl port-forward pods/splunk 8000
    You can then vistit Splunk web page: https://localhost:8000

#### Deploy sck otel collector
    # Get Splunk Host IP
    export SPLUNK_HOST=$(kubectl get pod splunk --template={{.status.podIP}})

    # Use ci_scripts/sck_otel_values.yaml file to deploy sck otel collector
    # Default image repository: quay.io/signalfx/splunk-otel-collector
    helm install ci-sck --set splunkPlatform.index=$CI_INDEX_EVENTS \
    --set splunkPlatform.token=$CI_SPLUNK_HEC_TOKEN \
    --set splunkPlatform.endpoint=https://$CI_SPLUNK_HOST:8088/services/collector \
    -f ci_scripts/sck_otel_values.yaml helm-charts/splunk-otel-collector/

#### Deploy log generator
    # Use test/test_setup.yaml file to deploy log generator
    kubectl apply -f test/test_setup.yaml

#### Check data on Splunk
    To see the test events generaged in Splunk, you can vistit Splunk web page: https://localhost:8000
    Search for events by index.
    For example: `index=ci_events`

## Testing Instructions
1. (Optional) Use a virtual environment for the test
    ```
    virtualenv --python=python3.6 venv
    source venv/bin/activate
    ```
2. Install the dependencies
    ```
    pip install -r requirements.txt
    export PYTHONWARNINGS="ignore:Unverified HTTPS request"
    ```
3. Start the test with the required options configured
    ```
    python -m pytest \
    --splunkd-url https://localhost:8089 \
    --splunk-user admin \
    --splunk-password helloworld \
    -p no:warnings -s
    ```
    **Options are:**
    --splunkd-url
    * Description: splunkd url used to send test data to.
    * Default: https://localhost:8089

    --splunk-user
    * Description: splunk username
    * Default: admin

    --splunk-password
    * Description: splunk user password
    * Default: helloworld

