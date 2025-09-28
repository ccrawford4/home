resource "google_service_account" "service_account" {
  account_id   = var.account_id
  display_name = "${var.account_id} Service Account"
}

resource "google_service_account_iam_binding" "workload_identity_user" {
  service_account_id = google_service_account.service_account.name
  role               = "roles/iam.workloadIdentityUser"
members = [ "principal://iam.googleapis.com/projects/${var.project_number}/locations/global/workloadIdentityPools/${var.workload_identity_pool_id}/subject/ns/${var.k8s_namespace}/sa/${var.k8s_service_account}" ]
}
