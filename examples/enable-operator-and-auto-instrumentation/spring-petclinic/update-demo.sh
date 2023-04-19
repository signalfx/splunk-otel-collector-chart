#!/usr/bin/env bash
# Updates the spring-petclinic demo application with the latest upstream changes
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
curl -L wget https://raw.githubusercontent.com/spring-petclinic/spring-petclinic-microservices/master/docker-compose.yml > $SCRIPT_DIR/docker-compose.yaml
kompose convert --file=$SCRIPT_DIR/docker-compose.yaml --out=$SCRIPT_DIR/02_install_resources.yaml --with-kompose-annotation=false
rm -rf $SCRIPT_DIR/docker-compose.yaml
