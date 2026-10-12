#!/bin/bash
# Provisions the gpu-node image (see gpu-node.pkr.hcl). Runs as root in two
# stages because the NVIDIA kernel module needs a reboot before it loads:
#
#   provision.sh driver    NVIDIA server driver (headless) + nvidia-smi
#   provision.sh userland  verify the GPU, then nvidia-container-toolkit,
#                          Tailscale, K3s (agent, not enabled), gVisor,
#                          Ollama (unit disabled) + pre-pulled models
#
# Nothing here joins a network or cluster: the GCE startup script
# (startup.sh.tftpl) does that on each VM.
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

log() { echo "==> $*"; }

apt_install() {
  apt-get install -y --no-install-recommends "$@"
}

stage_driver() {
  : "${NVIDIA_DRIVER_BRANCH:?}"

  # Keep the kernel and driver fixed for the life of the image; unattended
  # upgrades on a short-lived node would only race the startup script.
  systemctl disable --now unattended-upgrades.service apt-daily.timer apt-daily-upgrade.timer || true

  apt-get update
  apt-get -y upgrade
  apt_install ubuntu-drivers-common ca-certificates curl gnupg

  log "installing NVIDIA ${NVIDIA_DRIVER_BRANCH}-server driver"
  # Uses Canonical's prebuilt, signed kernel modules for the GCP kernel.
  ubuntu-drivers install --gpgpu "nvidia:${NVIDIA_DRIVER_BRANCH}-server"
  apt_install "nvidia-utils-${NVIDIA_DRIVER_BRANCH}-server"
}

install_nvidia_container_toolkit() {
  log "installing nvidia-container-toolkit"
  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey |
    gpg --dearmor --yes -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  curl -fsSL https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list |
    sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' \
      >/etc/apt/sources.list.d/nvidia-container-toolkit.list
  apt-get update
  apt_install nvidia-container-toolkit
  # No `nvidia-ctk runtime configure` here: K3s finds
  # /usr/bin/nvidia-container-runtime at agent start and adds the "nvidia"
  # runtime (and RuntimeClass) to its own containerd config.
}

install_tailscale() {
  log "installing Tailscale"
  curl -fsSL https://tailscale.com/install.sh | sh
  # Installed but not logged in. Drop any state so VMs never share a node key.
  systemctl stop tailscaled
  rm -rf /var/lib/tailscale/*
}

install_k3s() {
  : "${K3S_VERSION:?}"
  log "installing K3s ${K3S_VERSION} (agent, not enabled)"
  curl -sfL https://get.k3s.io |
    INSTALL_K3S_VERSION="$K3S_VERSION" \
      INSTALL_K3S_EXEC="agent" \
      INSTALL_K3S_SKIP_START=true \
      INSTALL_K3S_SKIP_ENABLE=true \
      sh -
  test -f /etc/systemd/system/k3s-agent.service
}

install_gvisor() {
  local release="${GVISOR_RELEASE:-latest}" arch url tmp
  arch="$(uname -m)"
  url="https://storage.googleapis.com/gvisor/releases/release/${release}/${arch}"
  tmp="$(mktemp -d)"
  log "installing gVisor ${release}"
  (
    cd "$tmp"
    for f in runsc containerd-shim-runsc-v1; do
      curl -fsSLO "${url}/${f}"
      curl -fsSLO "${url}/${f}.sha512"
      sha512sum -c "${f}.sha512"
    done
    install -m 0755 runsc containerd-shim-runsc-v1 /usr/local/bin/
  )
  rm -rf "$tmp"

  # Register runsc with K3s's containerd. K3s renders config-v3.toml.tmpl on
  # containerd 2.x releases and config.toml.tmpl on containerd 1.7 releases;
  # both extend the generated config (which includes the nvidia runtime).
  local dir=/var/lib/rancher/k3s/agent/etc/containerd
  install -d -m 0755 "$dir"
  cat >"$dir/config-v3.toml.tmpl" <<'TOML'
{{ template "base" . }}

[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.runsc]
  runtime_type = "io.containerd.runsc.v1"
TOML
  cat >"$dir/config.toml.tmpl" <<'TOML'
{{ template "base" . }}

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runsc]
  runtime_type = "io.containerd.runsc.v1"
TOML
}

install_ollama() {
  local models_dir=/var/lib/ollama/models model
  log "installing Ollama ${OLLAMA_VERSION:-latest}"
  curl -fsSL https://ollama.com/install.sh | OLLAMA_VERSION="${OLLAMA_VERSION:-}" sh

  install -d -m 0755 /var/lib/ollama
  install -d -m 0755 -o ollama -g ollama "$models_dir"
  install -d -m 0755 /etc/systemd/system/ollama.service.d
  cat >/etc/systemd/system/ollama.service.d/models.conf <<UNIT
[Service]
Environment=OLLAMA_MODELS=${models_dir}
UNIT
  systemctl daemon-reload
  systemctl restart ollama

  for _ in $(seq 1 30); do
    ollama list >/dev/null 2>&1 && break
    sleep 2
  done
  for model in ${OLLAMA_MODELS_TO_PULL:-}; do
    log "pulling ${model}"
    ollama pull "$model"
  done
  ollama list

  # Binary and models only; the cluster runs Ollama as a pod that mounts
  # /var/lib/ollama/models from the host.
  systemctl disable --now ollama
  rm -f /usr/share/ollama/.ollama/id_ed25519 /usr/share/ollama/.ollama/id_ed25519.pub
}

cleanup() {
  log "cleaning up"
  apt-get -y autoremove
  apt-get clean
  rm -rf /var/lib/apt/lists/*
  # Each VM gets its own machine-id (K3s/flannel and journald key off it).
  truncate -s 0 /etc/machine-id
  rm -f /var/lib/dbus/machine-id
  rm -f /var/tmp/provision.sh
}

stage_userland() {
  log "verifying the GPU"
  nvidia-smi

  install_nvidia_container_toolkit
  install_tailscale
  install_k3s
  install_gvisor
  install_ollama
  cleanup
}

case "${1:-}" in
  driver) stage_driver ;;
  userland) stage_userland ;;
  *)
    echo "usage: $0 driver|userland" >&2
    exit 2
    ;;
esac
