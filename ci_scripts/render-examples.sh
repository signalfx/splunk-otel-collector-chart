#!/bin/bash
# Purpose: Renders Kubernetes manifests from Helm charts for various examples in parallel.
# Notes:
#   - Renders all examples using Helm template.
#   - Uses the default example values file located in each example directory and optionally additional values files passed as parameters.
#   - Supports parallel rendering of examples for efficiency.
#
# Parameters:
#   $@: Additional values files to be applied on top of the default example values file for each Helm chart rendering (optional).
#
# Usage Examples:
#   ./render-examples.sh
#   ./render-examples.sh extra-values.yaml
#   ./render-examples.sh values1.yaml values2.yaml

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
EXAMPLES_DIR="$SCRIPT_DIR/../examples"
source "$SCRIPT_DIR/base_util.sh"

render_task() {
  example_dir=$1
  rendered_manifests_dir="${example_dir}rendered_manifests"
  default_values_yaml=$(ls "${example_dir}" | grep -m 1 'values.yaml')

  if [ -z "$default_values_yaml" ]; then
    echo "No default values.yaml found in ${example_dir}"
    exit 1
  fi

  # Initialize helm values with the default values file
  helm_values=("--values" "${example_dir}${default_values_yaml}")

  # Append additional values files if provided
  for value_file in "${values_files[@]}"; do
    helm_values+=("--values" "${value_file}")
  done

  # Clear out all rendered manifests
  rm -rf "${rendered_manifests_dir}"

  # Generate rendered files
  out=$(helm template \
    --namespace default \
    "${helm_values[@]}" \
    --output-dir "${rendered_manifests_dir}" \
    default helm-charts/splunk-otel-collector)
  if [ $? -ne 0 ]; then
    echo "$default_values_yaml FAIL - helm template: $out"
    exit 1
  fi

  # Redact data that has a unique value per run such as certificate data for the operator webhook
  redact_files "${rendered_manifests_dir}" "**webhook.yaml"

  # Move the chart renders
  cp -rp "${rendered_manifests_dir}/splunk-otel-collector/templates/"* "$rendered_manifests_dir"
  if [ $? -ne 0 ]; then
    echo "${default_values_yaml} FAIL - Move the chart renders"
    exit 1
  fi

  # Move any subchart renders
  if [ -d "${rendered_manifests_dir}/splunk-otel-collector/charts/" ]; then
    subcharts_dir="${rendered_manifests_dir}/splunk-otel-collector/charts"
    subcharts_di=$(find "${subcharts_dir}" -type d -maxdepth 1 -mindepth 1 -exec basename \{\} \;)
    for subchart in ${subcharts_di}; do
      mkdir -p "${rendered_manifests_dir}/${subchart}"
      mv "${subcharts_dir}/${subchart}/templates/"* "${rendered_manifests_dir}/${subchart}"
      if [ $? -ne 0 ]; then
        echo "${default_values_yaml} FAIL - Move subchart renders"
        exit 1
      fi
    done
  fi

  echo "${default_values_yaml} SUCCESS"
}

# Collect additional values files passed as arguments
values_files=("$@")

for example_dir in $EXAMPLES_DIR/*/; do
  render_task "${example_dir}" &
done
wait # Let all the render tasks finish

for example_dir in $EXAMPLES_DIR/*/; do
  rendered_manifests_dir="${example_dir}rendered_manifests"
  if [ ! -d "${rendered_manifests_dir}" ]; then
    echo "Examples were rendered, failure occurred"
    exit 1
  else
    # Temporary space cleanup
    if ls "${example_dir}" | grep -q ".norender."; then
        rm -rf "${rendered_manifests_dir}"
    else
        rm -rf "${rendered_manifests_dir}/splunk-otel-collector"
    fi
    if [ $? -ne 0 ]; then
        echo "${default_values_yaml} FAIL - Temporary space cleanup"
        exit 1
    fi
  fi
done

echo "Examples were rendered successfully"
exit 0
