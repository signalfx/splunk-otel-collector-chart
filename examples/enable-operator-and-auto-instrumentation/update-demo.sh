#!/usr/bin/env bash
# Updates the spring-petclinic demo application with the latest upstream changes

# Requirements: yq and kompose
# brew install yq
# brew install kompose

# Set default paths if environment variables are not set
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)
DOCKER_COMPOSE_PATH=${DOCKER_COMPOSE_PATH:-"$SCRIPT_DIR/spring-petclinic/docker-compose.yaml"}
SPRING_PETCLINIC_PATH=${SPRING_PETCLINIC_PATH:-"$SCRIPT_DIR/spring-petclinic/spring-petclinic.yaml"}

# Download the docker-compose file
curl -L https://raw.githubusercontent.com/spring-petclinic/spring-petclinic-microservices/master/docker-compose.yml \
    > "$DOCKER_COMPOSE_PATH"

# Delete extra servers to minimize resource usage
yq -i 'del(.services.grafana-server)' "$DOCKER_COMPOSE_PATH"
yq -i 'del(.services.prometheus-server)' "$DOCKER_COMPOSE_PATH"
yq -i 'del(.services.tracing-server)' "$DOCKER_COMPOSE_PATH"

# Convert docker-compose to Kubernetes YAML and add label
kompose convert \
    --file="$DOCKER_COMPOSE_PATH" \
    --out="$SPRING_PETCLINIC_PATH" \
    --with-kompose-annotation=false

# Add prefix 'spring-petclinic-' to Deployment names
yq eval -i 'select(.kind == "Deployment") .metadata.name |= "spring-petclinic-" + .' "$SPRING_PETCLINIC_PATH"
# Add 'app.kubernetes.io/part-of = spring-petclinic' label to Services, Deployments, and Pods
yq eval -i 'select(.kind == "Service") .metadata.labels += {"app.kubernetes.io/part-of": "spring-petclinic"}' "$SPRING_PETCLINIC_PATH"
yq eval -i 'select(.kind == "Deployment") .metadata.labels += {"app.kubernetes.io/part-of": "spring-petclinic"}' "$SPRING_PETCLINIC_PATH"
yq eval -i 'select(.kind == "Deployment") .spec.template.metadata.labels += {"app.kubernetes.io/part-of": "spring-petclinic"}' "$SPRING_PETCLINIC_PATH"

# Remove the downloaded docker-compose file
rm -rf "$DOCKER_COMPOSE_PATH"

echo "Conversion and label addition completed!"
