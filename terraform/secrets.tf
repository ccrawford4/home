module "search-app-secrets" {
  source         = "./modules/secrets_core"
  project_id     = var.project_id
  project_number = var.project_number
  region         = var.region

  label               = "search-app"
  k8s_namespace       = "search-app"
  k8s_service_account = var.secrets_manager_sa_id
  secrets = [
    "search-app-db-username",
    "search-app-db-password",
    "search-app-db-root-password",
    "search-app-redis-password",
    "search-app-nextauth-secret",
    "search-app-github-id",
    "search-app-github-secret",
    "search-app-google-id",
    "search-app-google-secret",
  ]
  google_service_account_id = var.secrets_manager_sa_id
  workload_identity_pool_id = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
}

module "ai-agent-api-secrets" {
  source         = "./modules/secrets_core"
  project_id     = var.project_id
  project_number = var.project_number
  region         = var.region

  label               = "ai-agent-api"
  k8s_namespace       = "ai-agent-api"
  k8s_service_account = var.secrets_manager_sa_id
  secrets = [
    "ai-agent-api-chat-api-key",
    "ai-agent-api-kube-api-server",
    "ai-agent-api-openai-api-key",
    "ai-agent-api-redis-password",
  ]

  google_service_account_id    = var.secrets_manager_sa_id
  google_service_account_email = var.secrets_manager_sa_email
  workload_identity_pool_id    = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
}

module "portfolio-secrets" {
  source         = "./modules/secrets_core"
  project_id     = var.project_id
  project_number = var.project_number
  region         = var.region

  label               = "portfolio"
  k8s_namespace       = "portfolio"
  k8s_service_account = var.secrets_manager_sa_id
  secrets = [
    "portfolio-chat-api-key",
  ]

  google_service_account_id    = var.secrets_manager_sa_id
  google_service_account_email = var.secrets_manager_sa_email
  workload_identity_pool_id    = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
}

module "openid-server-secrets" {
  source         = "./modules/secrets_core"
  project_id     = var.project_id
  project_number = var.project_number
  region         = var.region

  label               = "openid-server"
  k8s_namespace       = "openid-server"
  k8s_service_account = var.secrets_manager_sa_id
  secrets = [
    "openid-server-kubernetes-api-url",
  ]

  google_service_account_id    = var.secrets_manager_sa_id
  google_service_account_email = var.secrets_manager_sa_email
  workload_identity_pool_id    = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
}


module "atlantis-secrets" {
  source         = "./modules/secrets_core"
  project_id     = var.project_id
  project_number = var.project_number
  region         = var.region

  label               = "atlantis"
  k8s_namespace       = "atlantis"
  k8s_service_account = var.secrets_manager_sa_id
  secrets = [
    # GitHub webhook secrets
    "atlantis-github-token",
    "atlantis-github-webhook-secret",
    "atlantis-github-app-id",
    "atlantis-github-app-key",

    # Tf vars
    "atlantis-gcp-project-id",
    "atlantis-gcp-project-number",
    "atlantis-k8s-issuer-uri",
    "atlantis-region",
    "atlantis-cloudflare-api-token",
    "atlantis-cloudflare-account-id",
    "atlantis-cloudflare-tunnel-secret",
    "atlantis-cloudflare-email",
    "atlantis-k8s-server-ip",
    "atlantis-cloudflare-zone-id",
  ]

  google_service_account_id    = var.secrets_manager_sa_id
  google_service_account_email = var.secrets_manager_sa_email
  workload_identity_pool_id    = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
}
