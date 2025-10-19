module "hello-world-secrets" {
   source = "./modules/secrets_core"
   project_id = var.project_id
   project_number = var.project_number
   region = var.region

   label = "hello-world"
   k8s_namespace = "hello-world"
   k8s_service_account = "hello-world-sa"
   secrets = [
      "test-secret-1"
   ]
   google_service_account_id = "hello-world"
   workload_identity_pool_id = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
}

module "search-app-secrets" {
   source = "./modules/secrets_core"
   project_id = var.project_id
   project_number = var.project_number
   region = var.region

   label = "search-app"
   k8s_namespace = "search-app"
   k8s_service_account = "search-app"
   secrets = [
      "search-app-db-username",
      "search-app-db-password"
   ]
   google_service_account_id = "search-app-sa"
   workload_identity_pool_id = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
}
