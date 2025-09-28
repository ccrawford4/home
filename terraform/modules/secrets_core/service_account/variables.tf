variable "account_id" {
  description = "The ID of the GCP account"
  type        = string
}

variable "project_id" {
  description = "The ID of the GCP project"
  type        = string
}


variable "k8s_namespace" {
  description = "The Kubernetes namespace"
  type        = string
}

variable "k8s_service_account" {
  description = "The Kubernetes service account name"
  type        = string
}

variable "workload_identity_pool_id" {
  description = "The ID of the workload identity pool to use"
  type        = string
}

variable "project_number" {
  description = "The GCP project number"
  type        = string
}
