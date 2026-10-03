output "secret_ids" {
  description = "Secret Manager secret IDs owned by this workload."
  value       = sort(tolist(local.secret_ids))
}
