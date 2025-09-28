# Create the workload identity pool
resource "google_iam_workload_identity_pool" "home_cluster_pool" {
  workload_identity_pool_id = "home-cluster-pool"
}

# Create the OIDC provider for our Kubernetes Cluster
resource "google_iam_workload_identity_pool_provider" "home_cluster_oidc_provider" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.home_cluster_pool.workload_identity_pool_id
  workload_identity_pool_provider_id = "home-cluster-oidc-provider"
  display_name                       = "Home Cluster OIDC Provider"
  description                        = "OIDC Provider for Home Kubernetes Cluster"
  attribute_mapping                  = {
    "google.subject"                 = "assertion.sub"
    "attribute.ns"                   = "assertion['kubernetes.io/serviceaccount/namespace']"
    "attribute.sa"                   = "assertion['kubernetes.io/serviceaccount/service-account.name']"
  }
  oidc {
    issuer_uri        = var.k8s_issuer_uri
    jwks_json         = file("cluster-jwks.json")
  }
}
