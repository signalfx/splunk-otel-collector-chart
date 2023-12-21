#!/usr/bin/env bash
set -euo pipefail
# Purpose: Update demo applications with the latest upstream changes.
# Notes:
#   This script performs updates for demo applications like Spring PetClinic and OpenTelemetry Demo.
# Requirements:
#   - yq: A portable command-line YAML processor.
#   - kompose: A conversion tool to go from Docker Compose to Kubernetes.
#   Both can be installed using brew:
#       brew install yq
#       brew install kompose
#
# Example Usage:
#   ./update_demos.sh

# Set default paths if environment variables are not set
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)

function update_spring_petclinic_demo {
    SPRING_PETCLINIC_DOCKER_COMPOSE_PATH=${SPRING_PETCLINIC_DOCKER_COMPOSE_PATH:-"$SCRIPT_DIR/spring-petclinic/docker-compose.yaml"}
    SPRING_PETCLINIC_PATH=${SPRING_PETCLINIC_PATH:-"$SCRIPT_DIR/spring-petclinic/spring-petclinic.yaml"}

    # Download the docker-compose file
    curl -L https://raw.githubusercontent.com/spring-petclinic/spring-petclinic-microservices/master/docker-compose.yml \
        > "$SPRING_PETCLINIC_DOCKER_COMPOSE_PATH"

    # Delete extra servers to minimize resource usage
    yq -i 'del(.services.grafana-server)' "$SPRING_PETCLINIC_DOCKER_COMPOSE_PATH"
    yq -i 'del(.services.prometheus-server)' "$SPRING_PETCLINIC_DOCKER_COMPOSE_PATH"
    yq -i 'del(.services.tracing-server)' "$SPRING_PETCLINIC_DOCKER_COMPOSE_PATH"

    # Convert docker-compose to Kubernetes YAML and add label
    kompose convert \
        --file="$SPRING_PETCLINIC_DOCKER_COMPOSE_PATH" \
        --out="$SPRING_PETCLINIC_PATH" \
        --with-kompose-annotation=false

    # Add prefix 'spring-petclinic-' to Deployment names
    yq eval -i 'select(.kind == "Deployment") .metadata.name |= "spring-petclinic-" + .' "$SPRING_PETCLINIC_PATH"
    # Add 'app.kubernetes.io/part-of = spring-petclinic' label to Services, Deployments, and Pods
    yq eval -i 'select(.kind == "Service") .metadata.labels += {"app.kubernetes.io/part-of": "spring-petclinic"}' "$SPRING_PETCLINIC_PATH"
    yq eval -i 'select(.kind == "Deployment") .metadata.labels += {"app.kubernetes.io/part-of": "spring-petclinic"}' "$SPRING_PETCLINIC_PATH"
    yq eval -i 'select(.kind == "Deployment") .spec.template.metadata.labels += {"app.kubernetes.io/part-of": "spring-petclinic"}' "$SPRING_PETCLINIC_PATH"

    # Remove the downloaded docker-compose file
    rm -rf "$SPRING_PETCLINIC_DOCKER_COMPOSE_PATH"

    echo "Spring PetClinic update completed!"
}

function update_otel_demo {
    OTEL_DEMO_PATH=${OTEL_DEMO_PATH:-"$SCRIPT_DIR/otel-demo/otel-demo.yaml"}

    # Download the YAML file
    curl -L https://raw.githubusercontent.com/open-telemetry/opentelemetry-demo/main/kubernetes/opentelemetry-demo.yaml \
        > "$OTEL_DEMO_PATH"

    # Remove all env vars with OTEL_* prefixes from Deployment objects. The Operator will add these.
    yq eval -i '
        (select(.kind == "Deployment") | .spec.template.spec.containers[].env) |= map(select(.name | test("^OTEL_") | not))
    ' "$OTEL_DEMO_PATH"
    # Add back OTEL_SERVICE_NAME env var ONLY to opentelemetry-demo-recommendationservice deployment with the specified value.
    # This python deployment requires this env var to start the application, see: https://github.com/open-telemetry/opentelemetry-demo/blob/fc01d8f46f9d2a1cac6a4e674662fbfe8b66f3c4/src/recommendationservice/recommendation_server.py#L127
    yq eval -i '
        (select(.kind == "Deployment" and .metadata.name == "opentelemetry-demo-recommendationservice") | .spec.template.spec.containers[]) |= .env += [{"name": "OTEL_SERVICE_NAME", "valueFrom": {"fieldRef": {"apiVersion": "v1", "fieldPath": "metadata.labels['\''app.kubernetes.io/component'\'']"}}}]
    ' "$OTEL_DEMO_PATH"
    # Increase the memory limit for the NodeJS frontend deployment to avoid OOMing after auto-instrumentation
    yq eval -i '
        (select(.kind == "Deployment" and .metadata.name == "opentelemetry-demo-frontend") | .spec.template.spec.containers[0].resources.limits.memory) = "300Mi"
    ' "$OTEL_DEMO_PATH"
    # Update the frontendproxy to be frontend since we exclude the frontendproxy deployment
    awk '
    /name: LOCUST_HOST/ { print; getline; sub("frontendproxy", "frontend"); print; next }
    { print }
    ' "$OTEL_DEMO_PATH" > temp_file && mv temp_file "$OTEL_DEMO_PATH"

    # Remove all env vars with PUBLIC_OTEL_* prefixes from Deployment objects.
    # This is a special case that was likely for legacy support, only the NodeJS opentelemetry-demo-frontend deployment was using it.
    # These env vars can cause compatibility issues with Splunk instrumentation and they don't seem to follow the OpenTelemetry spec.
    yq eval -i '
        (select(.kind == "Deployment") | .spec.template.spec.containers[].env) |= map(select(.name | test("^PUBLIC_OTEL_") | not))
    ' "$OTEL_DEMO_PATH"

    # Remove objects by name for components we want to exclude
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-otelcol")' "$OTEL_DEMO_PATH"
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-frontendproxy")' "$OTEL_DEMO_PATH"
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-grafana")' "$OTEL_DEMO_PATH"
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-grafana-dashboards")' "$OTEL_DEMO_PATH"
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-grafana-clusterrole")' "$OTEL_DEMO_PATH"
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-grafana-clusterrolebinding")' "$OTEL_DEMO_PATH"
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-grafana-test")' "$OTEL_DEMO_PATH"
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-prometheus-server")' "$OTEL_DEMO_PATH"
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-jaeger")' "$OTEL_DEMO_PATH"
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-jaeger-collector")' "$OTEL_DEMO_PATH"
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-jaeger-query")' "$OTEL_DEMO_PATH"
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-jaeger-agent")' "$OTEL_DEMO_PATH"

    echo "OpenTelemetry Demo update completed!"
}

# ---- Spring PetClinic Update ----
# Downloads a dockercompose.yaml, uses kompose convert to transform the demo into k8s yaml, and removes extra components.
update_spring_petclinic_demo

# ---- OpenTelemetry Demo Update ----
# Downloads a k8s yaml and removes extra components.
# This demo must be deployed to a namespace called 'otel-demo' or else services will fail deployment.
update_otel_demo
