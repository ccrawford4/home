variable "project_id" {
  description = "The GCP project ID"
  type        = string
}

variable "project_number" {
  description = "The GCP project number"
  type        = string
}

variable "k8s_issuer_uri" {
  description = "The URI of the Kubernetes OIDC issuer"
  type        = string
}

variable "region" {
  description = "The GCP region"
  type        = string
  default     = "us-central1"
}

variable "cloudflare_api_token" {
  description = "The Cloudflare API token"
  type        = string
  sensitive   = true
}

variable "cloudflare_account_id" {
  description = "The Cloudflare account ID"
  type        = string
}

variable "cloudflare_tunnel_secret" {
  description = "The Cloudflare Tunnel secret"
  type        = string
  sensitive   = true
}

variable "cloudflare_email" {
  description = "The Cloudflare account email"
  type        = string
}

variable "access_policy_admin_emails" {
  description = "List of email addresses allowed admin access via Cloudflare Zero Trust policies"
  type        = list(string)
}

variable "k8s_server_ip" {
  description = "The Kubernetes API server IP address"
  type        = string
  sensitive   = true
}

variable "cloudflare_zone_id" {
  description = "The Cloudflare zone ID for the domain"
  type        = string
}

variable "tf_state_bucket_name" {
  description = "The name of the GCS bucket used for Terraform state"
  type        = string
}

variable "example_bucket_name" {
  description = "The name of the example GCS bucket"
  type        = string
}

variable "gar_location" {
  description = "The location for the Artifact Registry repository"
  type        = string
  default     = "us-central1"
}

variable "secrets_manager_sa_id" {
  description = "The account ID for the secrets manager service account"
  type        = string
  default     = "secrets-manager-sa"
}

variable "secrets_manager_sa_email" {
  description = "The email of the secrets manager service account"
  type        = string
}
