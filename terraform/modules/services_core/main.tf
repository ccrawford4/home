resource "google_service_account" "this" {
  account_id   = var.name
  display_name = var.name
}

resource "google_service_account_iam_member" "this" {
  service_account_id = google_service_account.this.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/projects/${var.project_number}/locations/global/workloadIdentityPools/${var.wif_pool_id}/subject/system:serviceaccount:${var.k8s_namespace}:${google_service_account.this.name}"
}
