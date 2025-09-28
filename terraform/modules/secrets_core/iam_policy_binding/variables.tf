variable "project_id" {
  description = "The GCP project ID"
  type        = string
}

variable "project_number" {
  description = "The GCP project number"
  type        = string
}

variable "region" {
  description = "The GCP region"
  type        = string
  default     = "us-central1"
}

variable "secret_id" {
  description = "The ID of the secret in Secret Manager"
  type        = string
}

variable "secret_label" {
  description = "The label to filter secrets in Secret Manager"
  type        = string
}

variable "k8s_namespace" {
  description = "The Kubernetes namespace where the service account is located"
  type        = string
}

variable "k8s_service_account" {
  description = "The Kubernetes service account name"
  type        = string
}

variable "google_service_account_email" {
  description = "The email of the Google service account that you are granting secrets access to"
  type        = string
}

variable "workload_identity_pool_id" {
  description = "The ID of the workload identity pool to use"
  type        = string
}
