# Cloudflare Tunnel so my cluster can be exposed to the internet
resource "cloudflare_zero_trust_tunnel_cloudflared" "master_tunnel" {
  account_id = var.cloudflare_account_id
  name = "home.master"
  config_src = "cloudflare"
  tunnel_secret = var.cloudflare_tunnel_secret

  # Connections is deprecated and annoying to manage, so ignore changes to it
  lifecycle {
    ignore_changes = [
      connections,
    ]
  }
}
