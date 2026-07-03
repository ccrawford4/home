resource "cloudflare_zero_trust_access_policy" "home_master_k8s_api_admin" {
  account_id = var.cloudflare_account_id
  name       = "Admin"
  decision   = "allow"

  include = [for email in var.access_policy_admin_emails : {
    email = {
      email = email
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

# Atlantis Zero Trust Access - protects UI but allows GitHub webhooks through
resource "cloudflare_zero_trust_access_policy" "atlantis_webhook_bypass" {
  account_id = var.cloudflare_account_id
  name       = "Atlantis Webhook Bypass"
  decision   = "bypass"

  include = [{
    everyone = {}
  }]

  exclude = []
  require = []
}

resource "cloudflare_zero_trust_access_policy" "atlantis_admin" {
  account_id = var.cloudflare_account_id
  name       = "Atlantis Admin"
  decision   = "allow"

  include = [for email in var.access_policy_admin_emails : {
    email = {
      email = email
    }
  }]

  exclude = []
  require = []
}

# Bypass access for the /events webhook endpoint
resource "cloudflare_zero_trust_access_application" "atlantis_webhooks" {
  account_id                = var.cloudflare_account_id
  name                      = "atlantis-webhooks"
  type                      = "self_hosted"
  allowed_idps              = []
  auto_redirect_to_identity = false
  session_duration          = "24h"
  domain                    = "atlantis.calum.sh"

  destinations = [{
    type = "public"
    uri  = "atlantis.calum.sh/events"
  }]

  policies = [{
    id = cloudflare_zero_trust_access_policy.atlantis_webhook_bypass.id
  }]

  app_launcher_visible       = false
  enable_binding_cookie      = false
  http_only_cookie_attribute = true
  options_preflight_bypass   = false
}

# Protect the Atlantis UI with Zero Trust
resource "cloudflare_zero_trust_access_application" "atlantis_ui" {
  account_id                = var.cloudflare_account_id
  name                      = "atlantis-ui"
  type                      = "self_hosted"
  allowed_idps              = []
  auto_redirect_to_identity = false
  session_duration          = "24h"
  domain                    = "atlantis.calum.sh"

  destinations = [{
    type = "public"
    uri  = "atlantis.calum.sh"
  }]

  policies = [{
    id = cloudflare_zero_trust_access_policy.atlantis_admin.id
  }]

  app_launcher_visible       = true
  enable_binding_cookie      = false
  http_only_cookie_attribute = true
  options_preflight_bypass   = false
}
