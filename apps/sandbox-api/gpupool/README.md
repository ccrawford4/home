# GPU pool scaler

sandbox-api keeps the GCE managed instance group behind the GPU nodes between
0 and `GPU_POOL_MAX_SIZE` nodes.

- **Scale up** to 1 when a pod requesting `nvidia.com/gpu` is pending, or when
  something calls `POST /gpu-pool/demand`. With a larger max size it adds one
  node at a time while GPU pods stay pending and every requested node has joined.
- **Scale down** to 0 once no GPU pod has run on a GPU node and no demand call
  has arrived for `GPU_POOL_IDLE_TIMEOUT`. It cordons and drains the nodes
  first, backs off and uncordons if GPU work shows up meanwhile, then resizes
  the group and deletes the Node objects.
- DaemonSet pods (the per-node Ollama, the device plugin) don't count as GPU
  work. Inference through Ollama keeps the pool up only through demand calls,
  so the gateway calls `/gpu-pool/demand` for every request it sends to a GPU.

## API

| Route | Response |
|---|---|
| `POST /gpu-pool/demand` | `202` with the pool status. Optional body `{"model": "...", "reason": "..."}`, logged only. Returns at once; it does not wait for a node. |
| `GET /gpu-pool/status` | `200` with `target_size`, `max_size`, `stable`, `pending_gpu_pods`, `gpu_pods`, `gpu_nodes`, `last_activity`, `idle_for`, `dry_run`, `last_sync`, `last_error`. |

Both return `503` when the scaler is disabled.

## Configuration

| Variable | Default | |
|---|---|---|
| `GPU_POOL_ENABLED` | `false` | Runs the scaler |
| `GPU_POOL_DRY_RUN` | `false` | Logs resizes, cordons, evictions and node deletions instead of doing them, and tracks the group's size in memory without calling GCE |
| `GPU_POOL_PROJECT` | | `home-473419` |
| `GPU_POOL_ZONE` | | `us-central1-a` |
| `GPU_POOL_MIG` | | `gpu-pool` |
| `GPU_POOL_NODE_SELECTOR` | `accelerator=nvidia-l4` | Label that marks GPU nodes |
| `GPU_POOL_MAX_SIZE` | `1` | |
| `GPU_POOL_IDLE_TIMEOUT` | `10m` | |
| `GPU_POOL_SYNC_INTERVAL` | `30s` | How often it reconciles; demand calls also trigger one |

Project, zone and MIG are required unless dry run is on.

## Deploying

GCP credentials come from Application Default Credentials. On this cluster
that means workload identity federation, set up the same way as
`helm/atlantis`: a projected service account token with the
`home-cluster-oidc-provider` audience mounted at
`/var/run/secrets/workload-identity-federation/token`, and
`GOOGLE_APPLICATION_CREDENTIALS` pointing at an `external_account` config that
impersonates `gpu-pool-scaler@home-473419.iam.gserviceaccount.com`. That
service account needs `compute.instanceGroupManagers.get` and `.update` on
the group.

The `sandbox-api` service account in namespace `sandbox-api` needs:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: sandbox-api-gpu-pool
rules:
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["get", "list", "patch", "delete"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["list"]
  - apiGroups: [""]
    resources: ["pods/eviction"]
    verbs: ["create"]
```
