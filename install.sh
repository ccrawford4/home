#!/bin/bash

# This script installs the necessary dependencies and sets up the environment for the project.
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update

# Create Argo CD values file
cat << EOF > argocd-values.yaml
server:
  extraArgs:
    - --insecure
  service:
    type: ClusterIP
EOF

# Install Argo CD using Helm with custom values
helm install argocd argo/argo-cd \
  --namespace argocd \
  --create-namespace \
  --values argocd-values.yaml
