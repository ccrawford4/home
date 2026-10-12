import { Fragment, useCallback, useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { api, BenchRequest, Run, RunConfig } from "./api";
import { Icon, Logo, ThemeToggle } from "./chrome";

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
    <>
      <header className="topbar">
        <div className="topbar-inner">
          <a className="brand" href="#" onClick={(e) => (e.preventDefault(), setDetailId(null))}>
            <Logo />
            <span className="brand-name">Virgo Bench</span>
          </a>
          <div className="topbar-right">
            <LiveDot active={active} />
            <ThemeToggle />
          </div>
        </div>
      </header>
      <main className="page">
        {error && <div className="banner error">{error}</div>}
        {detailId != null ? (
          <RunDetail id={detailId} onBack={() => setDetailId(null)} live={active} />
        ) : (
          <>
            <div className="page-head">
              <div>
                <h1>Benchmarks</h1>
                <p className="muted">
                  Latency, throughput, multi-turn and tool-calling benchmarks for models served by
                  virgo.
                </p>
              </div>
            </div>
            <Overview runs={runs} />
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
      </main>
      <footer className="footer">
        <div className="footer-inner">
          <span>Virgo Bench</span>
          <span className="muted">Self-hosted inference on a Raspberry Pi 5 with a Hailo NPU</span>
        </div>
      </footer>
    </>
  );
}

function LiveDot({ active }: { active: boolean }) {
  return (
    <span className={`live ${active ? "on" : ""}`} title={active ? "A run is in progress" : "Idle"}>
      <i />
      {active ? "Running" : "Idle"}
    </span>
  );
}

function bestBy(runs: Run[], get: (r: Run) => number | null | undefined, better: "low" | "high") {
  let best: { run: Run; v: number } | null = null;
  for (const r of runs) {
    const v = get(r);
    if (v == null) continue;
    if (!best || (better === "low" ? v < best.v : v > best.v)) best = { run: r, v };
  }
  return best;
}

function Stat(props: { label: string; value: string; hint?: string }) {
  return (
    <div className="stat">
      <div className="stat-label">{props.label}</div>
      <div className="stat-value">{props.value}</div>
      {props.hint && <div className="stat-hint">{props.hint}</div>}
    </div>
  );
}

function Overview({ runs }: { runs: Run[] }) {
  const done = runs.filter((r) => r.status === "done");
  const pending = runs.filter((r) => r.status === "queued" || r.status === "running").length;
  const ttft = bestBy(done, (r) => r.summary?.warm?.client_ttft_ms?.p50, "low");
  const tps = bestBy(done, (r) => r.summary?.warm?.decode_tps?.p50, "high");
  return (
    <div className="stats">
      <Stat
        label="Runs"
        value={String(runs.length)}
        hint={`${done.length} completed${pending ? ` · ${pending} in queue` : ""}`}
      />
      <Stat label="Models tested" value={String(new Set(runs.map((r) => r.model)).size)} />
      <Stat
        label="Fastest TTFT p50"
        value={fmtMs(ttft?.v)}
        hint={ttft?.run.model ?? "No results yet"}
      />
      <Stat
        label="Top decode speed"
        value={tps ? `${fmtNum(tps.v)} tok/s` : "–"}
        hint={tps?.run.model ?? "No results yet"}
      />
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
      <div className="card-head">
        <div>
          <h2>New run</h2>
          <p className="muted small">Queue a benchmark against a model served by virgo.</p>
        </div>
      </div>
      <form id="new-run" onSubmit={submit} className="form">
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
        <fieldset className="chips">
          <legend>Phases</legend>
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
      </form>
      {modelErr && <p className="banner warn small">Couldn't list models from virgo: {modelErr}</p>}
      {err && <p className="banner error small">{err}</p>}
      <div className="card-foot">
        <p className="muted small">
          Runs execute one at a time because the NPU serves one request at a time. Chats on
          chat.calum.sh during a run will skew its timings.
        </p>
        <button type="submit" form="new-run" className="primary" disabled={busy || !cfg.model}>
          {busy ? "Queuing…" : "Run benchmark"}
        </button>
      </div>
    </section>
  );
}

function StatusBadge({ run }: { run: Run }) {
  return (
    <span className={`badge ${run.status}`} title={run.error}>
      <i />
      {run.status[0].toUpperCase() + run.status.slice(1)}
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
    <section className="card flush">
      <div className="card-head">
        <div>
          <h2>Runs</h2>
          <p className="muted small">
            Tick two or more finished runs to compare them side by side.
          </p>
        </div>
        {runs.length > 0 && <span className="count">{runs.length}</span>}
      </div>
      {runs.length === 0 ? (
        <div className="empty">
          <p>No runs yet</p>
          <p className="muted small">Start one above to see latency and throughput here.</p>
        </div>
      ) : (
        <div className="scroll">
          <table>
            <thead>
              <tr>
                <th title="Select to compare" className="sel" />
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
                  <tr key={r.id} className={selected.includes(r.id) ? "selected" : ""}>
                    <td className="sel">
                      <input
                        type="checkbox"
                        aria-label={`Compare run #${r.id}`}
                        disabled={!s}
                        checked={selected.includes(r.id)}
                        onChange={() => onToggle(r.id)}
                      />
                    </td>
                    <td className="muted">{r.id}</td>
                    <td>
                      <a
                        className="model"
                        href={`#/runs/${r.id}`}
                        onClick={(e) => (e.preventDefault(), onOpen(r.id))}
                      >
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
                    <td className="num">{s?.tools ? `${s.tools.passed}/${s.tools.total}` : "–"}</td>
                    <td className="small muted">{fmtDate(r.created_at)}</td>
                    <td className="actions">
                      {r.status !== "running" && (
                        <button
                          className="icon-btn danger"
                          title={`Delete run #${r.id}`}
                          aria-label={`Delete run #${r.id}`}
                          onClick={() => onDelete(r.id)}
                        >
                          {Icon.trash}
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
    <section className="card flush">
      <div className="card-head">
        <div>
          <h2>Compare</h2>
          <p className="muted small">Best value in each row is highlighted.</p>
        </div>
        <button className="ghost" onClick={onClear}>
          {Icon.x}
          Clear
        </button>
      </div>
      {mismatched && (
        <p className="banner warn small">
          These runs used different max output tokens, so TTLT and multi-turn times aren't directly
          comparable.
        </p>
      )}
      <div className="scroll">
        <table>
          <thead>
            <tr>
              <th>Metric</th>
              {runs.map((r) => (
                <th key={r.id} className="num">
                  <span className="model">{r.model}</span>
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
                  <td className="metric">{row.label}</td>
                  {vals.map((v, i) => (
                    <td key={runs[i].id} className={`num ${v != null && v === best ? "best" : ""}`}>
                      {row.fmt(v)}
                    </td>
                  ))}
                </tr>
              );
            })}
            <tr>
              <td className="metric">History recall</td>
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
  if (!data) return <p className="muted loading">Loading…</p>;
  const { run } = data;
  const s = run.summary;

  return (
    <>
      <nav className="crumbs">
        <a href="#" onClick={(e) => (e.preventDefault(), onBack())}>
          Runs
        </a>
        {Icon.chevron}
        <span>#{run.id}</span>
      </nav>
      <div className="page-head">
        <div>
          <h1 className="detail-title">
            <span className="model">{run.model}</span>
            <StatusBadge run={run} />
          </h1>
          <p className="muted small">
            Run #{run.id} · {run.config.warm_runs} warm runs · max {run.config.num_predict} output
            tokens · started {fmtDate(run.started_at)} · finished {fmtDate(run.finished_at)}
          </p>
        </div>
        <button className="ghost" onClick={onBack}>
          {Icon.back}
          All runs
        </button>
      </div>
      {run.error && <p className="banner error small">{run.error}</p>}
      {s?.cold?.error && <p className="banner warn small">Cold start: {s.cold.error}</p>}
      {s && (
        <div className="stats">
          <Stat
            label="TTFT p50"
            value={fmtMs(s.warm?.client_ttft_ms?.p50)}
            hint={`Server ${fmtMs(s.warm?.server_ttft_ms?.p50)}`}
          />
          <Stat label="TTLT p50" value={fmtMs(s.warm?.client_ttlt_ms?.p50)} />
          <Stat
            label="Decode speed"
            value={s.warm?.decode_tps ? `${fmtNum(s.warm.decode_tps.p50)} tok/s` : "–"}
            hint="p50 across warm runs"
          />
          <Stat
            label="Cold TTFT"
            value={fmtMs(s.cold?.client_ttft_ms)}
            hint={`Load penalty ${fmtMs(s.cold?.load_penalty_ms)}`}
          />
          <Stat
            label="Tool calling"
            value={s.tools ? `${s.tools.passed}/${s.tools.total}` : "–"}
            hint={s.tools ? `${Math.round(s.tools.pass_rate * 100)}% passed` : "Not run"}
          />
        </div>
      )}
      {["cold", "warmup", "warm", "multiturn", "tools"]
        .filter((p) => phases[p])
        .map((p) => (
          <section className="card flush" key={p}>
            <div className="card-head">
              <h3>{phaseTitle[p]}</h3>
              <span className="count">{phases[p].length}</span>
            </div>
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
                      <tr
                        className="clickable"
                        onClick={() => setOpen(open === q.id ? null : q.id)}
                      >
                        <td className="muted">{q.seq}</td>
                        <td className="prompt">{q.prompt}</td>
                        <td className="num">{fmtMs(q.client_ttft_ms)}</td>
                        <td className="num">{fmtMs(q.server_ttft_ms)}</td>
                        <td className="num">{fmtMs(q.client_ttlt_ms)}</td>
                        <td className="num">{q.eval_count ?? "–"}</td>
                        <td className="num">{fmtNum(q.decode_tps)}</td>
                        {(p === "tools" || p === "multiturn") && (
                          <td>
                            {q.passed == null ? (
                              ""
                            ) : (
                              <span className={`pass ${q.passed ? "yes" : "no"}`}>
                                {q.passed ? "Pass" : "Fail"}
                              </span>
                            )}
                          </td>
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
