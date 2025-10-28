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
      "serach-app-db-root-password",
   ]
   google_service_account_id = "secrets-manager-sa"
   workload_identity_pool_id = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
}
