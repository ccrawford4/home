# Cloudflare Tunnel so the cluster can be exposed to the internet
resource "cloudflare_zero_trust_tunnel_cloudflared" "master_tunnel" {
  account_id = var.cloudflare_account_id
  name       = "home.master"
  config_src = "cloudflare"
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
        hostname = "calum.run"
        service  = "http://${var.k8s_server_ip}"
      },
      {
        hostname = "argocd.calum.run"
        service  = "http://${var.k8s_server_ip}"
      },
      {
        hostname = "search.calum.run"
        service  = "http://${var.k8s_server_ip}"
      },
      {
        hostname = "about.calum.run"
        service  = "http://${var.k8s_server_ip}"
      },
      {
        # Catch-all rule (required as the last ingress rule)
        service = "http_status:404"
      }
    ]
  }
}
