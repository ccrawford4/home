// Markdown versions of the result tables, for pasting into chats and issues.
import type { BenchRequest, Run } from "./api";
import { compareRows, fmtDate, fmtMs, fmtNum, phaseOrder, phaseTitle, recall } from "./format";

const cell = (v: unknown) =>
  String(v ?? "")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/\|/g, "\\|") || " ";

/** Renders a GitHub-flavored Markdown table. `right` marks numeric columns. */
export function mdTable(head: string[], rows: unknown[][], right: boolean[] = []) {
  const line = (cells: unknown[]) => `| ${cells.map(cell).join(" | ")} |`;
  const rule = head.map((_, i) => (right[i] ? "---:" : "---"));
  return [line(head), `| ${rule.join(" | ")} |`, ...rows.map(line)].join("\n");
}

const runConfig = (r: Run) =>
  `${r.config.warm_runs} warm runs · max ${r.config.num_predict} output tokens`;

/** The runs list, one row per run. */
export function runsMd(runs: Run[]) {
  return mdTable(
    ["#", "Model", "Status", "TTFT p50", "Server TTFT", "TTLT p50", "tok/s", "Cold TTFT", "Recall", "Tools", "Created"],
    runs.map((r) => {
      const s = r.summary;
      return [
        r.id,
        r.model,
        r.status,
        fmtMs(s?.warm?.client_ttft_ms?.p50),
        fmtMs(s?.warm?.server_ttft_ms?.p50),
        fmtMs(s?.warm?.client_ttlt_ms?.p50),
        fmtNum(s?.warm?.decode_tps?.p50),
        fmtMs(s?.cold?.client_ttft_ms),
        recall(r),
        s?.tools ? `${s.tools.passed}/${s.tools.total}` : "–",
        fmtDate(r.created_at),
      ];
    }),
    [false, false, false, true, true, true, true, true, false, true, false],
  );
}

/** Runs side by side, one column per run, the same rows as the Compare card. */
export function compareMd(runs: Run[]) {
  const rows = compareRows.map((row) => [row.label, ...runs.map((r) => row.fmt(row.get(r)))]);
  rows.push(["History recall", ...runs.map(recall)]);
  const table = mdTable(
    ["Metric", ...runs.map((r) => `${r.model} (#${r.id})`)],
    rows,
    [false, ...runs.map(() => true)],
  );
  const configs = runs.map((r) => `- #${r.id} ${r.model}: ${runConfig(r)}`).join("\n");
  return `${table}\n\n${configs}`;
}

/** One run's headline numbers, as a Metric / Value table under a title. */
export function runSummaryMd(run: Run) {
  const head = `### ${run.model} · run #${run.id}\n\n${runConfig(run)} · ${run.status}`;
  if (!run.summary) return head;
  const rows = compareRows.map((row) => [row.label, row.fmt(row.get(run))]);
  rows.push(["History recall", recall(run)]);
  const tools = run.summary.tools;
  if (tools) rows.push(["Tools passed", `${tools.passed}/${tools.total}`]);
  return `${head}\n\n${mdTable(["Metric", "Value"], rows, [false, true])}`;
}

/** One phase's requests, the same columns as its table on the run page. */
export function phaseMd(phase: string, requests: BenchRequest[]) {
  const pass = phase === "tools" || phase === "multiturn";
  const head = ["#", "Prompt", "TTFT", "Server TTFT", "TTLT", "Tokens", "tok/s", ...(pass ? ["Pass"] : []), "HTTP"];
  const rows = requests.map((q) => [
    q.seq,
    q.prompt,
    fmtMs(q.client_ttft_ms),
    fmtMs(q.server_ttft_ms),
    fmtMs(q.client_ttlt_ms),
    q.eval_count ?? "–",
    fmtNum(q.decode_tps),
    ...(pass ? [q.passed == null ? "" : q.passed ? "Pass" : "Fail"] : []),
    q.http_status || "–",
  ]);
  const right = head.map((h) => ["TTFT", "Server TTFT", "TTLT", "Tokens", "tok/s"].includes(h));
  return `#### ${phaseTitle[phase] ?? phase}\n\n${mdTable(head, rows, right)}`;
}

/** The whole run: its summary, then every phase that has requests. */
export function runMd(run: Run, requests: BenchRequest[]) {
  const phases = phaseOrder
    .map((p) => [p, requests.filter((q) => q.phase === p)] as const)
    .filter(([, qs]) => qs.length > 0)
    .map(([p, qs]) => phaseMd(p, qs));
  return [runSummaryMd(run), ...phases].join("\n\n");
}
