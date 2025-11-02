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
- **Repositories:** 
  - Backend: [ccrawford4/search](https://github.com/ccrawford4/search)
  - Frontend: [ccrawford4/search-app](https://github.com/ccrawford4/search-app)
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

### Setting Up Secrets for a New Application

After authenticating with Google Cloud (see above), follow these steps to set up secrets for a new application:

#### 1. Define Terraform Configuration

Add a new module in `terraform/main.tf` for your application's secrets:

```hcl
module "your-app-secrets" {
   source = "./modules/secrets_core"
   project_id = var.project_id
   project_number = var.project_number
   region = var.region

   label = "your-app"
   k8s_namespace = "your-app"
   k8s_service_account = "secrets-manager-sa"
   secrets = [
      "your-app-secret-1",
      "your-app-secret-2",
      # Add all secrets your application needs
   ]
   google_service_account_id = "secrets-manager-sa"
   workload_identity_pool_id = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
}
```

#### 2. Apply Terraform Configuration

Run Terraform to create the secrets in Google Secret Manager and set up IAM permissions:

```bash
cd terraform
terraform init
terraform plan
terraform apply
```

This will:
- Create all secret placeholders in Google Secret Manager
- Set up IAM bindings to allow your Kubernetes service account to access these secrets
- Configure Workload Identity for secure authentication

#### 3. Update Secret Values in Google Secret Manager

After Terraform creates the secrets, you need to add the actual secret values:

**Using gcloud CLI:**

```bash
# Add a new secret version with the actual value
echo -n "your-secret-value" | gcloud secrets versions add your-app-secret-1 \
    --data-file=- \
    --project=<project-id>
```

**Using Google Cloud Console:**

1. Navigate to [Secret Manager](https://console.cloud.google.com/security/secret-manager) in Google Cloud Console
2. Find your secret (e.g., `your-app-secret-1`)
3. Click "New Version"
4. Enter your secret value
5. Click "Add New Version"

**Important Notes:**
- Secrets are created with no versions by default - you must add a version with the actual value
- Always use `echo -n` to avoid adding a newline character to your secret
- For sensitive values, consider using a file: `gcloud secrets versions add secret-name --data-file=/path/to/file --project=<project-id>`

#### 4. Create Helm Configuration

Create or update your Helm values file in `helm/your-app/values.yaml` to reference the secrets:

```yaml
application-template:
  name: your-app
  namespace: your-app
  clusterSecretStoreName: cluster-secret-store
  
  secrets:
    - name: your-app-secrets
      targetName: your-app-secrets
      data:
        - secretKey: secret-1
          remoteRefKey: your-app-secret-1
        - secretKey: secret-2
          remoteRefKey: your-app-secret-2
  
  deployments:
    - name: your-app
      container:
        env:
          - name: SECRET_1
            valueFrom:
              secretKeyRef:
                name: your-app-secrets
                key: secret-1
```

The External Secrets Operator will automatically sync the secrets from Google Secret Manager to Kubernetes secrets in your namespace.
