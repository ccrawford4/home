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
