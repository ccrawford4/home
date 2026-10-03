package bench

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"virgo-bench/internal/ollama"
	"virgo-bench/internal/store"
)

// fakeVirgo mimics hailo-ollama 0.5.1: per-chunk created_at, only
// total_duration and eval_count in the final chunk, tool calls parsed only
// when stream=false.
func fakeVirgo(t *testing.T) (*httptest.Server, *atomic.Int32) {
	var unloads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			io.WriteString(w, `{"models":[{"name":"qwen2.5-coder:1.5b"}]}`)
			return
		}
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if r.URL.Path == "/api/generate" {
			unloads.Add(1)
			io.WriteString(w, `{"done":true,"done_reason":"unload"}`)
			return
		}
		msgs := req["messages"].([]any)
		last := msgs[len(msgs)-1].(map[string]any)["content"].(string)
		start := time.Now()
		prefill := 20 * time.Millisecond
		time.Sleep(prefill)
		if req["stream"] == false {
			resp := map[string]any{"done": true, "total_duration": time.Since(start).Nanoseconds(), "eval_count": 12,
				"created_at": time.Now().UTC().Format(time.RFC3339Nano),
				"message":    map[string]any{"role": "assistant", "content": ""}}
			if strings.Contains(last, "weather") {
				resp["message"].(map[string]any)["tool_calls"] = []any{map[string]any{"function": map[string]any{
					"name": "get_weather", "arguments": map[string]any{"city": "San Francisco"}}}}
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		fl := w.(http.Flusher)
		reply := "Calum is your name"
		toks := strings.Fields(reply)
		for _, tok := range toks {
			json.NewEncoder(w).Encode(map[string]any{"created_at": time.Now().UTC().Format(time.RFC3339Nano),
				"message": map[string]any{"role": "assistant", "content": tok + " "}, "done": false})
			fl.Flush()
			time.Sleep(10 * time.Millisecond)
		}
		json.NewEncoder(w).Encode(map[string]any{"created_at": time.Now().UTC().Format(time.RFC3339Nano),
			"message": map[string]any{"role": "assistant", "content": ""}, "done": true,
			"total_duration": time.Since(start).Nanoseconds(), "eval_count": len(toks)})
	}))
	t.Cleanup(srv.Close)
	return srv, &unloads
}

func TestRunnerEndToEnd(t *testing.T) {
	srv, unloads := fakeVirgo(t)
	st := store.NewMemory()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRunner(st, ollama.New(srv.URL, time.Minute), log)

	cfg := Config{Model: "qwen2.5-coder:1.5b", WarmRuns: 3, Cold: true, MultiTurn: true, Tools: true}
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(cfg)
	run := &store.Run{Model: cfg.Model, Status: store.StatusQueued, Config: raw, CreatedAt: time.Now()}
	st.CreateRun(context.Background(), run)
	r.execute(context.Background(), run.ID)

	got, _ := st.GetRun(context.Background(), run.ID)
	if got.Status != store.StatusDone {
		t.Fatalf("status = %s, error = %s", got.Status, got.Error)
	}
	if unloads.Load() != 1 {
		t.Errorf("unloads = %d, want 1", unloads.Load())
	}
	var sum Summary
	if err := json.Unmarshal(got.Summary, &sum); err != nil {
		t.Fatal(err)
	}
	if sum.Warm == nil || sum.Warm.ClientTTFT == nil || sum.Warm.ClientTTFT.N != 3 {
		t.Fatalf("warm summary = %+v", sum.Warm)
	}
	// Server TTFT is derived from created_at and should be ~prefill (20ms),
	// well below client TTLT.
	if sum.Warm.ServerTTFT == nil || sum.Warm.ServerTTFT.P50 < 15 || sum.Warm.ServerTTFT.P50 > sum.Warm.ClientTTLT.P50 {
		t.Errorf("server ttft = %+v, client ttlt = %+v", sum.Warm.ServerTTFT, sum.Warm.ClientTTLT)
	}
	if sum.Warm.DecodeTPS == nil || sum.Warm.DecodeTPS.P50 <= 0 {
		t.Errorf("decode tps = %+v", sum.Warm.DecodeTPS)
	}
	if sum.Cold == nil || sum.Cold.LoadPenaltyMs == nil {
		t.Errorf("cold summary = %+v", sum.Cold)
	}
	if sum.MultiTurn == nil || !sum.MultiTurn.Completed || !sum.MultiTurn.RecallOK || len(sum.MultiTurn.Turns) != 4 {
		t.Errorf("multi-turn summary = %+v", sum.MultiTurn)
	}
	if sum.Tools == nil || sum.Tools.Passed != 1 || sum.Tools.Total != len(ToolCases) {
		t.Errorf("tools summary = %+v", sum.Tools)
	}
	reqs, _ := st.ListRequests(context.Background(), run.ID)
	if want := 1 + 3 + 4 + len(ToolCases); len(reqs) != want {
		t.Errorf("requests = %d, want %d", len(reqs), want)
	}
}

func TestComputeHailoStats(t *testing.T) {
	base := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	at := func(ms int) string { return base.Add(time.Duration(ms) * time.Millisecond).Format(time.RFC3339Nano) }
	r := &ollama.Result{
		Chunks: []ollama.Chunk{
			{ClientMs: 720, CreatedAt: at(0), Text: "a"},
			{ClientMs: 845, CreatedAt: at(124), Text: "b"},
			{ClientMs: 970, CreatedAt: at(248), Text: "c"},
			{ClientMs: 980, CreatedAt: at(250), Done: true},
		},
		Stats: ollama.Stats{TotalDuration: 570 * time.Millisecond, EvalCount: 3},
	}
	m := Compute(r)
	if *m.ClientTTFTMs != 720 || *m.ClientTTLTMs != 980 {
		t.Errorf("client ttft/ttlt = %v/%v", *m.ClientTTFTMs, *m.ClientTTLTMs)
	}
	if *m.ServerTTFTMs != 320 {
		t.Errorf("server ttft = %v, want 320", *m.ServerTTFTMs)
	}
	if *m.DecodeTPS != 8 {
		t.Errorf("decode tps = %v, want 8", *m.DecodeTPS)
	}
}

func TestCheckToolCallStringArgs(t *testing.T) {
	var calls []ollama.ToolCall
	json.Unmarshal([]byte(`[{"function":{"name":"get_time","arguments":"{\"timezone\":\"Asia/Tokyo\"}"}}]`), &calls)
	if !CheckToolCall(calls, ToolCases[2]) {
		t.Error("string-encoded arguments should pass")
	}
	if CheckToolCall(calls, ToolCases[0]) {
		t.Error("wrong tool should fail")
	}
}
