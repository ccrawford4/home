variable "secrets" {
  description = "List of secret IDs to create in Secret Manager"
  type        = list(string)
}

variable "label" {
  description = "Label to apply to the secrets for organization"
  type        = string
}

variable "k8s_namespace" {
  description = "Kubernetes namespace where the service account is located"
  type        = string
}

variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "project_number" {
  description = "GCP project number"
  type        = string
}

variable "k8s_service_account_name" {
  description = "Kubernetes service account name that will access the secrets"
  type        = string
}

variable "service_account_email" {
  description = "Service account email that will have access to the secrets"
  type        = string
}
