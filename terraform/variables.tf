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

variable "gpu_zone" {
  description = "Zone for the on-demand GPU node pool (must offer NVIDIA L4 / g2-standard-4)"
  type        = string
  default     = "us-central1-a"
}

variable "k3s_server_url" {
  description = "K3s API server URL that GPU nodes join over Tailscale, e.g. https://<pi-tailscale-name-or-ip>:6443. The host must be in the server's tls-san."
  type        = string
  default     = "https://home-k3s:6443"
}

variable "gpu_quota_requests_enabled" {
  description = "Create Cloud Quotas preferences for the GPU pool (1 preemptible L4 in var.region, 1 GPU globally). Needs the Cloud Quotas API enabled and cloudquotas.admin on the Terraform identity."
  type        = bool
  default     = false
}
