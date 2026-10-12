# On-demand GPU worker pool: a zonal managed instance group of Spot
# g2-standard-4 VMs (1x NVIDIA L4) that the sandbox-api control plane scales
# between 0 and 1. Each VM boots the prebuilt "gpu-node" image
# (infrastructure/gpu-image/), joins the tailnet, and joins the home K3s
# cluster as a tainted agent running Ollama.
#
# The two secrets below are created empty. Set their values once by hand:
#   echo -n "<k3s node token>"         | gcloud secrets versions add gpu-node-k3s-token --data-file=-
#   echo -n "<tailscale tskey-auth-…>" | gcloud secrets versions add gpu-node-tailscale-authkey --data-file=-
# (the node token is /var/lib/rancher/k3s/server/node-token on the K3s server).
#
# Scale by hand with:
#   gcloud compute instance-groups managed resize gpu-pool --size=1 --zone=<var.gpu_zone>

locals {
  gpu_node_tag = "gpu-node"

  gpu_node_secrets = toset(["gpu-node-k3s-token", "gpu-node-tailscale-authkey"])

  gpu_pool_labels = {
    component   = "gpu-pool"
    cost-center = "home-cluster"
    managed-by  = "terraform"
  }

  # Kubernetes identity of the control plane that resizes the pool.
  gpu_pool_scaler_ksa = { namespace = "sandbox-api", name = "sandbox-api" }
}

# --- Node identity and bootstrap secrets -------------------------------------

resource "google_service_account" "gpu_node" {
  account_id   = "gpu-node"
  display_name = "GPU pool node"
}

resource "google_project_iam_member" "gpu_node" {
  for_each = toset(["roles/logging.logWriter", "roles/monitoring.metricWriter"])
  project  = var.project_id
  role     = each.key
  member   = google_service_account.gpu_node.member
}

resource "google_secret_manager_secret" "gpu_node" {
  for_each  = local.gpu_node_secrets
  secret_id = each.key

  labels = {
    label = "gpu-node"
  }

  replication {
    auto {}
  }
}

# Authoritative, as in modules/workload: only the node service account may
# read these secrets.
resource "google_secret_manager_secret_iam_binding" "gpu_node_accessor" {
  for_each  = google_secret_manager_secret.gpu_node
  project   = each.value.project
  secret_id = each.value.secret_id
  role      = "roles/secretmanager.secretAccessor"
  members   = [google_service_account.gpu_node.member]
}

# --- Network ------------------------------------------------------------------
#
# Nodes get an ephemeral external IP rather than Cloud NAT: it is free on Spot
# VMs, needs no router/NAT resources, and lets Tailscale make direct
# (non-DERP) connections. Ingress is closed except Tailscale's WireGuard port;
# everything else (SSH, kubelet, flannel) goes over the tailnet. The deny rule
# outranks the default network's default-allow-* rules (priority 65534).

resource "google_compute_firewall" "gpu_node_allow_tailscale" {
  name        = "gpu-node-allow-tailscale"
  network     = "default"
  description = "Tailscale WireGuard direct connections to GPU pool nodes"
  direction   = "INGRESS"
  priority    = 900

  allow {
    protocol = "udp"
    ports    = ["41641"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = [local.gpu_node_tag]
}

resource "google_compute_firewall" "gpu_node_deny_ingress" {
  name        = "gpu-node-deny-ingress"
  network     = "default"
  description = "Deny all other ingress to GPU pool nodes"
  direction   = "INGRESS"
  priority    = 1000

  deny {
    protocol = "all"
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = [local.gpu_node_tag]
}

# --- Instance template and group ---------------------------------------------

resource "google_compute_instance_template" "gpu_node" {
  name_prefix  = "gpu-node-"
  description  = "Spot g2-standard-4 (1x L4) K3s GPU agent"
  machine_type = "g2-standard-4" # the L4 is bundled; no guest_accelerator block
  tags         = [local.gpu_node_tag]
  labels       = local.gpu_pool_labels

  disk {
    # Resolved to the newest image in the family when each VM is created.
    source_image = "projects/${var.project_id}/global/images/family/gpu-node"
    disk_type    = "pd-balanced"
    disk_size_gb = 100
    auto_delete  = true
    boot         = true
    labels       = local.gpu_pool_labels
  }

  network_interface {
    network = "default"
    access_config {} # ephemeral external IP, see the firewall note above
  }

  scheduling {
    provisioning_model          = "SPOT"
    preemptible                 = true
    automatic_restart           = false
    on_host_maintenance         = "TERMINATE"
    instance_termination_action = "DELETE"
  }

  service_account {
    email  = google_service_account.gpu_node.email
    scopes = ["cloud-platform"]
  }

  shielded_instance_config {
    enable_secure_boot          = false # unsigned NVIDIA kernel modules
    enable_vtpm                 = true
    enable_integrity_monitoring = true
  }

  metadata = {
    block-project-ssh-keys = "true"
    startup-script = templatefile("${path.module}/../infrastructure/gpu-image/startup.sh.tftpl", {
      k3s_server_url        = var.k3s_server_url
      project_id            = var.project_id
      k3s_token_secret      = google_secret_manager_secret.gpu_node["gpu-node-k3s-token"].secret_id
      tailscale_auth_secret = google_secret_manager_secret.gpu_node["gpu-node-tailscale-authkey"].secret_id
    })
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "google_compute_instance_group_manager" "gpu_pool" {
  name               = "gpu-pool"
  zone               = var.gpu_zone
  base_instance_name = "gpu-node"
  description        = "On-demand GPU nodes, resized 0..1 by sandbox-api"

  version {
    instance_template = google_compute_instance_template.gpu_node.self_link_unique
  }

  # The control plane owns the size.
  target_size = 0

  # Template changes apply to the next VM created, never to a running one.
  update_policy {
    type                  = "OPPORTUNISTIC"
    minimal_action        = "REPLACE"
    max_surge_fixed       = 1
    max_unavailable_fixed = 0
  }

  lifecycle {
    ignore_changes = [target_size]
  }
}

# --- Control-plane identity ---------------------------------------------------
#
# sandbox-api/sandbox-api impersonates gpu-pool-scaler via Workload Identity
# Federation to resize the group and watch its instances.

resource "google_service_account" "gpu_pool_scaler" {
  account_id   = "gpu-pool-scaler"
  display_name = "GPU pool scaler (sandbox-api)"
}

resource "google_service_account_iam_member" "gpu_pool_scaler_workload_identity" {
  service_account_id = google_service_account.gpu_pool_scaler.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${local.wif_pool}/subject/system:serviceaccount:${local.gpu_pool_scaler_ksa.namespace}:${local.gpu_pool_scaler_ksa.name}"
}

resource "google_project_iam_custom_role" "gpu_pool_scaler" {
  role_id     = "gpuPoolScaler"
  title       = "GPU pool scaler"
  description = "Resize the gpu-pool managed instance group and read its instances"
  permissions = [
    "compute.instanceGroupManagers.get",    # get size, listManagedInstances
    "compute.instanceGroupManagers.update", # resize
    "compute.instanceGroups.get",
    "compute.instances.get",
    "compute.instances.list",
    "compute.zoneOperations.get", # poll the resize operation
  ]
}

resource "google_project_iam_member" "gpu_pool_scaler" {
  project = var.project_id
  role    = google_project_iam_custom_role.gpu_pool_scaler.id
  member  = google_service_account.gpu_pool_scaler.member
}

# Lets the scaler launch VMs that run as gpu-node.
resource "google_service_account_iam_member" "gpu_pool_scaler_act_as_node" {
  service_account_id = google_service_account.gpu_node.name
  role               = "roles/iam.serviceAccountUser"
  member             = google_service_account.gpu_pool_scaler.member
}

# --- Quota ----------------------------------------------------------------------
#
# Spot L4s need regional preemptible L4 quota and the global GPU quota, both 0
# on new projects. Off by default (var.gpu_quota_requests_enabled) because the
# Terraform identity needs roles/cloudquotas.admin and the Cloud Quotas API.
# Confirm the quota IDs first with:
#   gcloud quotas info list --service=compute.googleapis.com --project=<project> \
#     --filter="quotaId~GPU" --format="value(quotaId)"

locals {
  gpu_quota_preferences = var.gpu_quota_requests_enabled ? {
    gpu-pool-preemptible-l4 = {
      quota_id   = "PREEMPTIBLE-NVIDIA-L4-GPUS-per-project-region"
      dimensions = { region = var.region }
    }
    gpu-pool-gpus-all-regions = {
      quota_id   = "GPUS-ALL-REGIONS-per-project"
      dimensions = {}
    }
  } : {}
}

resource "google_cloud_quotas_quota_preference" "gpu_pool" {
  for_each      = local.gpu_quota_preferences
  parent        = "projects/${var.project_id}"
  name          = each.key
  service       = "compute.googleapis.com"
  quota_id      = each.value.quota_id
  dimensions    = each.value.dimensions
  contact_email = var.access_policy_admin_emails[0]
  justification = "Single on-demand Spot L4 node for a home Kubernetes cluster."

  quota_config {
    preferred_value = "1" # string in the provider schema
  }
}

output "gpu_pool_instance_group_manager" {
  description = "Name of the GPU pool managed instance group."
  value       = google_compute_instance_group_manager.gpu_pool.name
}

output "gpu_pool_zone" {
  description = "Zone of the GPU pool."
  value       = google_compute_instance_group_manager.gpu_pool.zone
}

output "gpu_pool_instance_template" {
  description = "Current GPU node instance template."
  value       = google_compute_instance_template.gpu_node.name
}

output "gpu_pool_scaler_service_account" {
  description = "Google service account sandbox-api impersonates to resize the GPU pool."
  value       = google_service_account.gpu_pool_scaler.email
}
