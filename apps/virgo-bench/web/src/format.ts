import type { Run } from "./api";

export const fmtMs = (v: number | null | undefined) =>
  v == null ? "–" : v >= 1000 ? `${(v / 1000).toFixed(2)} s` : `${Math.round(v)} ms`;
export const fmtNum = (v: number | null | undefined, digits = 1) =>
  v == null ? "–" : v.toFixed(digits);
export const fmtDate = (s?: string) => (s ? new Date(s).toLocaleString() : "–");

export type Row = {
  label: string;
  get: (r: Run) => number | null | undefined;
  better?: "low" | "high";
  fmt: (v: number | null | undefined) => string;
};

export const compareRows: Row[] = [
  { label: "TTFT p50 (client)", get: (r) => r.summary?.warm?.client_ttft_ms?.p50, better: "low", fmt: fmtMs },
  { label: "TTFT p50 (server)", get: (r) => r.summary?.warm?.server_ttft_ms?.p50, better: "low", fmt: fmtMs },
  { label: "TTLT p50", get: (r) => r.summary?.warm?.client_ttlt_ms?.p50, better: "low", fmt: fmtMs },
  { label: "Decode tok/s p50", get: (r) => r.summary?.warm?.decode_tps?.p50, better: "high", fmt: (v) => fmtNum(v) },
  { label: "Output tokens p50", get: (r) => r.summary?.warm?.eval_count?.p50, fmt: (v) => fmtNum(v, 0) },
  { label: "Cold TTFT", get: (r) => r.summary?.cold?.client_ttft_ms, better: "low", fmt: fmtMs },
  { label: "Load penalty", get: (r) => r.summary?.cold?.load_penalty_ms, better: "low", fmt: fmtMs },
  { label: "Multi-turn total", get: (r) => r.summary?.multi_turn?.total_ms, better: "low", fmt: fmtMs },
  {
    label: "Turn 4 TTFT",
    get: (r) => r.summary?.multi_turn?.turns?.[3]?.client_ttft_ms,
    better: "low",
    fmt: fmtMs,
  },
  {
    label: "Tool pass rate",
    get: (r) => (r.summary?.tools ? r.summary.tools.pass_rate * 100 : null),
    better: "high",
    fmt: (v) => (v == null ? "–" : `${Math.round(v)}%`),
  },
  { label: "Tool latency p50", get: (r) => r.summary?.tools?.latency_ms?.p50, better: "low", fmt: fmtMs },
];

export const recall = (r: Run) =>
  r.summary?.multi_turn ? (r.summary.multi_turn.recall_ok ? "yes" : "no") : "–";

export const phaseTitle: Record<string, string> = {
  cold: "Cold start (model unloaded first)",
  warmup: "Warm-up (not counted)",
  warm: "Warm runs",
  multiturn: "Multi-turn conversation",
  tools: "Tool calling (stream: false)",
};

export const phaseOrder = ["cold", "warmup", "warm", "multiturn", "tools"];
