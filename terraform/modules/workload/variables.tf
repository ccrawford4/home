variable "namespace" {
  description = "Kubernetes namespace the workload runs in. Also used as the prefix of its Secret Manager secret IDs."
  type        = string
}

variable "secrets" {
  description = "Secret names without the namespace prefix. Each becomes the Secret Manager secret \"<namespace>-<name>\"."
  type        = list(string)
  default     = []
}

variable "secret_accessors" {
  description = "IAM members granted roles/secretmanager.secretAccessor on every secret of this workload."
  type        = list(string)
}
