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

    # Remove objects named opentelemetry-demo-otelcol
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-otelcol")' "$OTEL_DEMO_PATH"

    # Remove objects named opentelemetry-demo-frontendproxy
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-frontendproxy")' "$OTEL_DEMO_PATH"

    # Remove objects named opentelemetry-demo-grafana-test
    yq eval -i 'select(.metadata.name != "opentelemetry-demo-grafana-test")' "$OTEL_DEMO_PATH"

    echo "OpenTelemetry Demo update completed!"
}

# ---- Spring PetClinic Update ----
# Downloads a dockercompose.yaml, uses kompose convert to transform the demo into k8s yaml, and removes extra components.
update_spring_petclinic_demo

# ---- OpenTelemetry Demo Update ----
# Downloads a k8s yaml and removes extra components.
# This demo must be deployed to a namespace called 'otel-demo' or else services will fail deployment.
update_otel_demo
