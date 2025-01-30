#!/bin/bash

# Description:
# This script collects debugging information from a Kubernetes cluster.
# It retrieves networking, firewall, security policies, custom resource definitions (CRDs),
# and logs from specified pods. The outputs are saved to files for each namespace and object type.
# This helps in diagnosing and troubleshooting cluster configurations.
# Finally, it compresses all the collected files into a ZIP archive.
#
# Input Parameters:
# - NAMESPACES: Comma-separated list of namespaces to collect data from. If not specified, the script collects data from all namespaces.
# - K8S_OBJECT_NAME_FILTER: Filter for Kubernetes object names (default: 'splunk|collector|otel|certmanager|test|sck|sock').
#
# Usage:
# 1. Ensure you have `kubectl`, `yq`, and `helm` installed and configured to access your Kubernetes cluster.
# 2. Save the script to a file called `splunk_kubernetes_debug_info.sh`.
# 3. Make the script executable:
#    chmod +x splunk_kubernetes_debug_info.sh
# 4. Run the script:
#    4.1. Via Terminal and Curl:
#         curl -s https://raw.githubusercontent.com/signalfx/splunk-otel-collector-chart/main/tools/splunk_kubernetes_debug_info.sh | bash
#    4.2. Via Terminal and Local Code:
#         ./splunk_kubernetes_debug_info.sh [NAMESPACES=namespace1,namespace2,...] [K8S_OBJECT_NAME_FILTER=splunk|collector|otel|certmanager|test|sck|sock|customname]
#    Note: If no namespaces are specified, the script will collect information from all namespaces.
# Sensitive Data Handling:
# The script attempts to redact sensitive information where possible, including tokens, passwords, and certificates.
# However, users should review the files for any sensitive data before sharing.
#
# Objects Scraped:
# - Pod logs for agent, cluster-receiver, certmanager, operator, gateway, splunk pods
# - Deployments, daemonsets, Helm releases matching K8S_OBJECT_NAME_FILTER
# - NetworkPolicies, Services, Ingress resources, Endpoints, Roles, RoleBindings, Security contexts
# - OpenTelemetry Instrumentation objects
# - Custom Resource Definitions (CRDs), Pod Security Policies (PSPs), Security Context Constraints (SCCs)
# - Cert-manager related objects
# - MutatingWebhookConfiguration objects

# Helper function to write output to a file
write_output() {
  local output="$1"
  local file_name="$2"
  local cmd="$3"

  # Check if output is empty, starts with "No resources found", or "error"
  if [[ -z "$output" || "$output" == "No resources found"* || "$output" == "error"* || "$output" == "Error"* ]]; then
    echo "[$(date)] Skipping $file_name: $output" >> "$temp_dir/errors.txt"
    return
  fi

  # Check if output is in YAML format
  if echo "$output" | yq eval '.' - > /dev/null 2>&1; then
    # Check if output contains empty list using yq
    if [[ $(echo "$output" | yq eval '.kind' -) == "List" ]] && [[ $(echo "$output" | yq eval '.items | length' -) -eq 0 ]]; then
      echo "[$(date)] Skipping $file_name: Empty list" >> "$temp_dir/errors.txt"
      return
    fi
  fi

  # Redact sensitive information from output
  redact_sensitive_info "$output" "$file_name"
}

# Function to collect data for a given namespace
collect_data_namespace() {
   local ns=$1

   object_types=("configmaps" "daemonsets" "deployments" "endpoints" "events" "ingress" "jobs" "networkpolicies" "otelinst" "rolebindings" "roles" "svc")
   for type in "${object_types[@]}"; do
    stdbuf -oL echo "Collecting $type data for $ns namespace with $k8s_object_name_filter name filter"
     if [[ "$type" == "deployment" ||  "$type" == "daemonset" || "$type" == "configmaps" ]]; then
       kubectl get "$type" -n "$ns" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep -E "$k8s_object_name_filter" | while read object; do
         cmd="kubectl get $type $object -n $ns -o yaml"
         output=$(eval "$cmd")
         write_output "$output" "$temp_dir/namespace_${ns}_${type}_${object}.yaml" "$cmd"
       done
     else
       kubectl get "$type" -n "$ns" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | while read object; do
         cmd="kubectl get $type $object -n $ns -o yaml"
         output=$(eval "$cmd")
         write_output "$output" "$temp_dir/namespace_${ns}_${type}_${object}.yaml" "$cmd"
       done
     fi
   done

  # Collect logs from specific pods
  pods=$(kubectl get pods -n "$ns" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep -E "$k8s_object_name_filter")
  # Collect logs from a single agent pod
  agent_pod=$(echo "$pods" | grep "agent" | head -n 1)
  if [ -n "$agent_pod" ]; then
    cmd="kubectl logs $agent_pod -n $ns"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/namespace_${ns}_logs_pod_${agent_pod}.log" "$cmd"
    pods=$(echo "$pods" | grep -v "$agent_pod")
  fi

  # Collect logs from a single cluster-receiver pod
  cluster_receiver_pod=$(echo "$pods" | grep "cluster-receiver" | head -n 1)
  if [ -n "$cluster_receiver_pod" ]; then
    cmd="kubectl logs $cluster_receiver_pod -n $ns"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/namespace_${ns}_logs_pod_${cluster_receiver_pod}.log" "$cmd"
    pods=$(echo "$pods" | grep -v "$cluster_receiver_pod")
  fi

  # Collect logs from all certmanager pods
  certmanager_pods=$(echo "$pods" | grep "certmanager")
  for pod in $certmanager_pods; do
    cmd="kubectl logs $pod -n $ns"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/namespace_${ns}_logs_pod_${pod}.log" "$cmd"
  done
  pods=$(echo "$pods" | grep -v "certmanager")

  # Collect logs from all operator pods
  operator_pods=$(echo "$pods" | grep "operator")
  for pod in $operator_pods; do
    cmd="kubectl logs $pod -n $ns"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/namespace_${ns}_logs_pod_${pod}.log" "$cmd"
  done
  pods=$(echo "$pods" | grep -v "operator")

  # Collect logs from a single Splunk pod
  splunk_pod=$(kubectl get pods -n "$ns" -l app=splunk -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
  if [ -n "$splunk_pod" ]; then
    echo "Getting logs for pod $splunk_pod in namespace ${ns}"
    cmd="kubectl logs -n ${ns} $splunk_pod"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/namespace_${ns}_logs_pod_${splunk_pod}.log" "$cmd"
  fi

  # Collect pod spec and logs for specific annotations
  annotations=(
    "instrumentation.opentelemetry.io/inject-java"
    "instrumentation.opentelemetry.io/inject-python"
    "instrumentation.opentelemetry.io/inject-dotnet"
    "instrumentation.opentelemetry.io/inject-go"
    "instrumentation.opentelemetry.io/inject-nodejs"
    "instrumentation.opentelemetry.io/inject-nginx"
    "instrumentation.opentelemetry.io/inject-sdk"
    "instrumentation.opentelemetry.io/inject-apache-httpd"
  )

  for annotation in "${annotations[@]}"; do
    pod_with_annotation=$(kubectl get pods -n "$ns" -o jsonpath="{range .items[?(@.metadata.annotations['$annotation'])]}{.metadata.name}{'\n'}{end}" | head -n 1)
    if [ -n "$pod_with_annotation" ]; then
      cmd="kubectl get pod $pod_with_annotation -n $ns -o yaml"
      output=$(eval "$cmd")
      write_output "$output" "$temp_dir/namespace_${ns}_pod_spec_${pod_with_annotation}.yaml" "$cmd"
      cmd="kubectl logs $pod_with_annotation -n $ns"
      output=$(eval "$cmd")
      write_output "$output" "$temp_dir/namespace_${ns}_logs_pod_${pod_with_annotation}.log" "$cmd"
    fi
  done
}

# Function to collect cluster-wide data
collect_data_cluster() {
  echo "Collecting cluster-wide data..."

  echo "Basic Cluster Configurations:" >> "$output_file"
  echo "Cluster Name: $(kubectl config view --minify -o jsonpath='{.clusters[].name}')" >> "$output_file"
  echo "Kubernetes Version:" >> "$output_file"
  kubectl version >> "$output_file"
  echo "Number of Namespaces:" >> "$output_file"
  kubectl get namespaces | wc -l >> "$output_file"
  echo "Namespaces: $(kubectl get namespaces -o jsonpath='{.items[*].metadata.name}')" >> "$output_file"
  echo "Number of Running Nodes:" >> "$output_file"
  kubectl get nodes | wc -l >> "$output_file"
  echo "Number of Running Pods:" >> "$output_file"
  kubectl get pods --all-namespaces --field-selector=status.phase=Running | wc -l >> "$output_file"
  echo "Splunk Related Pods:" >> "$output_file"
  kubectl get pods --all-namespaces | (head -n 1 && grep -E "$k8s_object_name_filter") >> "$output_file"
  echo "---" >> "$output_file"

  echo "Collecting custom resource definitions..."
  cmd="kubectl get crds -o yaml"
  output=$(eval "$cmd")
  write_output "$output" "$temp_dir/cluster_custom_resource_definitions.yaml" "$cmd"

  echo "Checking for cert-manager installation..."
  cert_manager_pods=$(kubectl get pods --all-namespaces -l app=cert-manager --no-headers)
  if [ -n "$cert_manager_pods" ]; then
    echo "Cert-manager is installed. Collecting related objects..."
    cmd="kubectl get Issuers,ClusterIssuers,Certificates,CertificateRequests,Orders,Challenges --all-namespaces -o yaml; kubectl describe Issuers,ClusterIssuers,Certificates,CertificateRequests,Orders,Challenges --all-namespaces"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/cluster_cert_manager_objects.yaml" "$cmd"
  fi

  echo "Collecting Helm values for relevant releases..."
  helm list -A | grep -E "$k8s_object_name_filter" | awk '{print $1, $2}' | while read release namespace; do
    cmd="helm get values $release -n $namespace"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/helm_values_${release}_${namespace}.yaml" "$cmd"
  done
}

collect_cluster_resources() {
  # List of cluster-scoped resource types to collect
  cluster_object_types=(
    "crds"
    "psp"
    "scc"
    "mutatingwebhookconfiguration.admissionregistration.k8s.io"
    "validatingwebhookconfiguration.admissionregistration.k8s.io"
  )

  for type in "${cluster_object_types[@]}"; do
    echo "Collecting $type cluster-scoped resources..."

    # Fetch each object's name
    kubectl get "$type" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | while read object; do
      # Get the API version for this object, fallback to "unknown"
      api_version=$(kubectl get "$type" "$object" -o jsonpath='{.apiVersion}' 2>/dev/null || echo "unknown")
      api_version=${api_version//\//_} # Sanitize slashes in API version

      # Collect YAML output
      cmd="kubectl get $type $object -o yaml"
      output=$(eval "$cmd")
      write_output "$output" "$temp_dir/cluster_${type//./_}_${api_version}_${object}.yaml" "$cmd"
    done
  done
}

# Parse input parameters
namespaces=""
k8s_object_name_filter="splunk|collector|otel|certmanager|test|sck|sock"

for arg in "$@"; do
  case $arg in
    NAMESPACES=*)
      namespaces="${arg#*=}"
      ;;
    K8S_OBJECT_NAME_FILTER=*)
      k8s_object_name_filter="${arg#*=}"
      ;;
    *)
      echo "Unknown parameter: $arg"
      exit 1
      ;;
  esac
done

# Collect data from all namespaces if no namespaces are specified
if [[ -z "$namespaces" ]]; then
  # Get all namespaces and convert the string into an array
  IFS=' ' read -r -a namespaces_array <<< "$(kubectl get namespaces -o jsonpath='{.items[*].metadata.name}')"
else
  # Split the specified namespaces string into an array
  IFS=',' read -r -a namespaces_array <<< "$namespaces"
fi

echo "Namespaces: ${namespaces_array[@]}"
echo "Kubernetes object name filter: $k8s_object_name_filter"

# Create a temporary directory with a unique name
temp_dir=$(mktemp -d -t splunk_kubernetes_debug_info_XXXXXX)
if [[ ! -d "$temp_dir" ]]; then
  echo "Failed to create temporary directory"
  exit 1
fi

# Output file for basic cluster information
output_file="$temp_dir/cluster.txt"

# Print script start time
script_start_time=$(date +"%Y-%m-%d %H:%M:%S")
echo "Script start time: $script_start_time"
echo "Script start time: $script_start_time" >> "$output_file"

# Collect cluster instance specific data
collect_data_cluster

# Collect cluster scoped resources data
collect_cluster_resources

# Function to manage parallel processing of namespaces
collect_data_namespace_namespaces() {
  local parallelism=20
  local pids=()

  for ns in "${namespaces_array[@]}"; do
    collect_data_namespace "$ns" &
    pids+=($!)

    if [[ ${#pids[@]} -ge $parallelism ]]; then
      for pid in "${pids[@]}"; do
        wait "$pid"
      done
      pids=()
    fi
  done

  # Wait for any remaining background processes to complete
  for pid in "${pids[@]}"; do
    wait "$pid"
  done
}

# Process namespaces in parallel
collect_data_namespace_namespaces

# Print script end time
script_end_time=$(date +"%Y-%m-%d %H:%M:%S")
echo "Script end time: $script_end_time"
echo "Script end time: $script_end_time" >> "$output_file"

# Create a ZIP archive of all the collected YAML files
output_zip="splunk_kubernetes_debug_info_$(date +%Y%m%d_%H%M%S).zip"
echo "Creating ZIP archive: $output_zip"

# Find and delete empty files before creating the ZIP archive
find "$temp_dir" -type f -empty -delete

zip -j -r "$output_zip" "$temp_dir"

# Clean up the temporary directory
rm -rf "$temp_dir"

echo "Data collection complete. Output files are available in the ZIP archive: $output_zip"
