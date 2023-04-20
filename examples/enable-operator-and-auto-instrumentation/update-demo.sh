#!/usr/bin/env bash
# Updates the spring-petclinic demo application with the latest upstream changes
# yq and kompose are required to run this
# brew install yq
# brew install kompose
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
curl -L wget https://raw.githubusercontent.com/spring-petclinic/spring-petclinic-microservices/master/docker-compose.yml > $SCRIPT_DIR/spring-petclinic/docker-compose.yaml
# Delete extra servers to minimize resource usage
yq -i 'del(.services.grafana-server)' $SCRIPT_DIR/spring-petclinic/docker-compose.yaml
yq -i 'del(.services.prometheus-server)' $SCRIPT_DIR/spring-petclinic/docker-compose.yaml
yq -i 'del(.services.tracing-server)' $SCRIPT_DIR/spring-petclinic/docker-compose.yaml
kompose convert --file=$SCRIPT_DIR/spring-petclinic/docker-compose.yaml --out=$SCRIPT_DIR/spring-petclinic/02_install_resources.yaml --with-kompose-annotation=false
rm -rf $SCRIPT_DIR/spring-petclinic/docker-compose.yaml
