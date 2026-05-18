variable "name" {
  description = "The name of the service account to create"
  type        = string
}


variable "project_number" {
  description = "The project number for GCP to deploy to"
  type        = string
}

variable "display_name" {
  description = "The display name for the service account"
  type        = string
}

variable "k8s_namespace" {
  description = "The Kubernetes namespace for the service account"
  type        = string
  default     = "default"
}

variable "wif_pool_id" {
  description = "The ID for the Workload Identity Pool"
  type        = string
}
