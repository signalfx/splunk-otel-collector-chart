#!/usr/bin/env bash
set -e
curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash

#Make sure to check and clean previously failed deployment
echo "Checking if previous deployment exist..."
if [ "`helm ls --short`" == "" ]; then
   echo "Nothing to clean, ready for deployment"
else
   helm delete $(helm ls --short)
fi
echo "Deploying Splunk OTel Collector for Kubernetes"
helm install ci-sck --set splunkPlatform.index=$CI_INDEX_EVENTS \
--set splunkPlatform.metricsIndex=$CI_INDEX_METRICS \
--set splunkPlatform.token=$CI_SPLUNK_HEC_TOKEN \
--set splunkPlatform.endpoint=https://$CI_SPLUNK_HOST:8088/services/collector \
-f ci_scripts/sck_otel_values.yaml helm-charts/splunk-otel-collector/
#--set containerLogs.containerRuntime=$CONTAINER_RUNTIME \
#wait for deployment to finish
until kubectl get pod | grep Running | [[ $(wc -l) == 1 ]]; do
   sleep 1;
done
