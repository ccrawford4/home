locals {
  secret_ids = toset([for name in var.secrets : "${var.namespace}-${name}"])
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

# Authoritative: the accessor role on each secret is held by exactly these members.
resource "google_secret_manager_secret_iam_binding" "accessor" {
  for_each  = google_secret_manager_secret.this
  project   = each.value.project
  secret_id = each.value.secret_id
  role      = "roles/secretmanager.secretAccessor"
  members   = var.secret_accessors
}
