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
