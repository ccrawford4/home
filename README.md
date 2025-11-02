# home

A production-ready Kubernetes home cluster running on Raspberry Pis, hosting multiple web applications and services. This repository contains all the Infrastructure as Code (IaC) configurations, Helm charts, and deployment manifests needed to run and manage the cluster using GitOps principles with ArgoCD.

## Overview

This home cluster is built on K3s (lightweight Kubernetes) and leverages modern cloud-native practices including:
- **GitOps** deployment model with ArgoCD ApplicationSets
- **Automated secret management** using External Secrets Operator with GCP Secret Manager
- **Ingress routing** with Cloudflare tunneling for secure external access
- **Containerized applications** deployed via Helm charts
- **Infrastructure as Code** for reproducible and version-controlled infrastructure

## Hosted Services

The cluster currently hosts the following production services:

### 🔍 [search.calum.run](https://search.calum.run) - Search Engine
A full-featured search engine with user authentication and search history tracking.
- **Repository:** [ccrawford4/search](https://github.com/ccrawford4/search)
- **Tech Stack:** Next.js frontend, Go backend, MySQL database, Redis cache
- **Features:** OAuth authentication (GitHub, Google), search history, persistent storage

### 👤 [about.calum.run](https://about.calum.run) - Portfolio Website
Personal portfolio and about page.
- **Repository:** [ccrawford4/portfolio-next](https://github.com/ccrawford4/portfolio-next)
- **Tech Stack:** Next.js

### 🚀 [argocd.calum.run](https://argocd.calum.run) - ArgoCD
GitOps continuous delivery platform managing all applications in the cluster.
- **Purpose:** Automated application deployment and lifecycle management
- **Features:** Self-healing deployments, automated sync from Git, application health monitoring

## Prerequisites
- A set of Raspberry Pis with K3s installed and running
- SSH access to the master node
- Helm installed on the master node

## Installation

1. SSH into the master node
```bash
# Or whatever your ssh command is
ssh pi@<master-node-ip>
```

2. Clone this repository

```bash
git clone https://github.com/ccrawford4/home.git
```

3. Navigate to the `home` directory and run the install script

```bash
cd home
./install.sh
```

4. Then add the applicationset

```bash
kubectl apply -f argocd-applicationset.yaml
```

5. Add a tunnel (I'm using cloudflare) so you can access it from outside your network


## Generating Secrets

### Create the kubernetes secret to authenticate with Google Cloud

#### Using the automated script (recommended)

1. Run the script with your GCP project ID and SSH hostname:
```bash
./apply-gcp-secret.sh <project-id> <hostname>
```
Example:
```bash
./apply-gcp-secret.sh home-473419 pi@192.168.1.100
```

2. SSH into your host to verify the secret was created, and then apply it to the cluster:
```bash
ssh <hostname>
kubectl apply -f gcp-sa-secret.yaml
```

**Note:** The secret is created in the `default` namespace, which allows the ClusterSecretStore to reference it from any namespace. The ClusterSecretStore is configured to look for this secret in the `default` namespace.

This will:
1. Create the service account key
2. Generate the `gcp-sa-secret.yaml` with the credentials
3. Copy it to the remote host

#### Manual steps

If you prefer to do it manually:

1. Create the service account key
```bash
gcloud iam service-accounts keys create credentials \
    --iam-account=secrets-manager-sa@<project-id>.iam.gserviceaccount.com
```

2. Update `gcp-sa-secret.yaml` with the contents of the credentials file (ensure the namespace is set to `default`)

3. Copy the secret to your cluster and apply it to the default namespace
```bash
scp gcp-sa-secret.yaml <hostname>:~/gcp-sa-secret.yaml
ssh <hostname> "kubectl apply -f gcp-sa-secret.yaml"
```

**Important:** The ClusterSecretStore is configured to reference this secret from the `default` namespace. This allows ExternalSecrets in any namespace to use the ClusterSecretStore to authenticate with GCP Secret Manager.

### TODO: Add terraform

### TODO: Update the secret value in google and set a new version
