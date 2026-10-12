# GPU node image

On-demand GPU workers for the home K3s cluster. `terraform/gpu_pool.tf` creates
a zonal managed instance group, `gpu-pool`, of Spot `g2-standard-4` VMs (one
NVIDIA L4 each). The group sits at size 0; the sandbox-api control plane
resizes it to 1 when GPU work arrives, and back to 0 afterwards.

Each VM:

1. boots the newest image in the `gpu-node` family (built here with Packer),
2. reads its K3s node token and Tailscale auth key from Secret Manager,
3. joins the tailnet, then joins the cluster as a K3s agent with its Tailscale
   IP as node IP, flannel over `tailscale0`, label `accelerator=nvidia-l4` and
   taint `gpu=true:NoSchedule` (`startup.sh.tftpl`, run on every boot).

| File | Purpose |
| --- | --- |
| `gpu-node.pkr.hcl` | Packer template (googlecompute, Ubuntu 22.04, Spot g2-standard-4, image family `gpu-node`). |
| `provision.sh` | Installs the NVIDIA server driver, nvidia-container-toolkit, Tailscale (logged out), K3s agent (not enabled), gVisor `runsc`, and Ollama (unit disabled) with pre-pulled models in `/var/lib/ollama/models`. |
| `startup.sh.tftpl` | GCE startup script, rendered by Terraform into the instance template metadata. |

What the image gives workloads:

- K3s detects `nvidia-container-runtime` and registers the `nvidia` containerd
  runtime and RuntimeClass. GPU pods set `runtimeClassName: nvidia`, tolerate
  `gpu=true:NoSchedule` and select `accelerator: nvidia-l4`. Requesting
  `nvidia.com/gpu` also needs the NVIDIA device plugin in the cluster.
- `runsc` is registered as containerd runtime handler `runsc` (via
  `config.toml.tmpl` / `config-v3.toml.tmpl`); a RuntimeClass with
  `handler: runsc` selects it.
- Ollama models live in `/var/lib/ollama/models` on the host; mount it as a
  `hostPath` and set `OLLAMA_MODELS` to it so the pod starts warm.

## One-time setup

Do these in order.

### 1. Enable the APIs

The repo does not manage project services in Terraform:

```bash
gcloud services enable compute.googleapis.com secretmanager.googleapis.com \
  iam.googleapis.com cloudquotas.googleapis.com --project=<project>
```

The Terraform identity (`home-cluster-sa`, whose project roles are granted
outside Terraform) also needs permission to manage what `gpu_pool.tf` creates:
for example `roles/compute.instanceAdmin.v1`, `roles/compute.securityAdmin`,
`roles/iam.roleAdmin`, `roles/iam.serviceAccountAdmin`,
`roles/iam.serviceAccountUser`, `roles/resourcemanager.projectIamAdmin` and
`roles/secretmanager.admin`.

### 2. Put the K3s server on the tailnet

Install Tailscale on the Pi running the K3s server and name it to match
`k3s_server_url` (default `https://home-k3s:6443`, a MagicDNS name):

```bash
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up --hostname=home-k3s
tailscale ip -4   # the Pi's tailnet IP, 100.x.y.z below
```

Then add these lines to `/etc/rancher/k3s/config.yaml` on the server, next to
the settings from `infrastructure/k3s/config.yaml`, and restart K3s
(`sudo systemctl restart k3s`):

```yaml
# GPU pool: let tailnet-only agents reach the API server and pod network.
tls-san:
  - home-k3s                    # host in k3s_server_url
  - home-k3s.<tailnet>.ts.net
  - 100.x.y.z                   # Pi's Tailscale IP
# Agents replace k3s_server_url with the addresses in the "kubernetes"
# endpoints after they join, so advertise the tailnet IP, not the LAN IP.
advertise-address: 100.x.y.z
node-external-ip: 100.x.y.z
# Run flannel VXLAN over the tailnet so pods on GPU nodes reach pods on the Pi.
flannel-iface: tailscale0
```

Notes:

- Every node that GPU-node pods must reach over the pod network needs the same
  treatment (Tailscale plus `flannel-iface: tailscale0`). A node whose flannel
  endpoint is a LAN IP is unreachable from GCP.
- Flannel takes its MTU from `tailscale0` (1280), so pod MTU becomes 1230
  cluster-wide.
- The cluster now depends on `tailscaled` on the Pi being up.

If the server URL is not `https://home-k3s:6443`, set the Terraform variable
`k3s_server_url` (Atlantis reads variables from `TF_VAR_*` env vars).

### 3. Tailscale tag, ACL and auth key

In the tailnet policy file, declare the tag and allow the K3s traffic:

```jsonc
"tagOwners": { "tag:gpu-node": ["autogroup:admin"] },
"grants": [
  // agent -> API server
  { "src": ["tag:gpu-node"], "dst": ["home-k3s"], "ip": ["tcp:6443"] },
  // kubelet (logs/exec/metrics) and flannel VXLAN, both directions
  { "src": ["home-k3s"], "dst": ["tag:gpu-node"], "ip": ["tcp:10250", "udp:8472"] },
  { "src": ["tag:gpu-node"], "dst": ["home-k3s"], "ip": ["tcp:10250", "udp:8472"] }
]
```

(`hosts` entries or the Pi's own tag can stand in for `home-k3s`.) Then create
an auth key in the admin console (Settings > Keys): **Reusable**,
**Ephemeral** (nodes vanish from the tailnet once the VM is deleted),
**Pre-approved**, tags `tag:gpu-node`.

### 4. Apply Terraform, then set the secrets

After Atlantis applies `terraform/gpu_pool.tf`, set the two secret values:

```bash
# On the K3s server:
sudo cat /var/lib/rancher/k3s/server/node-token |
  gcloud secrets versions add gpu-node-k3s-token --data-file=-

echo -n "tskey-auth-..." |
  gcloud secrets versions add gpu-node-tailscale-authkey --data-file=-
```

### 5. GPU quota

New projects have 0 Spot L4 quota and 0 global GPU quota. Terraform can file
the requests (`google_cloud_quotas_quota_preference`) when
`gpu_quota_requests_enabled = true`; that needs the Cloud Quotas API and
`roles/cloudquotas.admin` on the Terraform identity. Otherwise request them
by hand. Check the quota IDs first, since Compute Engine is moving GPU quotas
to per-family IDs:

```bash
gcloud quotas info list --service=compute.googleapis.com --project=<project> \
  --filter="quotaId~GPU" --format="value(quotaId)"

gcloud quotas preferences create --project=<project> \
  --service=compute.googleapis.com \
  --quota-id=PREEMPTIBLE-NVIDIA-L4-GPUS-per-project-region \
  --dimensions=region=us-central1 --preferred-value=1 --email=<you>

gcloud quotas preferences create --project=<project> \
  --service=compute.googleapis.com \
  --quota-id=GPUS-ALL-REGIONS-per-project --preferred-value=1 --email=<you>
```

The image build below runs on a Spot L4 too, so it needs this quota.

### 6. Build the image

Pin K3s to the server's version (`k3s --version` on the Pi); an agent must not
be newer than its server.

```bash
cd infrastructure/gpu-image
packer init .
packer build \
  -var project_id=<project> \
  -var k3s_version=v1.33.4+k3s1 \
  .
```

Optional variables: `zone` (default `us-central1-a`),
`nvidia_driver_branch` (`570`), `ollama_version` (latest),
`ollama_models` (`["qwen2.5-coder:7b"]`), `gvisor_release` (`latest`).
Rebuilding publishes a newer image in the `gpu-node` family; VMs created
afterwards use it with no Terraform change. The build VM is Spot, so a
preemption aborts the build; run it again.

## Operating the pool

Scale by hand:

```bash
gcloud compute instance-groups managed resize gpu-pool --size=1 --zone=us-central1-a
kubectl get nodes -l accelerator=nvidia-l4 -w
gcloud compute instance-groups managed resize gpu-pool --size=0 --zone=us-central1-a
```

Terraform ignores `target_size`, so applies never fight the control plane.
The control plane (Kubernetes service account `sandbox-api/sandbox-api`)
impersonates the `gpu-pool-scaler` Google service account through Workload
Identity Federation; see `terraform/gpu_pool.tf` for its permissions.

When a Spot VM is preempted or scaled away, its Kubernetes Node object stays
behind as `NotReady`; whoever scales down should `kubectl delete node` it.

Debugging: there is no open SSH port. Use Tailscale SSH
(`tailscale ssh <user>@gpu-node-xxxx`, allowed by an `ssh` rule in the tailnet
policy), or the serial console. Startup logs:
`sudo journalctl -u google-startup-scripts` and `sudo journalctl -u k3s-agent`.
