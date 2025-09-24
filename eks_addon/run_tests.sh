#!/bin/bash
# Tests the Splunk OpenTelemetry Collector EKS Add-on for compliance with AWS requirements.
# This script prepares the chart build and runs tests to verify it meets EKS addon requirements.

# Enable bash strict mode
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/common.sh"

if [[ -z "${K8S_VERSION}" ]]; then
  echo "❌ Error: K8S_VERSION environment variable is required"
  exit 1
fi

# EKS Add-on specific test functions
test_helm_lint() {
  echo "⏳ Running helm lint on ${EKS_CHART_DIR} ..."
  
  if ! helm lint "${EKS_CHART_DIR}"; then
    echo "❌ Helm lint test failed"
    return 1
  fi
  echo "✅ Helm lint test passed"
}

test_helm_template() {
  echo "⏳ Running helm template test with Kubernetes ${K8S_VERSION} ..."
  if ! helm template splunk-otel-collector "${EKS_CHART_DIR}" \
     --kube-version "${K8S_VERSION}" \
     --namespace splunk-otel-collector \
     --include-crds \
     --set k8sVersion="${K8S_VERSION}" \
     --no-hooks > /dev/null; then
    echo "❌ Helm template test failed"
    return 1
  fi
  echo "✅ Helm template test passed"
}

test_schema_validation() {
  echo "⏳ Checking aws_mp_configuration_schema.json ..."
  
  local schema_file="${EKS_CHART_DIR}/aws_mp_configuration_schema.json"

  if [[ ! -f "${schema_file}" ]]; then
    echo "❌ Schema validation test failed: aws_mp_configuration_schema.json not found"
    return 1
  fi
  
  if ! jq empty "${schema_file}" 2>/dev/null; then
    echo "❌ Schema validation test failed: aws_mp_configuration_schema.json is not valid JSON"
    return 1
  fi
  
  echo "✅ Schema validation test passed"
}

test_images() {
  echo "⏳ Verifying images ..."
  
  # Find all repository entries in the values file
  local image_repos=$(yq e '.image | .. | select(has("repository")) | .repository' "${EKS_CHART_DIR}/values.yaml" | sort -u)
  if [[ -z "${image_repos}" ]]; then
    echo "❌ Image verification failed: No image repositories found in values.yaml"
    return 1
  fi
  
  # Check each repository to ensure it's hosted on ECR
  echo "${image_repos}" | while read -r repo; do
    if ! [[ "${repo}" == *".dkr.ecr."*".amazonaws.com/"* || "${repo}" == "public.ecr.aws/"* ]]; then
      echo "❌ Image verification failed: Image is not hosted on ECR: ${repo}"
      return 1
    fi
  done

  # TODO: Add more checks once we automate image publishing:
  # - Check if the image exists in ECR
  # - Check if the image is publicly accessible
  # - Check if the image supports required Linux architectures (e.g., amd64, arm64)

  echo "✅ Image verification passed"
}

test_release_properties() {
  echo "⏳ Checking for forbidden Release properties in templates ..."
  local forbidden_release_refs=$(grep -r "Release\." --include="*.yaml" --include="*.tpl" "${EKS_CHART_DIR}/templates" | 
                              grep -v "Release\.Name" | 
                              grep -v "Release\.Namespace")
  
  if [[ -n "${forbidden_release_refs}" ]]; then
    echo "❌ Found forbidden Release properties in templates:"
    echo "${forbidden_release_refs}"
    return 1
  fi  
  echo "✅ No forbidden Release properties found in templates"
}

prepare_chart

test_helm_lint 
test_helm_template
test_schema_validation
test_images
test_release_properties

echo "🎉 All tests passed!"