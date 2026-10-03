# One entry per Kubernetes namespace that needs cloud resources.
#
# Secrets are listed without the namespace prefix: "db-password" in the
# "search-app" entry becomes the Secret Manager secret "search-app-db-password".
# Adding a secret is a one-line change here; set its value afterwards with
#   echo -n "value" | gcloud secrets versions add <namespace>-<name> --data-file=-
locals {
  workloads = {
    search-app = {
      secrets = [
        "db-username",
        "db-password",
        "db-root-password",
        "redis-password",
        "nextauth-secret",
        "github-id",
        "github-secret",
        "google-id",
        "google-secret",
      ]
    }

    ai-agent-api = {
      secrets = [
        "chat-api-key",
        "kube-api-server",
        "openai-api-key",
        "redis-password",
      ]
    }

    portfolio = {
      secrets = ["chat-api-key"]
    }

    openid-server = {
      secrets = ["kubernetes-api-url"]
    }

    atlantis = {
      secrets = [
        # GitHub
        "github-token",
        "github-webhook-secret",
        "github-app-id",
        "github-app-key",

        # Terraform variables
        "gcp-project-id",
        "gcp-project-number",
        "k8s-issuer-uri",
        "region",
        "cloudflare-api-token",
        "cloudflare-account-id",
        "cloudflare-tunnel-secret",
        "cloudflare-email",
        "k8s-server-ip",
        "cloudflare-zone-id",
      ]
    }

    tekton-pipelines = {
      secrets = ["github-token"]
    }

    virgo = {
      secrets = ["webui-admin-password"]
    }
  }
}

module "workload" {
  for_each = local.workloads
  source   = "./modules/workload"

  namespace              = each.key
  secrets                = each.value.secrets
  workload_identity_pool = local.wif_pool

  # Legacy readers kept while workloads move from the shared ClusterSecretStore
  # (secrets-manager-sa) to their per-namespace SecretStore. Remove once every
  # ExternalSecret reports SecretSynced through the new store.
  additional_secret_accessors = [
    "principal://iam.googleapis.com/${local.wif_pool}/subject/system:serviceaccount:${each.key}:${var.secrets_manager_sa_id}",
    "serviceAccount:${google_service_account.secrets_manager.email}",
  ]
}
