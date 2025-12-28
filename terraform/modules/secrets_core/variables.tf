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

variable "label" {
  description = "A label to organize the secrets"
  type        = string
}

variable "secrets" {
  description = "A map of secret names to their IDs in Secret Manager"
  type        = list(string)
}

variable "k8s_namespace" {
  description = "The Kubernetes namespace to deploy resources into"
  type        = string
}

variable "k8s_service_account" {
  description = "The Kubernetes service account to use for the application"
  type        = string
}

variable "google_service_account_id" {
  description = "The ID of the GCP service account to be created"
  type        = string
}

variable "workload_identity_pool_id" {
  description = "The ID of the workload identity pool to use"
  type        = string
}

variable "google_service_account_email" {
  description = "The email of the GCP service account to be used (optional)"
  type        = string
  default     = null
}
