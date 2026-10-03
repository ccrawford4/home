variable "subdomain" {
  description = "Subdomain of var.domain, or \"@\" for the apex."
  type        = string
}

variable "domain" {
  description = "Base domain, e.g. calum.sh."
  type        = string
}

variable "zone_id" {
  description = "Cloudflare zone ID of var.domain."
  type        = string
}

variable "account_id" {
  description = "Cloudflare account ID."
  type        = string
}

variable "tunnel_id" {
  description = "ID of the Cloudflare Tunnel the hostname routes through."
  type        = string
}

variable "dns_record" {
  description = "Whether to create a proxied CNAME pointing the hostname at the tunnel."
  type        = bool
  default     = true
}

variable "access_apps" {
  description = "Zero Trust Access applications protecting this hostname, keyed by application name."
  type = map(object({
    policy_id            = string
    path                 = optional(string, "")
    app_launcher_visible = optional(bool, true)
  }))
  default = {}
}
