# Google service accounts that are not owned by a single workload.

locals {
  # "<namespace>/<service account>" for every pod identity that pulls images
  # from the internal registry.
  image_pull_service_accounts = toset(flatten([
    for namespace, workload in local.workloads : [
      for ksa in try(workload.image_pull_service_accounts, []) : "${namespace}/${ksa}"
    ]
  ]))
}

# Impersonated by the kubelet credential provider
# (infrastructure/k3s/credential-provider-config.yaml) with the pulling pod's
# service account token. Can only read images.
resource "google_service_account" "gar_puller" {
  account_id   = "gar-puller"
  display_name = "Internal registry image puller"
}

resource "google_service_account_iam_member" "gar_puller_workload_identity" {
  for_each           = local.image_pull_service_accounts
  service_account_id = google_service_account.gar_puller.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${local.wif_pool}/subject/system:serviceaccount:${replace(each.key, "/", ":")}"
}

resource "google_artifact_registry_repository_iam_member" "gar_puller_reader" {
  location   = google_artifact_registry_repository.internal.location
  repository = google_artifact_registry_repository.internal.name
  role       = "roles/artifactregistry.reader"
  member     = google_service_account.gar_puller.member
}
