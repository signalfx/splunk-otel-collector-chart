#!/bin/bash

# Prompt user for the case they want to use
echo "Which case do you want to use?"
echo "1. Splunk Enterprise or Splunk Cloud Platform"
echo "2. Splunk Observability Cloud"
read -p "Enter 1 or 2: " case

# Initialize the clusterName variable
read -p "Enter clusterName: " clusterName

# Create the YAML file based on the user's selection
if [ "$case" = "1" ]; then
    # Prompt for Splunk Platform parameters
    read -p "Enter Splunk Platform token: " token
    # You can customize the endpoint or ask users to input it
    endpoint="http://localhost:8088/services/collector"

    # Write to the creds.values file
    cat << EOF > creds.values
clusterName: $clusterName
splunkPlatform:
  token: $token
  endpoint: $endpoint
EOF
elif [ "$case" = "2" ]; then
    # Prompt for Splunk Observability parameters
    read -p "Enter Splunk Observability realm: " realm
    read -p "Enter Splunk Observability accessToken: " accessToken

    # Write to the creds.values file
    cat << EOF > creds.values
clusterName: $clusterName
splunkObservability:
  realm: $realm
  accessToken: $accessToken
EOF
else
    echo "Invalid selection. Please run the script again."
fi

echo "Configuration written to creds.values"
