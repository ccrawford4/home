output "secret_ids" {
  description = "Secret Manager secret IDs owned by this workload."
  value       = sort(tolist(local.secret_ids))
}

output "namespace_principal" {
  description = "IAM principal set matching every Kubernetes service account in the namespace."
  value       = local.namespace_principal
}

output "service_account" {
  description = "The workload's Google service account (name, email), or null."
  value = var.service_account == null ? null : {
    name  = google_service_account.this[0].name
    email = google_service_account.this[0].email
  }
}
