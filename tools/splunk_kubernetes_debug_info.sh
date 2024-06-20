#!/bin/bash

# Description:
# This script collects debugging information from a Kubernetes cluster.
# It retrieves networking, firewall, security policies, custom resource definitions (CRDs),
# and logs from specified pods and secrets (sanitized). The outputs are saved to files for each namespace and object type.
# This helps in diagnosing and troubleshooting cluster configurations.
# Finally, it compresses all the collected files into a ZIP archive.
#
# Sensitive Data Handling:
# The script attempts to redact sensitive information where possible, including tokens, passwords, and certificates.
# However, users should review the files for any sensitive data before sharing.
#
# Usage:
# 1. Ensure you have `kubectl`, `yq`, and `helm` installed and configured to access your Kubernetes cluster.
# 2. Save the script to a file called `splunk_kubernetes_debug_info.sh`.
# 3. Make the script executable:
#    chmod +x splunk_kubernetes_debug_info.sh
# 4. Run the script:
#    ./splunk_kubernetes_debug_info.sh [namespace1 namespace2 ...]
#    If no namespaces are specified, the script will collect information from all namespaces.
#
# Default Behavior:
# The script defaults to scraping all namespaces because networking configurations can be stored in several namespaces.
# Supply a subset of namespaces if desired, but ensure to include namespaces that can contain networking or security configurations affecting the collector.
# Control plane namespaces should be included in target namespace list.
#
# Objects Scraped:
# - NetworkPolicies
# - Services
# - Ingress resources
# - Endpoints
# - ConfigMaps
# - Roles
# - RoleBindings
# - Security contexts
# - Secrets containing "splunk", "collector", or "otel" in their names
# - OpenTelemetry Instrumentation objects
# - Pod logs (agent, cluster-receiver, certmanager, operator, gateway pods)
# - Helm values for releases containing "splunk", "otel", or "collector"
# - Custom Resource Definitions (CRDs)
# - Pod Security Policies (PSPs)
# - Security Context Constraints (SCCs)
# - Cert-manager related objects (if installed)
# - MutatingWebhookConfiguration objects

# Helper function to write output to a file
write_output() {
  local output="$1"
  local file_name="$2"
  local cmd="$3"

  # Check if output is empty, starts with "No resources found", or "error: the server"
  if [[ -z "$output" || "$output" == "No resources found"* || "$output" == "error: the server"* ]]; then
    echo "[$(date)] Skipping $file_name: $output" >> "$temp_dir/errors.log"
    return
  fi

  # Check if output is in YAML format
  if echo "$output" | yq eval '.' - > /dev/null 2>&1; then
    # Check if output contains empty list using yq
    if [[ $(echo "$output" | yq eval '.kind' -) == "List" ]] && [[ $(echo "$output" | yq eval '.items | length' -) -eq 0 ]]; then
      echo "[$(date)] Skipping $file_name: Empty list" >> "$temp_dir/errors.log"
      return
    fi
  fi

  # Redact sensitive information
  output=$(echo "$output" | awk '
  /BEGIN CERTIFICATE/,/END CERTIFICATE/ {
      if (/BEGIN CERTIFICATE/) print;
      else if (/END CERTIFICATE/) print;
      else print "    [CERTIFICATE REDACTED]";
      next;
  }
  /ca\.crt|client\.crt|client\.key/ {
      print "    [SENSITIVE DATA REDACTED]";
      next;
  }
  /[Tt][Oo][Kk][Ee][Nn]/ {
      print "    [TOKEN REDACTED]";
      next;
  }
  /[Pp][Aa][Ss][Ss][Ww][Oo][Rr][Dd]/ {
      print "    [PASSWORD REDACTED]";
      next;
  }
  {print}')

  # Write command and output to file
  echo "# Command: $cmd" > "$file_name"
  echo "$output" >> "$file_name"
}

# Create a temporary directory with a unique name
temp_dir=$(mktemp -d -t splunk_kubernetes_debug_info_XXXXXX)
if [[ ! -d "$temp_dir" ]]; then
  echo "Failed to create temporary directory"
  exit 1
fi

# Function to collect data for a given namespace
collect_data() {
  local ns=$1
  echo "Collecting data for namespace: $ns"

  # Network policies control the traffic flow between pods within the cluster.
  cmd="kubectl get networkpolicies -n $ns -o yaml"
  output=$(eval "$cmd")
  write_output "$output" "$temp_dir/networkpolicies_$ns.yaml" "$cmd"

  # Services expose applications running on a set of pods as network services.
  cmd="kubectl get svc -n $ns -o yaml"
  output=$(eval "$cmd")
  write_output "$output" "$temp_dir/services_$ns.yaml" "$cmd"

  # Ingress resources manage external access to the services in the cluster.
  cmd="kubectl get ingress -n $ns -o yaml"
  output=$(eval "$cmd")
  write_output "$output" "$temp_dir/ingress_$ns.yaml" "$cmd"

  # Endpoints define the IP addresses of the endpoints associated with a service.
  cmd="kubectl get endpoints -n $ns -o yaml"
  output=$(eval "$cmd")
  write_output "$output" "$temp_dir/endpoints_$ns.yaml" "$cmd"

  # ConfigMaps store configuration data that other resources in the cluster can use.
  cmd="kubectl get configmaps -n $ns -o yaml"
  output=$(eval "$cmd")
  write_output "$output" "$temp_dir/configmaps_$ns.yaml" "$cmd"

  # Roles define permissions within a specific namespace.
  cmd="kubectl get roles -n $ns -o yaml"
  output=$(eval "$cmd")
  write_output "$output" "$temp_dir/roles_$ns.yaml" "$cmd"

  # RoleBindings assign roles to users or service accounts within a namespace.
  cmd="kubectl get rolebindings -n $ns -o yaml"
  output=$(eval "$cmd")
  write_output "$output" "$temp_dir/rolebindings_$ns.yaml" "$cmd"

  # Security contexts define privilege and access control settings for pods.
  kubectl get pods -n $ns -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | while read pod; do
    cmd="kubectl get pod $pod -n $ns -o yaml | grep -A 10 \"securityContext\""
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/security_context_${ns}_${pod}.yaml" "$cmd"
  done

  # Describe secrets that contain "splunk", "collector", or "otel" within the name.
  kubectl get secrets -n $ns -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep -E 'splunk|collector|otel' | while read secret; do
    cmd="kubectl describe secret $secret -n $ns"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/secret_${ns}_${secret}.yaml" "$cmd"
  done

  # Collect OpenTelemetry Instrumentation objects
  cmd="kubectl get otelinst -n $ns -o yaml"
  output=$(eval "$cmd")
  write_output "$output" "$temp_dir/otelinst_${ns}.yaml" "$cmd"

  # Collect logs from specific pods
  pods=$(kubectl get pods -n $ns -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep -E 'splunk|collector|otel')

  # Collect logs from a single agent pod
  agent_pod=$(echo "$pods" | grep "agent" | head -n 1)
  if [ -n "$agent_pod" ]; then
    cmd="kubectl logs $agent_pod -n $ns"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/logs_pod_${agent_pod}.log" "$cmd"
    pods=$(echo "$pods" | grep -v "$agent_pod")
  fi

  # Collect logs from a single cluster-receiver pod
  cluster_receiver_pod=$(echo "$pods" | grep "cluster-receiver" | head -n 1)
  if [ -n "$cluster_receiver_pod" ]; then
    cmd="kubectl logs $cluster_receiver_pod -n $ns"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/logs_pod_${cluster_receiver_pod}.log" "$cmd"
    pods=$(echo "$pods" | grep -v "$cluster_receiver_pod")
  fi

  # Collect logs from all certmanager pods
  certmanager_pods=$(echo "$pods" | grep "certmanager")
  for pod in $certmanager_pods; do
    cmd="kubectl logs $pod -n $ns"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/logs_pod_${pod}.log" "$cmd"
  done
  pods=$(echo "$pods" | grep -v "certmanager")

  # Collect logs from all operator pods
  operator_pods=$(echo "$pods" | grep "operator")
  for pod in $operator_pods; do
    cmd="kubectl logs $pod -n $ns"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/logs_pod_${pod}.log" "$cmd"
  done
  pods=$(echo "$pods" | grep -v "operator")

  # Collect logs from one of the gateway pods
  gateway_pod=$(echo "$pods" | grep -vE 'agent|k8s-cluster|operator|certmanager' | head -n 1)
  if [ -n "$gateway_pod" ]; then
    cmd="kubectl logs $gateway_pod -n $ns"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/logs_pod_${gateway_pod}.log" "$cmd"
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
    pod_with_annotation=$(kubectl get pods -n $ns -o jsonpath="{range .items[?(@.metadata.annotations['$annotation'])]}{.metadata.name}{'\n'}{end}" | head -n 1)
    if [ -n "$pod_with_annotation" ]; then
      cmd="kubectl get pod $pod_with_annotation -n $ns -o yaml"
      output=$(eval "$cmd")
      write_output "$output" "$temp_dir/pod_spec_${pod_with_annotation}.yaml" "$cmd"
      cmd="kubectl logs $pod_with_annotation -n $ns"
      output=$(eval "$cmd")
      write_output "$output" "$temp_dir/logs_pod_${pod_with_annotation}.log" "$cmd"
    fi
  done
}

# Collect Helm values for releases containing splunk, otel, or collector in their names
collect_helm_values() {
  echo "Collecting Helm values for relevant releases..."
  helm list -A | grep -E 'splunk|otel|collector' | awk '{print $1, $2}' | while read release namespace; do
    cmd="helm get values $release -n $namespace"
    output=$(eval "$cmd")
    write_output "$output" "$temp_dir/helm_values_${release}_${namespace}.yaml" "$cmd"
  done
}

# Collect data from all namespaces if no namespaces are specified
if [ "$#" -eq 0 ]; then
  namespaces=$(kubectl get namespaces -o jsonpath='{.items[*].metadata.name}')
else
  namespaces="$@"
fi

# Output file for basic cluster information
output_file="$temp_dir/_cluster_info.txt"

# Print script start time
script_start_time=$(date +"%Y-%m-%d %H:%M:%S")
echo "Script start time: $script_start_time"
echo "Script start time: $script_start_time" >> "$output_file"

echo "Collecting debugging information from Kubernetes cluster..."

# Collect basic cluster information
echo "Basic Cluster Configurations:" >> "$output_file"
echo "Cluster Name: $(kubectl config view --minify -o jsonpath='{.clusters[].name}')" >> "$output_file"
echo "Kubernetes Version:" >> "$output_file"
kubectl version >> "$output_file"
echo "Number of Namespaces: $(kubectl get namespaces | wc -l)" >> "$output_file"
echo "Namespaces: $(kubectl get namespaces -o jsonpath='{.items[*].metadata.name}')" >> "$output_file"
echo "Number of Running Pods:" >> "$output_file"
kubectl get pods --all-namespaces --field-selector=status.phase=Running | wc -l >> "$output_file"
echo "Number of Running Nodes:" >> "$output_file"
kubectl get nodes | wc -l >> "$output_file"
echo "---" >> "$output_file"

# Custom Resource Definitions (CRDs) extend Kubernetes' API with new resource types.
echo "Collecting custom resource definitions..."
cmd="kubectl get crds -o yaml"
output=$(eval "$cmd")
write_output "$output" "$temp_dir/cluster_custom_resource_definitions.yaml" "$cmd"

# Pod Security Policies (PSPs) control security settings for pods.
echo "Collecting pod security policies..."
cmd="kubectl get psp -o yaml"
output=$(eval "$cmd")
write_output "$output" "$temp_dir/cluster_pod_security_policies.yaml" "$cmd"

# Security Context Constraints (SCCs) are OpenShift-specific resources that define security requirements for pods.
echo "Collecting security context constraints..."
cmd="kubectl get scc -o yaml"
output=$(eval "$cmd")
write_output "$output" "$temp_dir/cluster_security_context_constraints.yaml" "$cmd"

# Check if cert-manager is installed and collect related objects if it is
cert_manager_pods=$(kubectl get pods --all-namespaces -l app=cert-manager --no-headers)
if [ -n "$cert_manager_pods" ]; then
  echo "Cert-manager is installed. Collecting related objects..."
  cmd="kubectl get Issuers,ClusterIssuers,Certificates,CertificateRequests,Orders,Challenges --all-namespaces -o yaml; kubectl describe Issuers,ClusterIssuers,Certificates,CertificateRequests,Orders,Challenges --all-namespaces"
  output=$(eval "$cmd")
  write_output "$output" "$temp_dir/cluster_cert_manager_objects.yaml" "$cmd"
fi

# Collect Webhook objects
echo "Collecting MutatingWebhookConfiguration objects..."
cmd="kubectl get mutatingwebhookconfiguration.admissionregistration.k8s.io -o yaml; kubectl describe mutatingwebhookconfiguration.admissionregistration.k8s.io; kubectl get --raw /metrics | grep apiserver_admission_webhook_rejection_count;"
output=$(eval "$cmd")
write_output "$output" "$temp_dir/cluster_webhooks.yaml" "$cmd"

# Function to manage parallel processing
process_namespaces_in_parallel() {
  local parallelism=20
  local pids=()

  for ns in $namespaces; do
    collect_data $ns & pids+=($!)

    if [[ ${#pids[@]} -ge $parallelism ]]; then
      for pid in "${pids[@]}"; do
        wait $pid
      done
      pids=()
    fi
  done

  # Wait for any remaining background processes to complete
  for pid in "${pids[@]}"; do
    wait $pid
  done
}

# Process namespaces in parallel
process_namespaces_in_parallel

# Collect Helm values
collect_helm_values

# Create a ZIP archive of all the collected YAML files
output_zip="splunk_kubernetes_debug_info_$(date +%Y%m%d_%H%M%S).zip"
echo "Creating ZIP archive: $output_zip"

# Find and delete empty files before creating the ZIP archive
find "$temp_dir" -type f -empty -delete

zip -j -r $output_zip "$temp_dir"

# Clean up the temporary directory
rm -rf "$temp_dir"

# Print script end time and duration
script_end_time=$(date +"%Y-%m-%d %H:%M:%S")
script_start_timestamp=$(date -j -f "%Y-%m-%d %H:%M:%S" "$script_start_time" +%s)
script_end_timestamp=$(date -j -f "%Y-%m-%d %H:%M:%S" "$script_end_time" +%s)
script_duration=$((script_end_timestamp - script_start_timestamp))
script_duration_human=$(printf '%02d:%02d:%02d' $((script_duration/3600)) $((script_duration%3600/60)) $((script_duration%60)))

echo "Script end time: $script_end_time"
echo "Script duration: $script_duration_human"

echo "Script end time: $script_end_time" >> "$output_file"
echo "Script duration: $script_duration_human" >> "$output_file"

echo "Data collection complete. Output files are available in the ZIP archive: $output_zip"
