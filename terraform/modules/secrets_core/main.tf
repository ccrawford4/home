resource "google_secret_manager_secret" "secret" {
  for_each = toset(var.secrets)
  secret_id = each.value
  labels = {
    label = var.label
  }
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_iam_binding" "sa_secret_binding" {
  for_each = toset(var.secrets)
  project = var.project_id
  secret_id = google_secret_manager_secret.secret[each.value].secret_id
  role = "roles/secretmanager.secretAccessor"
  members = [ var.service_account_email ]
}
