variable "namespace" {
  description = "Kubernetes namespace the workload runs in. Also used as the prefix of its Secret Manager secret IDs."
  type        = string
}

variable "secrets" {
  description = "Secret names without the namespace prefix. Each becomes the Secret Manager secret \"<namespace>-<name>\"."
  type        = list(string)
  default     = []
}

variable "workload_identity_pool" {
  description = "Full resource name of the workload identity pool (projects/<number>/locations/global/workloadIdentityPools/<id>)."
  type        = string
}

variable "additional_secret_accessors" {
  description = "IAM members granted roles/secretmanager.secretAccessor on every secret of this workload, in addition to the namespace's Kubernetes identities."
  type        = list(string)
  default     = []
}

variable "service_account" {
  description = <<-EOT
    Optional Google service account for this workload. The listed Kubernetes
    service accounts (in this namespace) may impersonate it via Workload
    Identity Federation. Use only when a workload needs Google APIs beyond its
    own secrets.
      account_id           - defaults to the namespace
      k8s_service_accounts - Kubernetes service account names allowed to impersonate it
      project_roles        - roles granted on the project
      bucket_roles         - { <logical name> = { bucket, role } } granted on GCS buckets
  EOT
  type = object({
    account_id           = optional(string)
    display_name         = optional(string)
    k8s_service_accounts = list(string)
    project_roles        = optional(list(string), [])
    bucket_roles = optional(map(object({
      bucket = string
      role   = string
    })), {})
  })
  default = null
}
