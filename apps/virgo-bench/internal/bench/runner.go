// Package bench runs benchmark suites against an Ollama-compatible server
// and records the results.
package bench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"virgo-bench/internal/ollama"
	"virgo-bench/internal/store"
)

// Summary is the per-run rollup shown in the UI and stored on the run.
type Summary struct {
	Cold      *ColdSummary      `json:"cold,omitempty"`
	Warm      *WarmSummary      `json:"warm,omitempty"`
	MultiTurn *MultiTurnSummary `json:"multi_turn,omitempty"`
	Tools     *ToolsSummary     `json:"tools,omitempty"`
}

type ColdSummary struct {
	ClientTTFTMs *float64 `json:"client_ttft_ms"`
	ServerTTFTMs *float64 `json:"server_ttft_ms"`
	ClientTTLTMs *float64 `json:"client_ttlt_ms"`
	// LoadPenaltyMs is cold client TTFT minus warm median client TTFT: the
	// cost of loading the model onto the NPU, which hailo-ollama does not
	// include in total_duration.
	LoadPenaltyMs *float64 `json:"load_penalty_ms"`
	Error         string   `json:"error,omitempty"`
}

type WarmSummary struct {
	ClientTTFT *Agg `json:"client_ttft_ms"`
	ServerTTFT *Agg `json:"server_ttft_ms"`
	ClientTTLT *Agg `json:"client_ttlt_ms"`
	DecodeTPS  *Agg `json:"decode_tps"`
	EvalCount  *Agg `json:"eval_count"`
	Errors     int  `json:"errors"`
}

type TurnSummary struct {
	Turn         int      `json:"turn"`
	ClientTTFTMs *float64 `json:"client_ttft_ms"`
	ServerTTFTMs *float64 `json:"server_ttft_ms"`
	ClientTTLTMs *float64 `json:"client_ttlt_ms"`
	Error        string   `json:"error,omitempty"`
}

type MultiTurnSummary struct {
	Turns     []TurnSummary `json:"turns"`
	TotalMs   float64       `json:"total_ms"`
	RecallOK  bool          `json:"recall_ok"`
	Completed bool          `json:"completed"`
}

type ToolsSummary struct {
	Passed   int     `json:"passed"`
	Total    int     `json:"total"`
	PassRate float64 `json:"pass_rate"`
	Latency  *Agg    `json:"latency_ms"`
}

// Runner executes queued runs one at a time. The Hailo NPU serializes
// requests, so running suites concurrently would only corrupt the timings.
type Runner struct {
	store  store.Store
	client *ollama.Client
	log    *slog.Logger
	queue  chan int64
}

// NewRunner returns a runner; call Start to begin processing.
func NewRunner(s store.Store, c *ollama.Client, log *slog.Logger) *Runner {
	return &Runner{store: s, client: c, log: log, queue: make(chan int64, 1000)}
}

// Enqueue schedules a run that is already stored with status queued.
func (r *Runner) Enqueue(id int64) error {
	select {
	case r.queue <- id:
		return nil
	default:
		return errors.New("queue is full")
	}
}

// Start recovers runs left over from a previous process and processes the
// queue until ctx is cancelled.
func (r *Runner) Start(ctx context.Context) {
	if stale, err := r.store.RunsWithStatus(ctx, store.StatusRunning); err == nil {
		for _, run := range stale {
			now := time.Now().UTC()
			run.Status, run.Error, run.FinishedAt = store.StatusFailed, "interrupted by a restart", &now
			if err := r.store.UpdateRun(ctx, &run); err != nil {
				r.log.Error("mark interrupted run", "run", run.ID, "error", err)
			}
		}
	}
	if queued, err := r.store.RunsWithStatus(ctx, store.StatusQueued); err == nil {
		for _, run := range queued {
			_ = r.Enqueue(run.ID)
		}
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case id := <-r.queue:
				r.execute(ctx, id)
			}
		}
	}()
}

func (r *Runner) execute(ctx context.Context, id int64) {
	log := r.log.With("run", id)
	run, err := r.store.GetRun(ctx, id)
	if err != nil {
		log.Warn("skipping run", "error", err) // deleted while queued
		return
	}
	if run.Status != store.StatusQueued {
		return
	}
	var cfg Config
	if err := json.Unmarshal(run.Config, &cfg); err != nil {
		r.finish(ctx, run, nil, fmt.Errorf("bad config: %w", err))
		return
	}
	now := time.Now().UTC()
	run.Status, run.StartedAt = store.StatusRunning, &now
	r.update(ctx, run)
	log.Info("run started", "model", cfg.Model)

	sum, err := r.suite(ctx, run, cfg)
	r.finish(ctx, run, sum, err)
	log.Info("run finished", "status", run.Status, "error", run.Error)
}

func (r *Runner) finish(ctx context.Context, run *store.Run, sum *Summary, err error) {
	now := time.Now().UTC()
	run.FinishedAt = &now
	run.Progress = ""
	run.Status = store.StatusDone
	if sum != nil {
		run.Summary, _ = json.Marshal(sum)
	}
	if err != nil {
		run.Status, run.Error = store.StatusFailed, err.Error()
	}
	r.update(ctx, run)
}

func (r *Runner) update(ctx context.Context, run *store.Run) {
	if err := r.store.UpdateRun(ctx, run); err != nil {
		r.log.Error("update run", "run", run.ID, "error", err)
	}
}

func (r *Runner) progress(ctx context.Context, run *store.Run, format string, args ...any) {
	run.Progress = fmt.Sprintf(format, args...)
	r.update(ctx, run)
}

// call performs one chat request and records it.
func (r *Runner) call(ctx context.Context, run *store.Run, phase string, seq int, req ollama.ChatRequest) (*ollama.Result, Metrics, error) {
	res := r.client.Chat(ctx, req)
	m := Compute(res)
	q := &store.Request{
		RunID: run.ID, Phase: phase, Seq: seq,
		Prompt:     req.Messages[len(req.Messages)-1].Content,
		Response:   res.Text,
		HTTPStatus: res.HTTPStatus, Error: res.Error,
		ClientTTFTMs: m.ClientTTFTMs, ClientTTLTMs: m.ClientTTLTMs,
		ServerTTFTMs: m.ServerTTFTMs, ServerTotalMs: m.ServerTotalMs,
		EvalCount: m.EvalCount, DecodeTPS: m.DecodeTPS,
		StartedAt: res.StartedAt.UTC(),
	}
	if !req.Stream {
		// A non-streamed reply has one chunk; its arrival is the end-to-end time.
		q.ClientTTLTMs = ptr(round2(res.ClientTotalMs))
	}
	if len(res.ToolCalls) > 0 {
		q.ToolCalls, _ = json.Marshal(res.ToolCalls)
	}
	if len(res.Chunks) > 0 {
		q.Chunks, _ = json.Marshal(res.Chunks)
	}
	if q.StartedAt.IsZero() {
		q.StartedAt = time.Now().UTC()
	}
	if phase == "tools" {
		q.Passed = ptr(CheckToolCall(res.ToolCalls, ToolCases[seq-1]))
	}
	if phase == "multiturn" && seq == len(MultiTurnPrompts) {
		q.Passed = ptr(strings.Contains(strings.ToLower(res.Text), MultiTurnRecall))
	}
	if err := r.store.AddRequest(ctx, q); err != nil {
		return res, m, fmt.Errorf("save request: %w", err)
	}
	return res, m, nil
}

func (r *Runner) suite(ctx context.Context, run *store.Run, cfg Config) (*Summary, error) {
	sum := &Summary{}
	opts := map[string]any{"num_predict": cfg.NumPredict}
	single := func() ollama.ChatRequest {
		return ollama.ChatRequest{Model: cfg.Model, Stream: true, Options: opts,
			Messages: []ollama.Message{{Role: "user", Content: WarmPrompt}}}
	}

	// 1. Cold start: evict the model, then time the first request.
	if cfg.Cold {
		r.progress(ctx, run, "cold start")
		cold := &ColdSummary{}
		if err := r.client.Unload(ctx, cfg.Model); err != nil {
			cold.Error = "unload failed: " + err.Error()
		}
		res, m, err := r.call(ctx, run, "cold", 1, single())
		if err != nil {
			return sum, err
		}
		cold.ClientTTFTMs, cold.ServerTTFTMs, cold.ClientTTLTMs = m.ClientTTFTMs, m.ServerTTFTMs, m.ClientTTLTMs
		if res.Error != "" && cold.Error == "" {
			cold.Error = res.Error
		}
		sum.Cold = cold
	} else {
		// Make sure the model is loaded so warm runs measure warm latency.
		r.progress(ctx, run, "warming up")
		if _, _, err := r.call(ctx, run, "warmup", 1, single()); err != nil {
			return sum, err
		}
	}

	// 2. Warm runs.
	warm := &WarmSummary{}
	var ttft, sttft, ttlt, tps, evals []float64
	var lastErr string
	for i := 1; i <= cfg.WarmRuns; i++ {
		if err := ctx.Err(); err != nil {
			return sum, err
		}
		r.progress(ctx, run, "warm %d/%d", i, cfg.WarmRuns)
		res, m, err := r.call(ctx, run, "warm", i, single())
		if err != nil {
			return sum, err
		}
		if res.Error != "" {
			warm.Errors++
			lastErr = res.Error
			continue
		}
		appendIf(&ttft, m.ClientTTFTMs)
		appendIf(&sttft, m.ServerTTFTMs)
		appendIf(&ttlt, m.ClientTTLTMs)
		appendIf(&tps, m.DecodeTPS)
		if m.EvalCount != nil {
			evals = append(evals, float64(*m.EvalCount))
		}
	}
	warm.ClientTTFT, warm.ServerTTFT, warm.ClientTTLT = aggregate(ttft), aggregate(sttft), aggregate(ttlt)
	warm.DecodeTPS, warm.EvalCount = aggregate(tps), aggregate(evals)
	sum.Warm = warm
	if warm.Errors == cfg.WarmRuns {
		return sum, fmt.Errorf("every warm request failed, last error: %s", lastErr)
	}
	if sum.Cold != nil && sum.Cold.ClientTTFTMs != nil && warm.ClientTTFT != nil {
		sum.Cold.LoadPenaltyMs = ptr(round2(*sum.Cold.ClientTTFTMs - warm.ClientTTFT.P50))
	}

	// 3. Multi-turn conversation, resending the growing history each turn.
	if cfg.MultiTurn {
		mt := &MultiTurnSummary{}
		var history []ollama.Message
		for i, p := range MultiTurnPrompts {
			r.progress(ctx, run, "multi-turn %d/%d", i+1, len(MultiTurnPrompts))
			history = append(history, ollama.Message{Role: "user", Content: p})
			req := ollama.ChatRequest{Model: cfg.Model, Stream: true, Options: opts,
				Messages: append([]ollama.Message(nil), history...)}
			res, m, err := r.call(ctx, run, "multiturn", i+1, req)
			if err != nil {
				return sum, err
			}
			mt.Turns = append(mt.Turns, TurnSummary{Turn: i + 1, ClientTTFTMs: m.ClientTTFTMs,
				ServerTTFTMs: m.ServerTTFTMs, ClientTTLTMs: m.ClientTTLTMs, Error: res.Error})
			mt.TotalMs = round2(mt.TotalMs + res.ClientTotalMs)
			if res.Error != "" {
				break
			}
			history = append(history, ollama.Message{Role: "assistant", Content: res.Text})
			if i == len(MultiTurnPrompts)-1 {
				mt.Completed = true
				mt.RecallOK = strings.Contains(strings.ToLower(res.Text), MultiTurnRecall)
			}
		}
		sum.MultiTurn = mt
	}

	// 4. Tool calling. hailo-ollama only parses tool calls when stream=false.
	if cfg.Tools {
		ts := &ToolsSummary{Total: len(ToolCases)}
		var lat []float64
		for i, tc := range ToolCases {
			r.progress(ctx, run, "tools %d/%d", i+1, len(ToolCases))
			req := ollama.ChatRequest{Model: cfg.Model, Stream: false, Tools: Tools,
				Options:  map[string]any{"num_predict": toolNumPredict},
				Messages: []ollama.Message{{Role: "user", Content: tc.Prompt}}}
			res, _, err := r.call(ctx, run, "tools", i+1, req)
			if err != nil {
				return sum, err
			}
			if res.Error == "" {
				lat = append(lat, round2(res.ClientTotalMs))
			}
			if CheckToolCall(res.ToolCalls, tc) {
				ts.Passed++
			}
		}
		ts.PassRate = round2(float64(ts.Passed) / float64(ts.Total))
		ts.Latency = aggregate(lat)
		sum.Tools = ts
	}
	return sum, nil
}

func appendIf(dst *[]float64, v *float64) {
	if v != nil {
		*dst = append(*dst, *v)
	}
}
