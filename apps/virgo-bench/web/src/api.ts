export interface Agg {
  n: number;
  mean: number;
  p50: number;
  min: number;
  max: number;
}

export interface Summary {
  cold?: {
    client_ttft_ms: number | null;
    server_ttft_ms: number | null;
    client_ttlt_ms: number | null;
    load_penalty_ms: number | null;
    error?: string;
  };
  warm?: {
    client_ttft_ms: Agg | null;
    server_ttft_ms: Agg | null;
    client_ttlt_ms: Agg | null;
    decode_tps: Agg | null;
    eval_count: Agg | null;
    errors: number;
  };
  multi_turn?: {
    turns: {
      turn: number;
      client_ttft_ms: number | null;
      server_ttft_ms: number | null;
      client_ttlt_ms: number | null;
      error?: string;
    }[];
    total_ms: number;
    recall_ok: boolean;
    completed: boolean;
  };
  tools?: { passed: number; total: number; pass_rate: number; latency_ms: Agg | null };
}

export interface RunConfig {
  model: string;
  warm_runs: number;
  num_predict: number;
  cold: boolean;
  multi_turn: boolean;
  tools: boolean;
}

export interface Run {
  id: number;
  model: string;
  status: "queued" | "running" | "done" | "failed";
  progress: string;
  config: RunConfig;
  summary?: Summary;
  error?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
}

export interface BenchRequest {
  id: number;
  phase: string;
  seq: number;
  prompt: string;
  response: string;
  http_status: number;
  error?: string;
  client_ttft_ms: number | null;
  client_ttlt_ms: number | null;
  server_ttft_ms: number | null;
  server_total_ms: number | null;
  eval_count: number | null;
  decode_tps: number | null;
  passed: boolean | null;
  tool_calls?: unknown;
  started_at: string;
}

async function call<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (!res.ok) {
    let msg = `HTTP ${res.status}`;
    try {
      msg = (await res.json()).error ?? msg;
    } catch {
      /* not JSON */
    }
    throw new Error(msg);
  }
  return res.status === 204 ? (undefined as T) : res.json();
}

export const api = {
  models: () => call<{ models: string[] }>("/api/models").then((r) => r.models),
  runs: () => call<{ runs: Run[] }>("/api/runs?limit=200").then((r) => r.runs),
  run: (id: number) =>
    call<{ run: Run; requests: BenchRequest[] }>(`/api/runs/${id}?chunks=0`),
  create: (cfg: RunConfig) =>
    call<Run>("/api/runs", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    }),
  remove: (id: number) => call<void>(`/api/runs/${id}`, { method: "DELETE" }),
};
