# Secret Management

## Overview

This project uses **GCP Secret Manager** with **Workload Identity Federation** to securely provide secrets to Kubernetes workloads. Service account key creation and manual secret distribution are no longer used.

## Architecture

1. Secrets are stored in **GCP Secret Manager** (managed via Terraform in `terraform/workloads.tf`)
2. Kubernetes workloads authenticate to GCP using **Workload Identity Federation** (configured in `terraform/platform.tf`)
3. Secrets are synced to Kubernetes by [External Secrets Operator](https://external-secrets.io/) through a `SecretStore` named `gcp-secret-manager` in each app namespace

## Access Boundary

Each namespace can only read its own secrets:

- `terraform/modules/workload` grants `roles/secretmanager.secretAccessor` on
  `<namespace>-*` secrets to
  `principalSet://iam.googleapis.com/<pool>/attribute.ns/<namespace>`, i.e. any
  Kubernetes service account in that namespace.
- The namespace's `SecretStore` (created by the `application-template` chart,
  or by the `atlantis` / `tekton-pipelines` charts) authenticates with
  `auth.workloadIdentityFederation`: ESO requests a token for the namespace's
  Kubernetes service account and exchanges it with Google STS. No Google
  service account or key is involved.

A compromised workload in `virgo` therefore cannot read `atlantis-*` secrets.

## Adding a New Secret

1. Add the secret name (without the namespace prefix) to the workload's `secrets` list in `terraform/workloads.tf`
2. Run `terraform apply` (or let Atlantis handle it)
3. Set the secret value in GCP Secret Manager:
   ```bash
   echo -n "my-secret-value" | gcloud secrets versions add SECRET_NAME --data-file=-
   ```
4. Reference it from the app chart's `secrets` list with `remoteRefKey: <namespace>-<name>`; ESO syncs it into the namespace

## Adding a New App

1. Add an entry to `local.workloads` in `terraform/workloads.tf`:
   ```hcl
   my-app = {
     secrets = ["api-key"]
   }
   ```
2. Use the `application-template` chart with `namespace: my-app` and a service
   account. The chart creates the `gcp-secret-manager` SecretStore for you.

## Why Not Service Account Keys?

Service account keys (previously a `secrets-manager-sa` key in the `gcp-sa-secret` Secret, read by a single cluster-wide `ClusterSecretStore`):
- Are long-lived credentials that can be leaked
- Cannot be automatically rotated
- Require manual distribution to hosts
- Violate the principle of least privilege

Workload Identity Federation eliminates these risks by providing short-lived, automatically-rotated tokens tied to specific workload identities.
