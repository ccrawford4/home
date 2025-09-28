locals {
   secrets = toset(var.secrets)
}

module "google_service_account" {
  source = "./service_account"
  account_id = var.google_service_account_id
  project_id = var.project_id
  project_number = var.project_number
  k8s_namespace = var.k8s_namespace
  k8s_service_account = var.k8s_service_account
  workload_identity_pool_id = var.workload_identity_pool_id
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
  google_service_account_email = module.google_service_account.email
  workload_identity_pool_id = var.workload_identity_pool_id
}
