variable "secrets" {
  description = "List of secret IDs to create in Secret Manager"
  type        = list(string)
}

variable "label" {
  description = "Label to apply to the secrets for organization"
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

variable "service_account_member" {
  description = "Service account member that will have access to the secrets"
  type        = string
}
