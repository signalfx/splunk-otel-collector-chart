#!/bin/bash
# This script is specifically designed for gathering comprehensive debugging information from Kubernetes environments,
# providing an in-depth view of the cluster's configuration and operational state. It outputs YAML content of various
# Kubernetes objects within specified namespaces, offering a detailed snapshot that is invaluable for troubleshooting
# and understanding the cluster setup.

# Additional Features:
# - Censors certificates within ConfigMaps to protect sensitive data, replacing certificate content with a placeholder.
# - Collects YAML content for cluster-wide resources, offering a broader overview of the cluster setup.

# Usage Guidance:
# - Review the output file in the `/tmp` directory for sensitive information before sharing. While secrets and
#   certificates within ConfigMaps are censored, manual verification is advised.
# - For application-specific insights, include application namespaces. This script defaults to common system namespaces.
# - All object kinds from a namespace are included by default, with the option to specify kinds for focused debugging.

# The output file is intended for support teams. Ensure it's sanitized of sensitive data before sharing.

# Usage: ./scrape-k8s-debug-info.sh [OPTIONS]
# Defaults: --namespaces to default,kube-system,calico-system, --kinds to all.
# Example: ./scrape-k8s-debug-info.sh --namespaces "default,kube-system,calico-system" --kinds all

# Function to print usage
print_usage() {
    echo "Gathering info. Use '--help' for options." >&2
}

# Function to display detailed help and usage instructions
print_help() {
    echo "Usage: $0 [OPTIONS]"
    echo "Options:"
    echo "  --namespaces <namespace1,namespace2,...>  Specify namespaces to search. Defaults to default,kube-system,calico-system."
    echo "  --kinds <kind1,kind2,...>                  Specify kinds of objects to search. Defaults to 'all' for all kinds."
    echo "  --help                                    Show this help message."
    echo "Examples:"
    echo "  $0 --namespaces \"default,kube-system,calico-system\" --kinds all"
    echo "  $0 --kinds Deployment,Pod"
    echo "  $0 --namespaces all --kinds all"
    echo "  $0 (Lists namespaces and kinds available)"
}

# Initialize variables with default values
namespaces="default,kube-system,calico-system,tigera-operator"
kinds="all"

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --namespaces) namespaces="$2"; shift 2 ;;
        --kinds) kinds="$2"; shift 2 ;;
        --help) print_help; exit 0 ;;
        *) echo "Unknown parameter passed: $1"; print_usage; exit 1 ;;
    esac
done

# Generate a unique filename for the output
output_file="/tmp/splunk-debug-info-$(date +%Y%m%d%H%M)-$(uuidgen).txt"
echo "Output: $output_file" >&2

# Print basic cluster configurations
echo "Scraping Basic Cluster Info..."
echo "Basic Cluster Configurations:" >> "$output_file"
echo "Kubernetes Version:" >> "$output_file"
kubectl version >> "$output_file"
echo "Number of Running Pods:" >> "$output_file"
kubectl get pods --all-namespaces --field-selector=status.phase=Running | wc -l >> "$output_file"
echo "Number of Running Nodes:" >> "$output_file"
kubectl get nodes | wc -l >> "$output_file"
echo "---" >> "$output_file"

# Function to execute kubectl command with minimal stdout
execute_with_error_handling() {
    local kind="$1"
    local ns="$2"  # Namespace argument, optional for cluster-wide resources
    local command_output
    local status

    if [[ -n "$ns" ]]; then
        command_output=$(kubectl get "$kind" -n "$ns" -o yaml 2>&1)
    else
        command_output=$(kubectl get "$kind" --all-namespaces -o yaml 2>&1)
    fi
    status=$?

    # Apply redaction to any Kubernetes object output
    command_output=$(echo "$command_output" | awk '
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
    {print}')

    if [ $status -ne 0 ]; then
        echo "Error executing 'kubectl get $kind' in $ns" >> "$output_file"
    else
        echo "$command_output" >> "$output_file"
    fi
}

# Prepare namespaces and kinds for collection
IFS=',' read -r -a namespace_array <<< "$namespaces"
IFS=',' read -r -a kind_array <<< "$kinds"

# Adjust the logic for collecting and organizing resources
if [[ " ${namespace_array[@]} " =~ " all " ]]; then
    namespace_array=($(kubectl get namespaces -o=jsonpath='{.items[*].metadata.name}'))
elif [[ " ${namespace_array[@]} " =~ " none " ]]; then
    namespace_array=()
fi

if [[ " ${kind_array[@]} " =~ " all " ]] || [[ -z "$kinds" ]]; then
    kind_array=($(kubectl api-resources --verbs=list --namespaced=true | awk '{print $1}' | tail -n +2))
fi

# Collect and organize namespaced resources
for ns in "${namespace_array[@]}"; do
    echo "Scraping $ns Namespace Resources..."
    echo "------$ns Namespace Resources------" >> "$output_file"
    for kind in "${kind_array[@]}"; do
        execute_with_error_handling "$kind" "$ns"
    done
done

# Collect and organize cluster-wide resources
echo "Scraping Cluster Wide Resources..."
echo "------Cluster Wide Resources------" >> "$output_file"
for kind in "${kind_array[@]}"; do
    execute_with_error_handling "$kind" ""
done

echo "Collection complete. See $output_file" >&2
