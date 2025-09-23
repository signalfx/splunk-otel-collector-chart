#!/bin/bash
# Builds, packages, and pushes artifacts for the Splunk OpenTelemetry Collector EKS Add-on.
#
# Parameters:
#   --dry-run : (Optional) Enables dry-run mode that skips pushing artifacts.
#   --chart-version VERSION : (Optional) Overrides the chart version from Chart.yaml.
#
# Note: This script requires OKTA_AWS_ROLE_ARN to be set in the environment.

# Enable bash strict mode to fail fast
set -euo pipefail

# Check required tools
for tool in "okta-aws-login" "aws" "yq" "helm" "docker"; do
  command -v "${tool}" &>/dev/null || { echo "❌ Required command '${tool}' is not installed or not in PATH"; exit 1; }
done

# Parse command line arguments
DRY_RUN_PREFIX=""
CHART_VERSION=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN_PREFIX="echo 🚧 [DRY-RUN] "
      shift
      ;;
    --chart-version)
      CHART_VERSION="$2"
      echo "ℹ️ Chart version override provided: $CHART_VERSION"
      shift 2
      ;;
    *)
      echo "❌ Unknown argument: $1"
      exit 1
      ;;
  esac
done

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
CHART_DIR="$SCRIPT_DIR/../helm-charts/splunk-otel-collector"
if [[ -z "$CHART_VERSION" ]]; then
  CHART_VERSION=$(yq e ".version" "${CHART_DIR}/Chart.yaml")
fi
CHART_APPVERSION=$(yq e ".appVersion" "${CHART_DIR}/Chart.yaml")
ECR_REGION="us-east-1"
ECR_REGISTRY="709825985650.dkr.ecr.${ECR_REGION}.amazonaws.com"
ECR_OTELCOL_REPO="${ECR_REGISTRY}/splunk/images/splunk-otel-collector"
ECR_FLUENTD_REPO="${ECR_REGISTRY}/splunk/docker.io/splunk/fluentd-hec"
ECR_FLUENTD_REPO_TAG="1.3.3-linux"
ECR_HELM_NAMESPACE="${ECR_REGISTRY}/splunk/charts"
ECR_HELM_REPO="${ECR_HELM_NAMESPACE}/splunk-otel-collector"

aws_okta_auth() {
  echo "🌐 Authenticating with Okta (browser will open)..."
  okta-aws-login > /tmp/okta_creds.txt 2>&1 || { echo "❌ AWS Authentication failed"; rm -f /tmp/okta_creds.txt; exit 1; }

  # Extract and evaluate AWS credentials
  eval "$(grep -E "^(AWS_|export)" /tmp/okta_creds.txt)"
  rm -f /tmp/okta_creds.txt
  AUTH_AWS_ECR_PASSWORD=$(aws ecr get-login-password --region "$ECR_REGION")
  [[ -n "$AUTH_AWS_ECR_PASSWORD" ]] || { echo "❌ Failed to get ECR login password."; exit 1; }

  # Login to ECR for Docker and Helm
  echo "$AUTH_AWS_ECR_PASSWORD" | docker login --username AWS --password-stdin "$ECR_REGISTRY"
  echo "$AUTH_AWS_ECR_PASSWORD" | helm registry login --username AWS --password-stdin "$ECR_REGISTRY"
  echo "✅ AWS Authentication successful"
}

copy_docker_image_to_ecr() {
  # Get the otelcol repository from values.yaml
  local src_repo=$(yq e ".image.otelcol.repository" "${CHART_DIR}/values.yaml")
  [[ -n "$src_repo" && "$src_repo" != "null" ]] || { echo "❌ Error: Could not find otelcol repository in values.yaml"; exit 1; }

  local src="${src_repo}:${CHART_APPVERSION}"
  local dest="${ECR_OTELCOL_REPO}:${CHART_APPVERSION}"

  echo "⏳ Copying otelcol image from ${src} to ${dest} ..."

  # Get supported Linux architectures from the source manifest and copy each to ECR
  local arch_digests=$(docker manifest inspect "${src}" | jq -c '.manifests[] | select(.platform.os == "linux")')
  echo "${arch_digests}" | while read -r manifest; do
    local arch=$(echo "${manifest}" | jq -r '.platform.architecture')
    local digest=$(echo "${manifest}" | jq -r '.digest')
    local src_digest="${src_repo}@${digest}"
    local dest_arch="${dest}-${arch}"
    ${DRY_RUN_PREFIX} docker pull "${src_digest}"
    ${DRY_RUN_PREFIX} docker tag "${src_digest}" "${dest_arch}"
    ${DRY_RUN_PREFIX} docker push "${dest_arch}"
    ${DRY_RUN_PREFIX} docker manifest create "${dest}" "${dest_arch}" --amend
    ${DRY_RUN_PREFIX} docker manifest annotate "${dest}" "${dest_arch}" --os linux --arch "${arch}"
  done

  ${DRY_RUN_PREFIX} docker manifest push ${dest}
  echo "✅ Successfully copied multi-arch image: ${src} → ${dest}"
}

# Function to modify the Helm chart to meet EKS Add-on requirements
modify_helm_chart() {
  local chart_dir="$1"
  local overrides_dir="${SCRIPT_DIR}/overrides"

  echo "⏳ Removing subcharts ..."
  rm -rf "${chart_dir}/charts"

  echo "⏳ Modifying Helm chart in ${chart_dir} using overrides from ${overrides_dir} ..."
  for override in "${overrides_dir}"/*.yaml; do
    if [[ -f "${override}" ]]; then
      local override_basename=$(basename "${override}")
      local target_file="${chart_dir}/${override_basename}"

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
  cp "${tmp_chart_dir}/values.schema.json" "${tmp_chart_dir}/aws_mp_configuration_schema.json"
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
      yq e "del(.properties.\"${prop}\")" -i "${tmp_chart_dir}/aws_mp_configuration_schema.json"
  done

  echo "✅ Successfully modified the Helm chart for EKS Add-on compliance"
}

package_and_push_helm_chart() {
  local ecr_chart_release="oci://${ECR_HELM_REPO}:${CHART_VERSION}"

  # Check if chart already exists in the registry
  if helm show chart "${ecr_chart_release}" &>/dev/null; then
    echo "❌ Chart already exists in registry: ${ecr_chart_release}. Use --chart-version to push another version of this chart."
    return 1
  fi

  # Copy chart to a temporary build directory
  local build_dir="${SCRIPT_DIR}/build"
  rm -rf "${build_dir}"
  mkdir -p "${build_dir}"
  cp -R "${CHART_DIR}" "${build_dir}/"

  local tmp_chart_dir="${build_dir}/splunk-otel-collector"
  modify_helm_chart "${tmp_chart_dir}"

  echo "⏳ Packaging and pushing Helm chart ${ecr_chart_release} ..."

  # Package the chart
  helm package "${tmp_chart_dir}" -d "${build_dir}"
  local package_file="${build_dir}/splunk-otel-collector-${CHART_VERSION}.tgz"

  # Push the chart from the build location
  ${DRY_RUN_PREFIX} helm push ${package_file} oci://${ECR_HELM_NAMESPACE}

  echo "✅ Successfully pushed Helm chart to ${ecr_chart_release}"
}

print_summary() {
  echo ""
  echo "📋 Use these image references when creating the EKS Add-on release."
  echo "  Helm Chart:"
  echo "  - ${ECR_HELM_REPO}:${CHART_VERSION}"
  echo "  Container Images:"
  echo "  - ${ECR_OTELCOL_REPO}:${CHART_APPVERSION}"
  echo "  - ${ECR_FLUENTD_REPO}:${ECR_FLUENTD_REPO_TAG}"
}

aws_okta_auth
copy_docker_image_to_ecr
package_and_push_helm_chart
print_summary
