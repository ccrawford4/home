# Image for the on-demand GPU pool (terraform/gpu_pool.tf). Builds on a Spot
# g2-standard-4 so the NVIDIA driver is loaded and verified with nvidia-smi
# and the Ollama models are pulled with the GPU present.
#
#   packer init .
#   packer build -var project_id=<project> -var k3s_version=<server version> .

packer {
  required_plugins {
    googlecompute = {
      source  = "github.com/hashicorp/googlecompute"
      version = "~> 1.1"
    }
  }
}

variable "project_id" {
  type        = string
  description = "GCP project the image is built and stored in."
}

variable "zone" {
  type        = string
  description = "Zone for the build VM; must offer g2-standard-4 (L4)."
  default     = "us-central1-a"
}

variable "k3s_version" {
  type        = string
  description = "K3s release to install, e.g. v1.33.4+k3s1. Must not be newer than the K3s server (check with `k3s --version` on the Pi)."
}

variable "nvidia_driver_branch" {
  type        = string
  description = "NVIDIA server driver branch installed with ubuntu-drivers."
  default     = "570"
}

variable "ollama_version" {
  type        = string
  description = "Ollama release, e.g. 0.12.6. Empty installs the latest."
  default     = ""
}

variable "ollama_models" {
  type        = list(string)
  description = "Models pre-pulled into /var/lib/ollama/models so cold starts skip the download."
  default     = ["qwen2.5-coder:7b"]
}

variable "gvisor_release" {
  type        = string
  description = "gVisor release (e.g. 20250929) or \"latest\"."
  default     = "latest"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "googlecompute" "gpu_node" {
  project_id              = var.project_id
  zone                    = var.zone
  source_image_family     = "ubuntu-2204-lts"
  source_image_project_id = ["ubuntu-os-cloud"]
  machine_type            = "g2-standard-4"
  on_host_maintenance     = "TERMINATE"
  # Spot, so the build uses the same preemptible L4 quota as the pool.
  preemptible  = true
  disk_size    = 50
  disk_type    = "pd-balanced"
  ssh_username = "packer"

  image_name        = "gpu-node-${local.timestamp}"
  image_family      = "gpu-node"
  image_description = "K3s GPU agent: NVIDIA ${var.nvidia_driver_branch}-server, nvidia-container-toolkit, Tailscale, K3s ${var.k3s_version}, gVisor, Ollama"
  image_labels = {
    component   = "gpu-pool"
    cost-center = "home-cluster"
  }
}

build {
  sources = ["source.googlecompute.gpu_node"]

  provisioner "file" {
    source      = "${path.root}/provision.sh"
    destination = "/var/tmp/provision.sh" # /tmp is wiped by the reboot
  }

  # Stage 1: NVIDIA driver, then reboot so the kernel module loads.
  provisioner "shell" {
    inline = ["sudo NVIDIA_DRIVER_BRANCH='${var.nvidia_driver_branch}' bash /var/tmp/provision.sh driver"]
  }

  provisioner "shell" {
    inline            = ["sudo systemctl reboot"]
    expect_disconnect = true
  }

  # Stage 2: everything else, starting with an nvidia-smi check.
  provisioner "shell" {
    pause_before = "30s"
    inline = [
      "sudo K3S_VERSION='${var.k3s_version}' OLLAMA_VERSION='${var.ollama_version}' GVISOR_RELEASE='${var.gvisor_release}' OLLAMA_MODELS_TO_PULL='${join(" ", var.ollama_models)}' bash /var/tmp/provision.sh userland",
    ]
  }
}
