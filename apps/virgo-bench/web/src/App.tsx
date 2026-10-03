import { Fragment, useCallback, useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { api, BenchRequest, Run, RunConfig } from "./api";

const fmtMs = (v: number | null | undefined) =>
  v == null ? "–" : v >= 1000 ? `${(v / 1000).toFixed(2)} s` : `${Math.round(v)} ms`;
const fmtNum = (v: number | null | undefined, digits = 1) =>
  v == null ? "–" : v.toFixed(digits);
const fmtDate = (s?: string) => (s ? new Date(s).toLocaleString() : "–");

function useHashRoute(): [number | null, (id: number | null) => void] {
  const parse = () => {
    const m = window.location.hash.match(/^#\/runs\/(\d+)/);
    return m ? Number(m[1]) : null;
  };
  const [id, setId] = useState(parse);
  useEffect(() => {
    const on = () => setId(parse());
    window.addEventListener("hashchange", on);
    return () => window.removeEventListener("hashchange", on);
  }, []);
  return [id, (next) => (window.location.hash = next == null ? "" : `#/runs/${next}`)];
}

export default function App() {
  const [runs, setRuns] = useState<Run[]>([]);
  const [error, setError] = useState<string>("");
  const [selected, setSelected] = useState<number[]>([]);
  const [detailId, setDetailId] = useHashRoute();

  const refresh = useCallback(async () => {
    try {
      setRuns(await api.runs());
      setError("");
    } catch (e) {
      setError(String((e as Error).message));
    }
  }, []);

  const active = runs.some((r) => r.status === "queued" || r.status === "running");
  useEffect(() => {
    refresh();
    const t = setInterval(refresh, active ? 2000 : 10000);
    return () => clearInterval(t);
  }, [refresh, active]);

  const toggle = (id: number) =>
    setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));
  const compared = runs.filter((r) => selected.includes(r.id) && r.summary);

  return (
    <div className="page">
      <header>
        <h1>Virgo Bench</h1>
        <p className="muted">
          Latency, throughput, multi-turn and tool-calling benchmarks for models served by virgo.
        </p>
      </header>
      {error && <div className="banner error">{error}</div>}
      {detailId != null ? (
        <RunDetail id={detailId} onBack={() => setDetailId(null)} live={active} />
      ) : (
        <>
          <NewRun onCreated={refresh} />
          {compared.length > 0 && <Compare runs={compared} onClear={() => setSelected([])} />}
          <RunsTable
            runs={runs}
            selected={selected}
            onToggle={toggle}
            onOpen={setDetailId}
            onDelete={async (id) => {
              if (!confirm(`Delete run #${id}?`)) return;
              try {
                await api.remove(id);
                setSelected((s) => s.filter((x) => x !== id));
                refresh();
              } catch (e) {
                setError(String((e as Error).message));
              }
            }}
          />
        </>
      )}
    </div>
  );
}

function NewRun({ onCreated }: { onCreated: () => void }) {
  const [models, setModels] = useState<string[]>([]);
  const [modelErr, setModelErr] = useState("");
  const [cfg, setCfg] = useState<RunConfig>({
    model: "",
    warm_runs: 5,
    num_predict: 64,
    cold: true,
    multi_turn: true,
    tools: true,
  });
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  useEffect(() => {
    api
      .models()
      .then((m) => {
        setModels(m);
        setCfg((c) => ({ ...c, model: c.model || m[0] || "" }));
      })
      .catch((e) => setModelErr(String(e.message)));
  }, []);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await api.create(cfg);
      onCreated();
    } catch (e) {
      setErr(String((e as Error).message));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="card">
      <h2>New run</h2>
      <form onSubmit={submit} className="form">
        <label>
          Model
          {models.length > 0 ? (
            <select value={cfg.model} onChange={(e) => setCfg({ ...cfg, model: e.target.value })}>
              {models.map((m) => (
                <option key={m}>{m}</option>
              ))}
            </select>
          ) : (
            <input
              value={cfg.model}
              placeholder="model name"
              onChange={(e) => setCfg({ ...cfg, model: e.target.value })}
            />
          )}
        </label>
        <label>
          Warm runs
          <input
            type="number"
            min={1}
            max={50}
            value={cfg.warm_runs}
            onChange={(e) => setCfg({ ...cfg, warm_runs: Number(e.target.value) })}
          />
        </label>
        <label>
          Max output tokens
          <input
            type="number"
            min={1}
            max={1024}
            value={cfg.num_predict}
            onChange={(e) => setCfg({ ...cfg, num_predict: Number(e.target.value) })}
          />
        </label>
        <fieldset>
          <label className="check">
            <input
              type="checkbox"
              checked={cfg.cold}
              onChange={(e) => setCfg({ ...cfg, cold: e.target.checked })}
            />
            Cold start
          </label>
          <label className="check">
            <input
              type="checkbox"
              checked={cfg.multi_turn}
              onChange={(e) => setCfg({ ...cfg, multi_turn: e.target.checked })}
            />
            Multi-turn
          </label>
          <label className="check">
            <input
              type="checkbox"
              checked={cfg.tools}
              onChange={(e) => setCfg({ ...cfg, tools: e.target.checked })}
            />
            Tool calling
          </label>
        </fieldset>
        <button type="submit" disabled={busy || !cfg.model}>
          {busy ? "Queuing…" : "Run benchmark"}
        </button>
      </form>
      {modelErr && <p className="muted small">Couldn't list models from virgo: {modelErr}</p>}
      {err && <p className="error small">{err}</p>}
      <p className="muted small">
        Runs execute one at a time because the NPU serves one request at a time. Chats on
        chat.calum.sh during a run will skew its timings.
      </p>
    </section>
  );
}

function StatusBadge({ run }: { run: Run }) {
  return (
    <span className={`badge ${run.status}`} title={run.error}>
      {run.status}
      {run.status === "running" && run.progress ? ` · ${run.progress}` : ""}
    </span>
  );
}

function RunsTable(props: {
  runs: Run[];
  selected: number[];
  onToggle: (id: number) => void;
  onOpen: (id: number) => void;
  onDelete: (id: number) => void;
}) {
  const { runs, selected, onToggle, onOpen, onDelete } = props;
  return (
    <section className="card">
      <h2>Runs</h2>
      {runs.length === 0 ? (
        <p className="muted">No runs yet.</p>
      ) : (
        <div className="scroll">
          <table>
            <thead>
              <tr>
                <th title="Select to compare">Compare</th>
                <th>#</th>
                <th>Model</th>
                <th>Status</th>
                <th className="num">TTFT p50</th>
                <th className="num">Server TTFT</th>
                <th className="num">TTLT p50</th>
                <th className="num">tok/s</th>
                <th className="num">Cold TTFT</th>
                <th>Recall</th>
                <th className="num">Tools</th>
                <th>Created</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {runs.map((r) => {
                const s = r.summary;
                return (
                  <tr key={r.id}>
                    <td>
                      <input
                        type="checkbox"
                        disabled={!s}
                        checked={selected.includes(r.id)}
                        onChange={() => onToggle(r.id)}
                      />
                    </td>
                    <td>{r.id}</td>
                    <td>
                      <a href={`#/runs/${r.id}`} onClick={(e) => (e.preventDefault(), onOpen(r.id))}>
                        {r.model}
                      </a>
                    </td>
                    <td>
                      <StatusBadge run={r} />
                    </td>
                    <td className="num">{fmtMs(s?.warm?.client_ttft_ms?.p50)}</td>
                    <td className="num">{fmtMs(s?.warm?.server_ttft_ms?.p50)}</td>
                    <td className="num">{fmtMs(s?.warm?.client_ttlt_ms?.p50)}</td>
                    <td className="num">{fmtNum(s?.warm?.decode_tps?.p50)}</td>
                    <td className="num">{fmtMs(s?.cold?.client_ttft_ms)}</td>
                    <td>{s?.multi_turn ? (s.multi_turn.recall_ok ? "yes" : "no") : "–"}</td>
                    <td className="num">
                      {s?.tools ? `${s.tools.passed}/${s.tools.total}` : "–"}
                    </td>
                    <td className="small">{fmtDate(r.created_at)}</td>
                    <td>
                      {r.status !== "running" && (
                        <button className="link" onClick={() => onDelete(r.id)}>
                          Delete
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

type Row = { label: string; get: (r: Run) => number | null | undefined; better?: "low" | "high"; fmt: (v: number | null | undefined) => string };

const compareRows: Row[] = [
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

function Compare({ runs, onClear }: { runs: Run[]; onClear: () => void }) {
  const mismatched = new Set(runs.map((r) => r.config.num_predict)).size > 1;
  return (
    <section className="card">
      <div className="row">
        <h2>Compare</h2>
        <button className="link" onClick={onClear}>
          Clear
        </button>
      </div>
      {mismatched && (
        <p className="banner warn small">
          These runs used different max output tokens, so TTLT and multi-turn times aren't
          directly comparable.
        </p>
      )}
      <div className="scroll">
        <table>
          <thead>
            <tr>
              <th>Metric</th>
              {runs.map((r) => (
                <th key={r.id} className="num">
                  {r.model}
                  <div className="muted small">#{r.id}</div>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {compareRows.map((row) => {
              const vals = runs.map(row.get);
              const nums = vals.filter((v): v is number => v != null);
              const best =
                row.better && nums.length > 1 && new Set(nums).size > 1
                  ? row.better === "low"
                    ? Math.min(...nums)
                    : Math.max(...nums)
                  : null;
              return (
                <tr key={row.label}>
                  <td>{row.label}</td>
                  {vals.map((v, i) => (
                    <td key={runs[i].id} className={`num ${v != null && v === best ? "best" : ""}`}>
                      {row.fmt(v)}
                    </td>
                  ))}
                </tr>
              );
            })}
            <tr>
              <td>History recall</td>
              {runs.map((r) => (
                <td key={r.id} className="num">
                  {r.summary?.multi_turn ? (r.summary.multi_turn.recall_ok ? "yes" : "no") : "–"}
                </td>
              ))}
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  );
}

function RunDetail({ id, onBack, live }: { id: number; onBack: () => void; live: boolean }) {
  const [data, setData] = useState<{ run: Run; requests: BenchRequest[] } | null>(null);
  const [err, setErr] = useState("");
  const [open, setOpen] = useState<number | null>(null);

  useEffect(() => {
    let stop = false;
    const load = () =>
      api
        .run(id)
        .then((d) => !stop && setData(d))
        .catch((e) => !stop && setErr(String(e.message)));
    load();
    const t = setInterval(load, live ? 2000 : 15000);
    return () => {
      stop = true;
      clearInterval(t);
    };
  }, [id, live]);

  const phases = useMemo(() => {
    const out: Record<string, BenchRequest[]> = {};
    for (const q of data?.requests ?? []) (out[q.phase] ??= []).push(q);
    return out;
  }, [data]);

  if (err) return <div className="banner error">{err}</div>;
  if (!data) return <p className="muted">Loading…</p>;
  const { run } = data;

  return (
    <>
      <button className="link" onClick={onBack}>
        ← All runs
      </button>
      <section className="card">
        <div className="row">
          <h2>
            #{run.id} · {run.model}
          </h2>
          <StatusBadge run={run} />
        </div>
        <p className="muted small">
          {run.config.warm_runs} warm runs · max {run.config.num_predict} output tokens ·
          started {fmtDate(run.started_at)} · finished {fmtDate(run.finished_at)}
        </p>
        {run.error && <p className="banner error small">{run.error}</p>}
        {run.summary?.cold?.error && (
          <p className="banner warn small">Cold start: {run.summary.cold.error}</p>
        )}
      </section>
      {["cold", "warmup", "warm", "multiturn", "tools"]
        .filter((p) => phases[p])
        .map((p) => (
          <section className="card" key={p}>
            <h3>{phaseTitle[p]}</h3>
            <div className="scroll">
              <table>
                <thead>
                  <tr>
                    <th>#</th>
                    <th>Prompt</th>
                    <th className="num">TTFT</th>
                    <th className="num">Server TTFT</th>
                    <th className="num">TTLT</th>
                    <th className="num">Tokens</th>
                    <th className="num">tok/s</th>
                    {(p === "tools" || p === "multiturn") && <th>Pass</th>}
                    <th>HTTP</th>
                  </tr>
                </thead>
                <tbody>
                  {phases[p].map((q) => (
                    <Fragment key={q.id}>
                      <tr className="clickable" onClick={() => setOpen(open === q.id ? null : q.id)}>
                        <td>{q.seq}</td>
                        <td className="prompt">{q.prompt}</td>
                        <td className="num">{fmtMs(q.client_ttft_ms)}</td>
                        <td className="num">{fmtMs(q.server_ttft_ms)}</td>
                        <td className="num">{fmtMs(q.client_ttlt_ms)}</td>
                        <td className="num">{q.eval_count ?? "–"}</td>
                        <td className="num">{fmtNum(q.decode_tps)}</td>
                        {(p === "tools" || p === "multiturn") && (
                          <td>{q.passed == null ? "" : q.passed ? "✓" : "✗"}</td>
                        )}
                        <td className={q.error ? "error" : ""}>{q.http_status || "–"}</td>
                      </tr>
                      {open === q.id && (
                        <tr className="expanded">
                          <td colSpan={9}>
                            {q.error && <p className="error small">{q.error}</p>}
                            <pre>{q.response || "(empty response)"}</pre>
                            {q.tool_calls != null && (
                              <pre>{JSON.stringify(q.tool_calls, null, 2)}</pre>
                            )}
                          </td>
                        </tr>
                      )}
                    </Fragment>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        ))}
    </>
  );
}

const phaseTitle: Record<string, string> = {
  cold: "Cold start (model unloaded first)",
  warmup: "Warm-up (not counted)",
  warm: "Warm runs",
  multiturn: "Multi-turn conversation",
  tools: "Tool calling (stream: false)",
};
