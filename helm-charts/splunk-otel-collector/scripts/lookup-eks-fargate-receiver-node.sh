#! /usr/bin/bash
set -ex

# If we are the first pod (cluster receiver), set the kubelet stats node filter to only follow labelled nodes.
# This node label will be set by the second pod.
if [[ "${K8S_POD_NAME}" == *-0 ]]; then
  echo "will configure kubelet stats receiver to follow node ${FIRST_CR_REPLICA_NODE_NAME}, as well as use cluster receiver."
  echo "export CR_KUBELET_STATS_NODE_FILTER='&& labels[\"splunk-otel-is-eks-fargate-cluster-receiver-node\"] == \"true\"'" >/splunk-messages/environ
  echo "export CR_K8S_OBSERVER_OBSERVE_PODS='false'" >>/splunk-messages/environ

  cat /splunk-messages/environ

  # copy config to meet container command args
  cp /conf/relay.yaml /splunk-messages/config.yaml
  exit 0
fi

# Else we are the second pod (wide kubelet stats) label our node to be monitored by the first pod and disable the k8s_cluster receiver.
# Update our config to not monitor ourselves
echo "Labelling our fargate node to denote it hosts the cluster receiver"

# download kubectl (verifying checksum)
curl -o kubectl https://amazon-eks.s3.us-west-2.amazonaws.com/1.16.15/2020-11-02/bin/linux/amd64/kubectl
ACTUAL=$(sha256sum kubectl | awk '{print $1}')
if [ "${ACTUAL}" != "e76b2f1271a5046686e03d6c68a16a34de736cfff30c92d80c9d6d87fe3cdc6c" ]; then
  echo "will not attempt to use kubectl with unexpected sha256 (${ACTUAL} != e76b2f1271a5046686e03d6c68a16a34de736cfff30c92d80c9d6d87fe3cdc6c)"
  exit 1
fi
chmod a+x kubectl
# label node
./kubectl label nodes $K8S_NODE_NAME splunk-otel-is-eks-fargate-cluster-receiver-node=true

echo "Disabling k8s_cluster receiver for this instance"
# download yq to strip k8s_cluster receiver
curl -L -o yq https://github.com/mikefarah/yq/releases/download/v4.16.2/yq_linux_amd64
ACTUAL=$(sha256sum yq | awk '{print $1}')
if [ "${ACTUAL}" != "5c911c4da418ae64af5527b7ee36e77effb85de20c2ce732ed14c7f72743084d" ]; then
  echo "will not attempt to use yq with unexpected sha256 (${ACTUAL} != 5c911c4da418ae64af5527b7ee36e77effb85de20c2ce732ed14c7f72743084d)"
  exit 1
fi
chmod a+x yq
# strip k8s_cluster and its pipeline
./yq e 'del(.service.pipelines.metrics)' /conf/relay.yaml >/splunk-messages/config.yaml
./yq e -i 'del(.receivers.k8s_cluster)' /splunk-messages/config.yaml

# set kubelet stats to not monitor ourselves (all other kubelets)
echo "EKS kubelet stats receiver node lookup not applicable for $K8S_POD_NAME. Ensuring it won't monitor itself to avoid Fargate network limitation."
echo "export CR_KUBELET_STATS_NODE_FILTER='&& not ( name contains \"${K8S_NODE_NAME}\" )'" >/splunk-messages/environ
echo "export CR_K8S_OBSERVER_OBSERVE_PODS='true'" >>/splunk-messages/environ
