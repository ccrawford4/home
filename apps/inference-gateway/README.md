# inference-gateway

An Ollama-API-compatible gateway. Clients call it exactly like Ollama; it picks
a backend for each request by model name, enforces per-client privacy, queues
requests per backend and streams responses straight through.

Backends today: the Hailo NPU server (hailo-ollama) at
`http://virgo.virgo.svc.cluster.local:8000`. Later: GPU nodes running stock
Ollama that register themselves at runtime.

## Configuration

YAML at `CONFIG_PATH` (default `/etc/inference-gateway/config.yaml`):

```yaml
listen: ":8080"           # default :8080
queueDepth: 16            # per-backend waiting requests before 429 (0/omitted → 16)
registrationTTL: 60s      # dynamic backends expire without a heartbeat (default 60s)
demandURL: http://sandbox-api.sandbox-api.svc.cluster.local:8080/gpu-pool/demand  # optional; GPU demand signals
demandInterval: 30s       # throttle for "gpu request" signals (default 30s)
backends:                 # static backends
  - name: hailo           # no '/'; gpu* names (not bare "gpu") join the gpu wildcard
    url: http://virgo.virgo.svc.cluster.local:8000
    onPrem: true          # private requests may only use onPrem backends
    concurrency: 1        # simultaneous requests; 0 = unlimited
    keepAlive: -1         # optional; any scalar (-1, 0, "5m")
models:                   # catalog, in order: model → backends that serve it
  qwen2.5-coder:1.5b: [hailo]
  qwen2:1.5b: [hailo]
  qwen2.5-coder:7b: [gpu] # "gpu" = every backend named gpu* (static or registered)
tiers:                    # virtual model names → ordered backend entries
  fast: [gpu, hailo]
  local: [hailo]
clients:
  - name: sandbox-agents
    private: false
  - name: personal-agent
    private: true
```

| Env | Default | |
|---|---|---|
| `CONFIG_PATH` | `/etc/inference-gateway/config.yaml` | Config file |
| `KEYS_DIR` | `/etc/inference-gateway/keys` | One file per client, named after it; content is its key (whitespace trimmed) |
| `ADMIN_KEY_FILE` | `/etc/inference-gateway/admin/key` | Admin key for `/internal/backends`. Missing → registration disabled |

Keys are read at startup. Clients without a key file are logged and cannot
authenticate.

## Auth

Every `/api/*` and `/v1/*` request needs `Authorization: Bearer <client key>`,
otherwise 401. The gateway does not forward the client's `Authorization`
header to backends. `GET /healthz` is open. `/internal/*` takes the admin key.

## Privacy

A request is private if its client has `private: true`, or if it sends
`X-Inference-Private: true` (or `1`). The header can make a request private but
never makes a private client public. Private requests only go to `onPrem: true`
backends. There are no model-based privacy rules.

## Routing

The `model` field of the request body (for `/api/show`, `name` is accepted
when `model` is absent) resolves to an ordered candidate list:

1. **`<backend>/<model>`**, where `<backend>` is a known backend name: pinned
   to that backend; `<model>` (prefix stripped) is forwarded. The model need
   not be in the catalog. Unknown prefixes are treated as plain model names.
2. **Tier name**: walk the tier's entries in order. Each entry expands to the
   matching backends (`gpu` → every registered `gpu*` backend, sorted by
   name). Each backend is sent the **first model in the catalog (file order)
   that lists that backend** (or `gpu`, for gpu backends); backends no catalog
   model lists are skipped. With the example config, `fast` sends
   `qwen2.5-coder:7b` to a GPU node, else `qwen2.5-coder:1.5b` to hailo.
3. **Plain model**: the backends listed for it in `models`. A plain model is
   never swapped for another model.

Candidates are then filtered by privacy and health. If none remain: 503
`{"error":"no backend available for model X"}` (with a note when privacy
filtering removed candidates).

**Fallback**: candidates are tried in order. The gateway moves to the next one
only if the connection fails or the backend returns 5xx, before anything is
written to the client; the last candidate's 5xx is passed through. Once
streaming starts, errors pass through. If every connection fails: 502.

**Concurrency**: each backend has a semaphore of size `concurrency` with a FIFO
wait queue of `queueDepth`. A request that finds the queue full skips to the
next candidate; if no candidate had room, 429 `{"error":"backend busy"}`.
Requests whose client disconnects leave the queue.

**keep_alive**: for backends with `keepAlive` set, it is added to
`/api/chat` and `/api/generate` bodies that don't have a `keep_alive` key.
Bodies are forwarded byte-for-byte unless the model name or keep_alive changes.

**Streaming**: responses (NDJSON or SSE) are copied unchanged and flushed after
every read. The final `done: true` chunk is parsed (not altered) and logged
with backend, model, client, `eval_count`, `eval_duration` and
`tok_s = eval_count / (eval_duration / 1e9)`.

**Health**: every backend's `GET /api/version` is polled every 10s (2s
timeout). One failure marks it unhealthy; one success marks it healthy. Static
backends start healthy, and registration marks a dynamic backend healthy.

## GPU demand signalling

When `demandURL` is set, the gateway POSTs `{"model": "<model>", "reason": "<reason>"}`
to it (2s timeout, in the background; failures are only logged and never affect
the client's request):

| Reason | When | Throttle |
|---|---|---|
| `no gpu backend` | The requested model's catalog entry, or tier, includes `gpu`, but no registered `gpu*` backend is healthy. Sent even if the request is then served by another backend (e.g. `fast` falling back to hailo). `model` is the requested model, or for a tier the first catalog model listing `gpu` | at most once per 5s |
| `gpu request` | A request is sent to a `gpu*` backend (by catalog, tier or `gpu-x/` prefix). `model` is the model forwarded | at most once per `demandInterval` |

Each reason is throttled on its own, across all models. Not set → no signals.

## Endpoints

| Method | Path | |
|---|---|---|
| POST | `/api/chat`, `/api/generate`, `/api/show` | Routed by model |
| POST | `/v1/chat/completions` | OpenAI-compatible; routed by model, passed through |
| GET | `/api/tags` | Catalog models, then `<backend>/<model>` for every model each healthy backend reports from its own `/api/tags` (only on-prem backends for private requests) |
| GET | `/api/version` | `{"version":"inference-gateway"}` |
| GET | `/healthz` | 200, no auth |
| POST | `/internal/backends` | Admin. Register or heartbeat a dynamic backend: `{"name":"gpu-abc","url":"http://10.0.0.5:11434","onPrem":false,"concurrency":4}` (optional `keepAlive`). Expires after `registrationTTL` without another POST. Static names → 409 |
| DELETE | `/internal/backends/{name}` | Admin. Remove a dynamic backend (204, 404, or 409 for static) |

## Run locally

```sh
mkdir -p /tmp/ig/keys && echo -n devkey > /tmp/ig/keys/sandbox-agents && echo -n admin > /tmp/ig/admin
CONFIG_PATH=./config.yaml KEYS_DIR=/tmp/ig/keys ADMIN_KEY_FILE=/tmp/ig/admin go run .

curl -H 'Authorization: Bearer devkey' localhost:8080/api/tags
curl -H 'Authorization: Bearer devkey' localhost:8080/api/chat \
  -d '{"model":"local","messages":[{"role":"user","content":"hi"}]}'
```

Point the backend URL at a local Ollama (`http://localhost:11434`) to try it
without the cluster.

```sh
go vet ./... && go test -race ./...
docker build -t inference-gateway .
```
