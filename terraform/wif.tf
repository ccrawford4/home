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

resource "google_service_account" "home_cluster_sa" {
  account_id   = "home-cluster-sa"
  display_name = "Home Cluster Service Account"
}

resource "google_service_account_iam_member" "workload_identity_binding" {
  service_account_id = google_service_account.home_cluster_sa.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/projects/${var.project_number}/locations/global/workloadIdentityPools/${google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id}/subject/system:serviceaccount:portfolio:nginx-example"
}

resource "google_artifact_registry_repository_iam_member" "internal_reader" {
  location   = google_artifact_registry_repository.internal.location
  repository = google_artifact_registry_repository.internal.name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.home_cluster_sa.email}"
}

resource "google_service_account_iam_member" "workload_identity_binding_atlantis" {
  service_account_id = google_service_account.home_cluster_sa.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/projects/${var.project_number}/locations/global/workloadIdentityPools/${google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id}/subject/system:serviceaccount:atlantis:atlantis"
}

resource "google_storage_bucket_iam_member" "atlantis_write_tf_state" {
  bucket = "tf-state-home-prod"
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.home_cluster_sa.email}"
}
