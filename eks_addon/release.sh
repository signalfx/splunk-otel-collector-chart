#!/bin/bash
# Builds, packages, and pushes artifacts for the Splunk OpenTelemetry Collector EKS Add-on.
#
# Parameters:
#   --dry-run : (Optional) Enables dry-run mode that skips pushing artifacts.
#   --chart-version VERSION : (Optional) Overrides the chart version from Chart.yaml.
#
# Note: This script requires OKTA_AWS_ROLE_ARN to be set in the environment.

# Check required tools
for tool in "okta-aws-login" "aws" "yq" "helm" "docker"; do
  command -v "${tool}" &>/dev/null || { echo "❌ Required command '${tool}' is not installed or not in PATH"; exit 1; }
done

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/common.sh"

# Parse command line arguments
DRY_RUN_PREFIX=""
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

push_chart() {
  local ecr_chart_release="oci://${ECR_HELM_REPO}:${CHART_VERSION}"

  # Check if chart already exists in the registry
  if helm show chart "${ecr_chart_release}" &>/dev/null; then
    echo "❌ Chart already exists in registry: ${ecr_chart_release}. Use --chart-version to push another version of this chart."
    return 1
  fi

  echo "⏳ Packaging and pushing Helm chart ${ecr_chart_release} ..."
  helm package "${EKS_CHART_DIR}" -d "${BUILD_DIR}"
  local package_file="${BUILD_DIR}/splunk-otel-collector-${CHART_VERSION}.tgz"
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
prepare_chart
push_chart
print_summary
