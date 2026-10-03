locals {
  fqdn = var.subdomain == "@" ? var.domain : "${var.subdomain}.${var.domain}"
}

resource "cloudflare_dns_record" "this" {
  count   = var.dns_record ? 1 : 0
  zone_id = var.zone_id
  name    = var.subdomain
  type    = "CNAME"
  content = "${var.tunnel_id}.cfargotunnel.com"
  ttl     = 1
  proxied = true
}

resource "cloudflare_zero_trust_access_application" "this" {
  for_each                  = var.access_apps
  account_id                = var.account_id
  name                      = each.key
  type                      = "self_hosted"
  allowed_idps              = []
  auto_redirect_to_identity = false
  session_duration          = "24h"
  domain                    = "${local.fqdn}${each.value.path}"

  destinations = [{
    type = "public"
    uri  = "${local.fqdn}${each.value.path}"
  }]

  policies = [{
    id = each.value.policy_id
  }]

  app_launcher_visible       = each.value.app_launcher_visible
  enable_binding_cookie      = false
  http_only_cookie_attribute = true
  options_preflight_bypass   = false
}
