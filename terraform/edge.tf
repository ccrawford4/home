# Public entrypoint: Cloudflare Tunnel, DNS and Zero Trust Access.

locals {
  domain = "calum.sh"

  # One entry per public hostname, keyed by subdomain ("@" is the apex domain).
  # Each entry gets a tunnel ingress rule and, unless dns_record = false, a
  # proxied CNAME to the tunnel. Optional fields:
  #   service     - tunnel origin (default: the cluster ingress)
  #   access_apps - Zero Trust Access applications, keyed by app name:
  #                 { policy = <key of local.access_policies>, path = "/x", app_launcher_visible = bool }
  public_hostnames = {
    "@" = { dns_record = false }

    about  = {}
    ai     = {}
    chat   = {}
    ci     = {}
    ollama = {}
    openid = {}
    search = {}

    argocd = {
      access_apps = {
        home-master-argocd = { policy = "admin" }
      }
    }

    atlantis = {
      access_apps = {
        atlantis-ui = { policy = "atlantis_admin" }
        # GitHub webhooks must reach /events without logging in.
        atlantis-webhooks = { policy = "atlantis_webhook_bypass", path = "/events", app_launcher_visible = false }
      }
    }

    k8s = {
      service = "http://127.0.0.1:8000"
      access_apps = {
        home-master-k8s-api = { policy = "admin" }
      }
    }
  }

  # Reusable Zero Trust Access policies. "allow" policies admit
  # var.access_policy_admin_emails; "bypass" policies admit everyone.
  access_policies = {
    admin                   = { name = "Admin", decision = "allow" }
    atlantis_admin          = { name = "Atlantis Admin", decision = "allow" }
    atlantis_webhook_bypass = { name = "Atlantis Webhook Bypass", decision = "bypass" }
  }
}

resource "cloudflare_zero_trust_tunnel_cloudflared" "master_tunnel" {
  account_id    = var.cloudflare_account_id
  name          = "home.master"
  config_src    = "cloudflare"
  tunnel_secret = var.cloudflare_tunnel_secret

  lifecycle {
    ignore_changes = [connections]
  }
}

resource "cloudflare_zero_trust_tunnel_cloudflared_config" "master_tunnel_config" {
  account_id = var.cloudflare_account_id
  tunnel_id  = cloudflare_zero_trust_tunnel_cloudflared.master_tunnel.id

  config = {
    ingress = concat(
      [for name, host in local.public_hostnames : {
        hostname = name == "@" ? local.domain : "${name}.${local.domain}"
        service  = try(host.service, "http://${var.k8s_server_ip}")
      }],
      # Catch-all rule (required as the last ingress rule)
      [{ service = "http_status:404" }],
    )
  }
}

resource "cloudflare_zero_trust_access_policy" "this" {
  for_each   = local.access_policies
  account_id = var.cloudflare_account_id
  name       = each.value.name
  decision   = each.value.decision

  include = each.value.decision == "bypass" ? [{ everyone = {} }] : [
    for email in var.access_policy_admin_emails : { email = { email = email } }
  ]

  exclude = []
  require = []
}

module "public_hostname" {
  for_each = local.public_hostnames
  source   = "./modules/public_hostname"

  subdomain  = each.key
  domain     = local.domain
  zone_id    = var.cloudflare_zone_id
  account_id = var.cloudflare_account_id
  tunnel_id  = cloudflare_zero_trust_tunnel_cloudflared.master_tunnel.id
  dns_record = try(each.value.dns_record, true)

  access_apps = {
    for app, cfg in try(each.value.access_apps, {}) : app => {
      policy_id            = cloudflare_zero_trust_access_policy.this[cfg.policy].id
      path                 = try(cfg.path, "")
      app_launcher_visible = try(cfg.app_launcher_visible, true)
    }
  }
}
