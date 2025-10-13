resource "google_service_account" "secrets_manager_service_account" {
  account_id   = "secrets-manager-sa"
  display_name = "Secrets Manager Service Account"
  description  = "Service account for accessing secrets in Secrets Manager"
}

module "search_app_secrets" {
  source = "./modules/secrets_core"
  secrets = [
    "mysql-username",
    "mysql-password",
    "redis-host",
  ]
  label                  = "search-app"
  project_id             = var.project_id
  project_number         = var.project_number
  service_account_member = google_service_account.secrets_manager_service_account.member
}
