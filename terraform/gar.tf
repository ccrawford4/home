resource "google_artifact_registry_repository" "internal" {
  location      = "us-central1"
  repository_id = "internal"
  description   = "Internal Artifact Repository"
  format        = "DOCKER"
}
