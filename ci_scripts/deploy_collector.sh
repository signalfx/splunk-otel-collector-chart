#!/usr/bin/env bash
set -e
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-4 | bash

#Make sure to check and clean previously failed deployment
echo "Checking if previous deployment exist..."
if [ "`helm ls --short`" == "" ]; then
   echo "Nothing to clean, ready for deployment"
else
   helm uninstall $(helm ls --short)
fi
echo "Deploying Splunk OTel Collector for Kubernetes"
helm install ci-sck --set splunkPlatform.index=$CI_INDEX_EVENTS \
--set splunkPlatform.metricsIndex=$CI_INDEX_METRICS \
--set splunkPlatform.token=$CI_SPLUNK_HEC_TOKEN \
--set splunkPlatform.endpoint=https://$CI_SPLUNK_HOST:8088/services/collector \
-f ci_scripts/sck_otel_values.yaml helm-charts/splunk-otel-collector/
#--set containerLogs.containerRuntime=$CONTAINER_RUNTIME \
# Wait for the Collector workloads, including their processors, to become ready.
kubectl rollout status daemonset/ci-sck-splunk-otel-collector-agent --timeout=3m
kubectl rollout status deployment/ci-sck-splunk-otel-collector-k8s-cluster-receiver --timeout=3m
