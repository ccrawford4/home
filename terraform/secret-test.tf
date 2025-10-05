resource "google_service_account" "secrets_manager_service_account" {
  account_id   = "secrets-manager-sa"
  display_name = "Secrets Manager Service Account"
  description = "Service account for accessing secrets in Secrets Manager"
}

resource "google_secret_manager_secret" "test_secret_two" {
  secret_id = "test-secret-2"
  labels = {
    label = "test"
  }
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_iam_binding" "k8s_sa_secret_binding_test_two" {
  project = var.project_id
  secret_id = google_secret_manager_secret.test_secret_two.secret_id
  role = "roles/secretmanager.secretAccessor"
  members = [ google_service_account.secrets_manager_service_account.member ]
}
