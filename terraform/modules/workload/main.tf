locals {
  secret_ids = toset([for name in var.secrets : "${var.namespace}-${name}"])

  # Every Kubernetes service account in the namespace, via the pool's
  # attribute.ns mapping. Secrets are only readable from their own namespace.
  namespace_principal = "principalSet://iam.googleapis.com/${var.workload_identity_pool}/attribute.ns/${var.namespace}"
}

resource "google_secret_manager_secret" "this" {
  for_each  = local.secret_ids
  secret_id = each.key

  labels = {
    label = var.namespace
  }

  replication {
    auto {}
  }
}

# Authoritative: the accessor role on each secret is held by exactly these
# members, so access granted outside Terraform is removed on apply.
resource "google_secret_manager_secret_iam_binding" "accessor" {
  for_each  = google_secret_manager_secret.this
  project   = each.value.project
  secret_id = each.value.secret_id
  role      = "roles/secretmanager.secretAccessor"
  members   = concat([local.namespace_principal], var.additional_secret_accessors)
}

resource "google_service_account" "this" {
  count        = var.service_account == null ? 0 : 1
  account_id   = coalesce(var.service_account.account_id, var.namespace)
  display_name = coalesce(var.service_account.display_name, "${var.namespace} workload")
}

resource "google_service_account_iam_member" "workload_identity" {
  for_each           = toset(var.service_account == null ? [] : var.service_account.k8s_service_accounts)
  service_account_id = google_service_account.this[0].name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${var.workload_identity_pool}/subject/system:serviceaccount:${var.namespace}:${each.key}"
}

resource "google_project_iam_member" "this" {
  for_each = toset(var.service_account == null ? [] : var.service_account.project_roles)
  project  = google_service_account.this[0].project
  role     = each.key
  member   = "serviceAccount:${google_service_account.this[0].email}"
}

resource "google_storage_bucket_iam_member" "this" {
  for_each = var.service_account == null ? {} : var.service_account.bucket_roles
  bucket   = each.value.bucket
  role     = each.value.role
  member   = "serviceAccount:${google_service_account.this[0].email}"
}
