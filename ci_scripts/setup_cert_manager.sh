#!/usr/bin/env bash
if kubectl get pods -l app=cert-manager --all-namespaces | grep "cert-manager"; then
  kubectl get pods -l app=cert-manager --all-namespaces
  echo "cert-manager is already deployed"
  exit 0
fi
echo "No cert-manager detected, deploying it now"
kubectl  apply -f https://github.com/jetstack/cert-manager/releases/download/v1.10.0/cert-manager.yaml
kubectl wait deployment -n cert-manager cert-manager  --for condition=Available=True --timeout=120s
