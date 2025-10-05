# Create a Secret Manager secret
resource "google_secret_manager_secret" "secret" {
  secret_id = var.secret_id

  labels = {
    label = var.secret_label
  }

  replication {
     auto {}
  }
}

# Grant the K8s service account access to the secret
resource "google_secret_manager_secret_iam_binding" "k8s_sa_secret_binding" {
  project = var.project_id
  secret_id = google_secret_manager_secret.secret.secret_id
  role = "roles/secretmanager.secretAccessor"
  members = [ "principal://iam.googleapis.com/projects/${var.project_number}/locations/global/workloadIdentityPools/${var.workload_identity_pool_id}/subject/ns/${var.k8s_namespace}/sa/${var.k8s_service_account}", "serviceAccount:${var.google_service_account_email}" ]
}

# # Grant the GCP service account access to the secret
# resource "google_secret_manager_secret_iam_binding" "google_sa_secret_binding" {
#   project = var.project_id
#   secret_id = google_secret_manager_secret.secret.secret_id
#   role = "roles/secretmanager.secretAccessor"
#   members = [ "serviceAccount:${var.google_service_account_email}" ]
# }
