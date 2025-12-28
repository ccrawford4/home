locals {
   secrets = toset(var.secrets)
}

resource "google_service_account" "service_account" {
  count = var.google_service_account_email == null ? 1 : 0
  account_id   = var.google_service_account_id
  display_name = "${var.google_service_account_id} Service Account"
}

module "secrets_iam_binding" {
  for_each = local.secrets
  source = "./iam_policy_binding"

  project_id            = var.project_id
  project_number        = var.project_number
  region                = var.region
  secret_id             = each.key
  secret_label          = var.label
  k8s_namespace         = var.k8s_namespace
  k8s_service_account   = var.k8s_service_account
  google_service_account_email = var.google_service_account_email == null ? google_service_account.service_account[0].email : var.google_service_account_email
  workload_identity_pool_id = var.workload_identity_pool_id
}
