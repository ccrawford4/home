# One entry per Kubernetes namespace that needs cloud resources.
#
# Secrets are listed without the namespace prefix: "db-password" in the
# "search-app" entry becomes the Secret Manager secret "search-app-db-password".
# Adding a secret is a one-line change here; set its value afterwards with
#   echo -n "value" | gcloud secrets versions add <namespace>-<name> --data-file=-
#
# Optional fields:
#   image_pull_service_accounts - Kubernetes service accounts whose pods pull
#                                 from the internal registry (see gar_puller)
#   service_account             - a dedicated Google service account; see
#                                 modules/workload/variables.tf
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
      secrets                     = ["chat-api-key"]
      image_pull_service_accounts = ["nginx-example"]
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

      # Runs Terraform. home-cluster-sa predates this layout and holds project
      # roles granted outside Terraform, so it is kept rather than recreated.
      service_account = {
        account_id           = "home-cluster-sa"
        display_name         = "Home Cluster Service Account"
        k8s_service_accounts = ["atlantis"]
        bucket_roles = {
          terraform-state = { bucket = var.tf_state_bucket_name, role = "roles/storage.admin" }
        }
      }
    }

    tekton-pipelines = {
      secrets = ["github-token"]
    }

    virgo = {
      secrets                     = ["webui-admin-password"]
      image_pull_service_accounts = ["virgo"]
    }
  }
}

module "workload" {
  for_each = local.workloads
  source   = "./modules/workload"

  namespace              = each.key
  secrets                = each.value.secrets
  workload_identity_pool = local.wif_pool
  service_account        = try(each.value.service_account, null)
}
