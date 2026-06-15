# Cloudflare Tunnel so the cluster can be exposed to the internet
resource "cloudflare_zero_trust_tunnel_cloudflared" "master_tunnel" {
  account_id    = var.cloudflare_account_id
  name          = "home.master"
  config_src    = "cloudflare"
  tunnel_secret = var.cloudflare_tunnel_secret

  lifecycle {
    ignore_changes = [connections]
  }
}

# Tunnel configuration with ingress rules for public hostnames
resource "cloudflare_zero_trust_tunnel_cloudflared_config" "master_tunnel_config" {
  account_id = var.cloudflare_account_id
  tunnel_id  = cloudflare_zero_trust_tunnel_cloudflared.master_tunnel.id

  config = {
    ingress = [
      {
        hostname = "calum.sh"
        service  = "http://${var.k8s_server_ip}"
      },
      {
        hostname = "argocd.calum.sh"
        service  = "http://${var.k8s_server_ip}"
      },
      {
        hostname = "search.calum.sh"
        service  = "http://${var.k8s_server_ip}"
      },
      {
        hostname = "about.calum.sh"
        service  = "http://${var.k8s_server_ip}"
      },
      {
        hostname = "openid.calum.sh"
        service  = "http://${var.k8s_server_ip}"
      },
      {
        hostname = "ollama.calum.sh"
        service  = "http://${var.k8s_server_ip}"
      },
      {
        hostname = "atlantis.calum.sh"
        service  = "http://${var.k8s_server_ip}"
      },
      {
        hostname = "k8s.calum.sh"
        service  = "http://127.0.0.1:8000"
      },
      {
        # Catch-all rule (required as the last ingress rule)
        service = "http_status:404"
      }
    ]
  }
}

resource "cloudflare_dns_record" "argocd" {
  zone_id = var.cloudflare_zone_id
  name    = "argocd"
  type    = "CNAME"
  content = "${cloudflare_zero_trust_tunnel_cloudflared.master_tunnel.id}.cfargotunnel.com"
  ttl     = 1
  proxied = true
}

resource "cloudflare_dns_record" "search" {
  zone_id = var.cloudflare_zone_id
  name    = "search"
  type    = "CNAME"
  content = "${cloudflare_zero_trust_tunnel_cloudflared.master_tunnel.id}.cfargotunnel.com"
  ttl     = 1
  proxied = true
}

resource "cloudflare_dns_record" "about" {
  zone_id = var.cloudflare_zone_id
  name    = "about"
  type    = "CNAME"
  content = "${cloudflare_zero_trust_tunnel_cloudflared.master_tunnel.id}.cfargotunnel.com"
  ttl     = 1
  proxied = true
}

resource "cloudflare_dns_record" "openid" {
  zone_id = var.cloudflare_zone_id
  name    = "openid"
  type    = "CNAME"
  content = "${cloudflare_zero_trust_tunnel_cloudflared.master_tunnel.id}.cfargotunnel.com"
  ttl     = 1
  proxied = true
}

resource "cloudflare_dns_record" "ollama" {
  zone_id = var.cloudflare_zone_id
  name    = "ollama"
  type    = "CNAME"
  content = "${cloudflare_zero_trust_tunnel_cloudflared.master_tunnel.id}.cfargotunnel.com"
  ttl     = 1
  proxied = true
}

resource "cloudflare_dns_record" "atlantis" {
  zone_id = var.cloudflare_zone_id
  name    = "atlantis"
  type    = "CNAME"
  content = "${cloudflare_zero_trust_tunnel_cloudflared.master_tunnel.id}.cfargotunnel.com"
  ttl     = 1
  proxied = true
}

resource "cloudflare_dns_record" "k8s" {
  zone_id = var.cloudflare_zone_id
  name    = "k8s"
  type    = "CNAME"
  content = "${cloudflare_zero_trust_tunnel_cloudflared.master_tunnel.id}.cfargotunnel.com"
  ttl     = 1
  proxied = true
}
