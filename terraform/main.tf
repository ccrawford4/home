module "search-app-secrets" {
   source = "./modules/secrets_core"
   project_id = var.project_id
   project_number = var.project_number
   region = var.region

   label = "search-app"
   k8s_namespace = "search-app"
   k8s_service_account = "secrets-manager-sa"
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
   google_service_account_id = "secrets-manager-sa"
   workload_identity_pool_id = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
}

module "ai-agent-api-secrets" {
  source = "./modules/secrets_core"
  project_id = var.project_id
  project_number = var.project_number
  region = var.region

  label = "ai-agent-api"
  k8s_namespace = "ai-agent-api"
  k8s_service_account = "secrets-manager-sa"
  secrets = [
    "ai-agent-api-chat-api-key",
    "ai-agent-api-kube-api-server",
    "ai-agent-openai-api-key"
  ]

   google_service_account_id = "secrets-manager-sa"
   google_service_account_email = "secrets-manager-sa@${var.project_id}.iam.gserviceaccount.com"
   workload_identity_pool_id = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
}

module "portfolio-secrets" {
  source = "./modules/secrets_core"
  project_id = var.project_id
  project_number = var.project_number
  region = var.region

  label = "portfolio"
  k8s_namespace = "portfolio"
  k8s_service_account = "secrets-manager-sa"
  secrets = [
    "portfolio-chat-api-key",
  ]

   google_service_account_id = "secrets-manager-sa"
   google_service_account_email = "secrets-manager-sa@${var.project_id}.iam.gserviceaccount.com"
   workload_identity_pool_id = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
}
