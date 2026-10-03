# Shared cloud platform: Workload Identity Federation for the cluster,
# the private image registry and storage buckets.

# Create the workload identity pool
resource "google_iam_workload_identity_pool" "home_cluster_pool" {
  workload_identity_pool_id = "home-cluster-pool"
}

# Create the OIDC provider for our Kubernetes Cluster
resource "google_iam_workload_identity_pool_provider" "home_cluster_oidc_provider" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
  workload_identity_pool_provider_id = "home-cluster-oidc-provider"
  display_name                       = "Home Cluster OIDC Provider"
  description                        = "OIDC Provider for Home Kubernetes Cluster"
  attribute_mapping = {
    "google.subject" = "assertion.sub"
    "attribute.ns"   = "assertion['kubernetes.io']['namespace']"
    "attribute.sa"   = "assertion['kubernetes.io']['serviceaccount']['name']"
  }
  oidc {
    issuer_uri = var.k8s_issuer_uri
  }
}

resource "google_artifact_registry_repository" "internal" {
  location      = var.gar_location
  repository_id = "internal"
  description   = "Internal Artifact Repository"
  format        = "DOCKER"
}

resource "google_storage_bucket" "terraform_state_bucket" {
  name     = var.tf_state_bucket_name
  location = var.region

  lifecycle_rule {
    action {
      type = "Delete"
    }

    condition {
      age                                     = 0
      days_since_custom_time                  = 0
      days_since_noncurrent_time              = 0
      matches_prefix                          = []
      matches_storage_class                   = []
      matches_suffix                          = []
      no_age                                  = false
      num_newer_versions                      = 2
      send_age_if_zero                        = true
      send_days_since_custom_time_if_zero     = false
      send_days_since_noncurrent_time_if_zero = false
      send_num_newer_versions_if_zero         = false
      with_state                              = "ARCHIVED"
    }
  }

  lifecycle_rule {
    action {
      type = "Delete"
    }

    condition {
      age                                     = 0
      days_since_custom_time                  = 0
      days_since_noncurrent_time              = 7
      matches_prefix                          = []
      matches_storage_class                   = []
      matches_suffix                          = []
      no_age                                  = false
      num_newer_versions                      = 0
      send_age_if_zero                        = true
      send_days_since_custom_time_if_zero     = false
      send_days_since_noncurrent_time_if_zero = false
      send_num_newer_versions_if_zero         = false
      with_state                              = "ANY"
    }
  }
}

resource "google_storage_bucket" "example_bucket" {
  name     = var.example_bucket_name
  location = var.region
}

locals {
  # Full resource name of the workload identity pool, used to build
  # principal:// and principalSet:// IAM members for Kubernetes identities.
  wif_pool = "projects/${var.project_number}/locations/global/workloadIdentityPools/${google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id}"
}
