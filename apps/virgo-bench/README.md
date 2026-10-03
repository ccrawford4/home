# virgo-bench

Benchmark suite for the models served by virgo (hailo-ollama on the Hailo NPU).
Pick a model in the UI, run the suite, and compare runs side by side. Results
are stored in MySQL.

- **UI and API:** `https://bench.calum.sh`, behind Cloudflare Access (admins only)
- **Target:** `http://virgo.virgo.svc.cluster.local:8000`, called from inside the cluster so the timings leave out Cloudflare

## What a run measures

| Phase | What it does | Metrics |
|---|---|---|
| Cold start | Unloads the model (`keep_alive: 0`), then sends one streamed request | Cold TTFT, and the load penalty (cold TTFT minus warm median TTFT) |
| Warm-up | Only runs when cold start is off, and isn't counted | |
| Warm runs | N streamed requests with the same prompt | TTFT (client and server), TTLT, decode tok/s, output tokens: mean, p50, min and max |
| Multi-turn | A scripted 4-turn chat that resends the full history each turn | TTFT and TTLT per turn, total time, and whether turn 4 recalls the name given in turn 1 |
| Tool calling | 5 prompts, each offered all 5 tools, sent with `stream: false` | Pass rate (right tool and all required arguments present) and latency |

hailo-ollama 0.5.1 reports only `total_duration` and `eval_count`, so some metrics are derived:

- **Server TTFT** is `total_duration − (done.created_at − first_token.created_at)`. When a server reports `prompt_eval_duration` (stock Ollama), that is used instead.
- **Decode tok/s** is `(eval_count − 1) / (done.created_at − first.created_at)`. Without server timestamps, it falls back to the client-side chunk rate.
- **Model load time** is missing from `total_duration`, so the cold-start cost is measured on the client.

Every request keeps its raw chunks (client receive offset plus server `created_at`), so any metric can be recomputed later.

Runs go through a single in-process queue, one at a time, because the NPU serves one request at a time. Avoid chatting on chat.calum.sh during a run. Keep the deployment at one replica.

## API

| Method | Path | |
|---|---|---|
| GET | `/api/models` | Model names from virgo's `/api/tags` |
| GET | `/api/runs` | Recent runs with their summaries |
| POST | `/api/runs` | Queue a run: `{"model": "...", "warm_runs": 5, "num_predict": 64, "cold": true, "multi_turn": true, "tools": true}` |
| GET | `/api/runs/{id}` | Run plus its requests. Add `?chunks=0` to leave out the raw chunks |
| DELETE | `/api/runs/{id}` | Delete a run (not allowed while it is running) |

## Configuration

| Env | Default |
|---|---|
| `VIRGO_URL` | `http://virgo.virgo.svc.cluster.local:8000` |
| `MYSQL_HOST` | unset, which uses an in-memory store |
| `MYSQL_PORT` | `3306` |
| `MYSQL_DATABASE` | `virgobench` |
| `MYSQL_USERNAME` / `MYSQL_PASSWORD` | |
| `PORT` | `8080` |

The schema is created on startup. Runs left `running` by a restart are marked failed, and `queued` runs are picked up again.

## Local development

```bash
cd web && npm ci && npm run build && cd ..
kubectl -n virgo port-forward svc/virgo 8000:8000 &
VIRGO_URL=http://localhost:8000 go run .          # in-memory store, http://localhost:8080
# UI hot reload: (cd web && npm run dev), which proxies /api to :8080
go test ./...
```

## Deploying

1. Add values for the three secrets in Google Secret Manager: `virgo-bench-db-username`, `virgo-bench-db-password`, `virgo-bench-db-root-password`. Terraform creates the secrets but not their values.
2. Build and push the arm64 image:
   ```bash
   docker buildx build --platform linux/arm64 \
     -t us-central1-docker.pkg.dev/home-473419/internal/virgo-bench:v0.1.0 --push apps/virgo-bench
   ```
3. Apply Terraform for the DNS record, tunnel route, Access app and secrets. Argo CD then syncs `helm/virgo-bench`, `helm/namespaces` and `helm/networking`.
