resource "cloudflare_zero_trust_access_policy" "home_master_k8s_api_admin" {
  account_id = var.cloudflare_account_id
  name       = "Admin"
  decision   = "allow"

  include = [{
    email = {
      email = "calumcrawford9@gmail.com"
    }
  }]

  exclude = []
  require = []
}

resource "cloudflare_zero_trust_access_application" "home_master_k8s_api" {
  account_id                = var.cloudflare_account_id
  name                      = "home-master-k8s-api"
  type                      = "self_hosted"
  allowed_idps              = []
  auto_redirect_to_identity = false
  session_duration          = "24h"
  domain                    = "k8s.calum.sh"

  destinations = [{
    type = "public"
    uri  = "k8s.calum.sh"
  }]

  policies = [{
    id = cloudflare_zero_trust_access_policy.home_master_k8s_api_admin.id
  }]

  app_launcher_visible       = true
  enable_binding_cookie      = false
  http_only_cookie_attribute = true
  options_preflight_bypass   = false
}


resource "cloudflare_zero_trust_access_application" "home_master_argocd" {
  account_id                = var.cloudflare_account_id
  name                      = "home-master-argocd"
  type                      = "self_hosted"
  allowed_idps              = []
  auto_redirect_to_identity = false
  session_duration          = "24h"
  domain                    = "argocd.calum.sh"

  destinations = [{
    type = "public"
    uri  = "argocd.calum.sh"
  }]

  policies = [{
    id = cloudflare_zero_trust_access_policy.home_master_k8s_api_admin.id
  }]

  app_launcher_visible       = true
  enable_binding_cookie      = false
  http_only_cookie_attribute = true
  options_preflight_bypass   = false
}
