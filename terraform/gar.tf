resource "google_artifact_registry_repository" "internal" {
  location      = var.gar_location
  repository_id = "internal"
  description   = "Internal Artifact Repository"
  format        = "DOCKER"
}
