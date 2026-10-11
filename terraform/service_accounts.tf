# Shared Google service accounts that are not owned by a single workload.

# Used by the kubelet credential provider to pull images from the internal
# registry, and by Atlantis to run Terraform.
resource "google_service_account" "home_cluster_sa" {
  account_id   = "home-cluster-sa"
  display_name = "Home Cluster Service Account"
}

# Kubernetes service accounts ("<namespace>/<name>") allowed to impersonate home-cluster-sa.
resource "google_service_account_iam_member" "home_cluster_sa_workload_identity" {
  for_each = toset([
    "portfolio/nginx-example",
    "virgo/virgo",
    "atlantis/atlantis",
  ])

  service_account_id = google_service_account.home_cluster_sa.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${local.wif_pool}/subject/system:serviceaccount:${replace(each.key, "/", ":")}"
}

resource "google_artifact_registry_repository_iam_member" "internal_reader" {
  location   = google_artifact_registry_repository.internal.location
  repository = google_artifact_registry_repository.internal.name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.home_cluster_sa.email}"
}

resource "google_storage_bucket_iam_member" "atlantis_write_tf_state" {
  bucket = var.tf_state_bucket_name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.home_cluster_sa.email}"
}
