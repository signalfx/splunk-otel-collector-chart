#!/bin/bash
# Common functions for EKS Add-on preparation and testing

# Enable bash strict mode
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
CHART_DIR="$SCRIPT_DIR/../helm-charts/splunk-otel-collector"
ORIG_CHART_DIR="$SCRIPT_DIR/../helm-charts/splunk-otel-collector"
BUILD_DIR="${SCRIPT_DIR}/build"
EKS_CHART_OVERRIDES_DIR="${SCRIPT_DIR}/overrides"
EKS_CHART_DIR="${BUILD_DIR}/splunk-otel-collector"

CHART_VERSION=$(yq e ".version" "${CHART_DIR}/Chart.yaml")
CHART_APPVERSION=$(yq e ".appVersion" "${CHART_DIR}/Chart.yaml")

ECR_REGION="us-east-1"
ECR_REGISTRY="709825985650.dkr.ecr.${ECR_REGION}.amazonaws.com"
ECR_OTELCOL_REPO="${ECR_REGISTRY}/splunk/images/splunk-otel-collector"
ECR_FLUENTD_REPO="${ECR_REGISTRY}/splunk/docker.io/splunk/fluentd-hec"
ECR_FLUENTD_REPO_TAG="1.3.3-linux"
ECR_HELM_NAMESPACE="${ECR_REGISTRY}/splunk/charts"
ECR_HELM_REPO="${ECR_HELM_NAMESPACE}/splunk-otel-collector"

# Function to prepare the chart for building or testing
prepare_chart() {
  echo "⏳ Preparing chart for building/testing..."
  
  # Copy chart to a temporary build directory
  rm -rf "${BUILD_DIR}"
  mkdir -p "${BUILD_DIR}"
  cp -R "${ORIG_CHART_DIR}" "${BUILD_DIR}/"

  echo "⏳ Removing subcharts ..."
  rm -rf "${EKS_CHART_DIR}/charts"

  echo "⏳ Modifying Helm chart in ${EKS_CHART_DIR} using overrides from ${EKS_CHART_OVERRIDES_DIR} ..."
  for override in "${EKS_CHART_OVERRIDES_DIR}"/*.yaml; do
    if [[ -f "${override}" ]]; then
      local override_basename=$(basename "${override}")
      local target_file="${EKS_CHART_DIR}/${override_basename}"

      # Process environment variables
      local tmp_file="/tmp/${override_basename}.expanded"
      eval "cat <<EOF
$(cat "${override}")
EOF" > "${tmp_file}"

      # Merge override with the corresponding file in the chart
      yq eval-all 'select(fileIndex==0) * select(fileIndex==1)' -i "${target_file}" "${tmp_file}"
      rm -f "${tmp_file}"
    fi
  done

  echo "⏳ Moving values.schema.json to aws_mp_configuration_schema.json and removing unsupported properties ..."
  cp "${EKS_CHART_DIR}/values.schema.json" "${EKS_CHART_DIR}/aws_mp_configuration_schema.json"
  disabled_properties=(
    "enabled"
    "operatorcrds"
    "operator-crds"
    "operator"
    "opentelemetry-operator"
    "instrumentation"
    "certmanager"
    "cert-manager"
    "targetAllocator"
  )
  for prop in "${disabled_properties[@]}"; do
      yq e "del(.properties.\"${prop}\")" -i "${EKS_CHART_DIR}/aws_mp_configuration_schema.json"
  done

  echo "✅ Successfully prepared chart for for EKS Add-on at ${EKS_CHART_DIR}"
}
