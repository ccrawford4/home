# Secret Management

## Overview

This project uses **GCP Secret Manager** with **Workload Identity Federation** to securely provide secrets to Kubernetes workloads. Service account key creation and manual secret distribution are no longer used.

## Architecture

1. Secrets are stored in **GCP Secret Manager** (managed via Terraform in `terraform/secrets.tf`)
2. Kubernetes workloads authenticate to GCP using **Workload Identity Federation** (configured in `terraform/wif.tf`)
3. Secrets are synced to Kubernetes using an in-cluster secrets controller (e.g., [External Secrets Operator](https://external-secrets.io/) or [Secrets Store CSI Driver](https://secrets-store-csi-driver.sigs.k8s.io/))

## Adding a New Secret

1. Add the secret name to the appropriate module in `terraform/secrets.tf`
2. Run `terraform apply` (or let Atlantis handle it)
3. Set the secret value in GCP Secret Manager:
   ```bash
   echo -n "my-secret-value" | gcloud secrets versions add SECRET_NAME --data-file=-
   ```
4. The in-cluster secrets controller will automatically sync the secret to the target namespace

## Why Not Service Account Keys?

Service account keys (`apply-gcp-secret.sh` was previously used for this):
- Are long-lived credentials that can be leaked
- Cannot be automatically rotated
- Require manual distribution to hosts
- Violate the principle of least privilege

Workload Identity Federation eliminates these risks by providing short-lived, automatically-rotated tokens tied to specific workload identities.
