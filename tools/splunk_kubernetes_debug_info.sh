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
#    4.1. In terminal run this to collect debug info at a small scope, mostly related to this charts objects:
#         curl -s https://raw.githubusercontent.com/signalfx/splunk-otel-collector-chart/main/tools/splunk_kubernetes_debug_info.sh | bash
#    4.2. In terminal run this to collect debug info at a larger scope, mostly used to see if other user consutomizations are affecting this chart:
#         curl -s https://raw.githubusercontent.com/signalfx/splunk-otel-collector-chart/main/tools/splunk_kubernetes_debug_info.sh | bash -s -- K8S_OBJECT_NAME_FILTER=".*"
#    4.3. In terminal for running locally with any options:
#         ./splunk_kubernetes_debug_info.sh [NAMESPACES=namespace1,namespace2,...] [K8S_OBJECT_NAME_FILTER=splunk|collector|otel|certmanager|test|sck|sock|customname]
#    Note: If no namespaces are specified, the script will collect information from all namespaces.
# Sensitive Data Handling:
# The script attempts to redact sensitive information where possible, including tokens, passwords, and certificates.
# However, users should review the files for any sensitive data before sharing.
#
# Object Definitions Collected:
# - See the cluster_object_types and namespace_object_types listed below.
# - Custom Resource Definitions (CRDs): Dynamically fetched at runtime (both namespace-scoped and cluster-scoped) as they can impact this chart.

  cluster_object_types=(
    "crds"
    "mutatingwebhookconfiguration.admissionregistration.k8s.io"
    "validatingwebhookconfiguration.admissionregistration.k8s.io"
  )

  namespace_object_types=(
    "configmaps"
    "daemonsets"
    "deployments"
    "endpoints"
    "ingress"
    "networkpolicies"
    "rolebindings"
    "roles"
    "svc"
  )

# Helper function to fetch CRDs and extend both namespace and cluster object types
extend_object_types_lists_with_crds() {
  echo "Fetching namespace-scoped CRDs from the cluster..."
  namespace_crd_names=$(kubectl get crds -o jsonpath='{range .items[?(@.spec.scope=="Namespaced")]}{.metadata.name}{"\n"}{end}' 2>/dev/null)

  if [[ -n "$namespace_crd_names" ]]; then
    namespace_crd_array=($namespace_crd_names)
    namespace_object_types+=("${namespace_crd_array[@]}")
    echo "Extended namespace_object_types with namespace-scoped CRDs:"
    printf "%s\n" "${namespace_object_types[@]}"
  else
    echo "No namespace-scoped CRDs found or unable to fetch CRDs."
  fi

  echo "Fetching cluster-scoped CRDs from the cluster..."
  cluster_crd_names=$(kubectl get crds -o jsonpath='{range .items[?(@.spec.scope=="Cluster")]}{.metadata.name}{"\n"}{end}' 2>/dev/null)

  if [[ -n "$cluster_crd_names" ]]; then
    cluster_crd_array=($cluster_crd_names)
    cluster_object_types+=("${cluster_crd_array[@]}")
    echo "Extended cluster_object_types with cluster-scoped CRDs:"
    printf "%s\n" "${cluster_object_types[@]}"
  else
    echo "No cluster-scoped CRDs found or unable to fetch CRDs."
  fi
}

# Helper function to write output to a file
write_output() {
  local output="$1"
  local file_name="$2"
  local cmd="$3"

  # Check if output is empty, starts with "No resources found", or "error"
  if [[ -z "$output" || "$output" == "No resources found"* || "$output" == "error"* || "$output" == "Error"* ]]; then
    echo "[$(date)] Skipping $file_name: $output" >> "$temp_dir/splunk_kubernetes_debug_info.log"
    return
  fi

  # Check if output is in YAML format
  if echo "$output" | yq eval '.' - > /dev/null 2>&1; then
    # Check if output contains empty list using yq
    if [[ $(echo "$output" | yq eval '.kind' -) == "List" ]] && [[ $(echo "$output" | yq eval '.items | length' -) -eq 0 ]]; then
      echo "[$(date)] Skipping $file_name: Empty list" >> "$temp_dir/splunk_kubernetes_debug_info.log"
      return
    fi
  fi

  # Write the output to the file
  redact_sensitive_info_local "$output" > "$file_name"
}

# Redacts sensitive information from a given input string and returns the redacted content as a string.
redact_sensitive_info_local() {
    local input="$1"

    # Redact sensitive information from the input string using awk and return the result
    echo "$input" | awk '
    # Redact certificate sections
    /BEGIN CERTIFICATE/,/END CERTIFICATE/ {
        if (/BEGIN CERTIFICATE/) print;
        else if (/END CERTIFICATE/) print;
        else print "    [CERTIFICATE REDACTED]";
        next;
    }
    # Redact sensitive data patterns like caBundle, certificates, keys
    /caBundle|ca\.crt|client\.crt|client\.key|tls\.crt|tls\.key/ {
        print "    [SENSITIVE DATA REDACTED]";
        next;
    }
    # Redact tokens
    /[Tt][Oo][Kk][Ee][Nn]/ {
        print "    [TOKEN REDACTED]";
        next;
    }
    # Redact passwords
    /[Pp][Aa][Ss][Ss][Ww][Oo][Rr][Dd]/ {
        print "    [PASSWORD REDACTED]";
        next;
    }
    # Print other content unchanged
    {print}
    '
}

# Function to collect data for a given namespace
collect_data_namespace() {
   local ns=$1

  for type in "${namespace_object_types[@]}"; do
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
  kubectl get pods -n "$ns" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep -E "$k8s_object_name_filter" | while read -r pod; do
    # Collect the top 200 and bottom 200 lines of logs
    cmd="kubectl logs \"$pod\" -n \"$ns\" | (head -n 200; echo -e \"\n...\n\"; tail -n 200)"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/namespace_${ns}_logs_pod_${pod}.log" "$cmd"
  done

  # Collect events for namespace
  cmd="kubectl get events -n \"$ns\""
  output=$(eval "$cmd")
  write_output "$output" "$temp_dir/namespace_${ns}_events.txt" "$cmd"
}

# Function to collect cluster instance data
collect_cluster_info() {
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

# Function to collect cluster scoped object data
collect_cluster_scoped_resources() {
  for type in "${cluster_object_types[@]}"; do
    echo "Collecting $type cluster-scoped resources..."

    # Fetch each object's name
    kubectl get "$type" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep -E "$k8s_object_name_filter" | while read object; do
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

# Function to collect namespace scoped object data with parallel namespace processing
collect_namespace_scoped_resources() {
  local parallelism=50
  local pids=()

  for ns in "${namespaces_array[@]}"; do
    sleep 1
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

echo "Namespaces: ${namespaces_array[*]}"
echo "Kubernetes object name filter: $k8s_object_name_filter"

# Look up deployed CRDs and add them to our object lists for object to search for
extend_object_types_lists_with_crds

# Create a temporary directory with a unique name
temp_dir=$(mktemp -d -t splunk_kubernetes_debug_info_XXXXXX)
if [[ ! -d "$temp_dir" ]]; then
  echo "Failed to create temporary directory"
  exit 1
fi
echo "[$(date)] Temp Dir: $temp_dir" >> "$temp_dir/splunk_kubernetes_debug_info.log"

# Output file for basic cluster information
output_file="$temp_dir/cluster.txt"

# Print script start time
script_start_time=$(date +"%Y-%m-%d %H:%M:%S")
echo "Script start time: $script_start_time"
echo "Script start time: $script_start_time" >> "$output_file"

# Collect cluster instance specific data
collect_cluster_info
collect_cluster_scoped_resources
collect_namespace_scoped_resources

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
