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
export CI_SPLUNK_HOST=$(kubectl get pod splunk --template={{.status.podIP}})
echo "Deploying Splunk OTel Collector for Kubernetes"
# TODO: Remove networkExplorer.enabled after https://github.com/signalfx/splunk-otel-collector-chart/issues/896
helm install ci-sck --set splunkPlatform.index=$CI_INDEX_EVENTS \
--set splunkPlatform.metricsIndex=$CI_INDEX_METRICS \
--set splunkPlatform.token=$CI_SPLUNK_HEC_TOKEN \
--set splunkPlatform.endpoint=https://$CI_SPLUNK_HOST:8088/services/collector \
--set networkExplorer.enabled=${NETWORK_EXPLORER_ENABLED:-false} \
--set splunkPlatform.metricsEnabled=true \
--set splunkPlatform.tracesIndex=$CI_INDEX_TRACES \
--set splunkPlatform.tracesEnabled=true \
--set splunkPlatform.token=00000000-0000-0000-0000-0000000000000 \
--set environment=dev \
-f ci_scripts/sck_otel_values.yaml helm-charts/splunk-otel-collector/
#--set containerLogs.containerRuntime=$CONTAINER_RUNTIME \
#wait for deployment to finish
until kubectl get pod | grep Running | [[ $(wc -l) == 1 ]]; do
   sleep 1;
done
