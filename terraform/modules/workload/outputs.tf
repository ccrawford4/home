output "secret_ids" {
  description = "Secret Manager secret IDs owned by this workload."
  value       = sort(tolist(local.secret_ids))
}

output "namespace_principal" {
  description = "IAM principal set matching every Kubernetes service account in the namespace."
  value       = local.namespace_principal
}
