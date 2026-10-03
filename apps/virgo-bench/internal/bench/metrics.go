package bench

import (
	"math"
	"sort"
	"time"

	"virgo-bench/internal/ollama"
)

// Metrics are the per-request numbers derived from a Result.
type Metrics struct {
	ClientTTFTMs  *float64
	ClientTTLTMs  *float64
	ServerTTFTMs  *float64
	ServerTotalMs *float64
	EvalCount     *int
	DecodeTPS     *float64
}

// Compute derives latency and throughput from a streamed (or non-streamed)
// result.
//
// hailo-ollama only reports total_duration and eval_count, but each chunk
// carries a server created_at timestamp. Server-side TTFT is therefore
//
//	total_duration - (done.created_at - first_token.created_at)
//
// and decode speed is (eval_count-1) / (done.created_at - first.created_at).
// When the server does report prompt/eval durations (stock Ollama), those
// are used instead.
func Compute(r *ollama.Result) Metrics {
	var m Metrics
	first, last := -1, -1
	for i, c := range r.Chunks {
		if first < 0 && (c.Text != "" || c.ToolCall) {
			first = i
		}
		if c.Done {
			last = i
		}
	}
	if last < 0 && len(r.Chunks) > 0 {
		last = len(r.Chunks) - 1
	}
	if first >= 0 {
		m.ClientTTFTMs = ptr(r.Chunks[first].ClientMs)
	}
	if last >= 0 {
		m.ClientTTLTMs = ptr(r.Chunks[last].ClientMs)
	}

	s := r.Stats
	if s.TotalDuration > 0 {
		m.ServerTotalMs = ptr(ms(s.TotalDuration))
	}
	if s.EvalCount > 0 {
		m.EvalCount = &s.EvalCount
	}

	// Server span between first token and the done chunk.
	var span time.Duration
	if first >= 0 && last > first {
		t0, err0 := parseTime(r.Chunks[first].CreatedAt)
		t1, err1 := parseTime(r.Chunks[last].CreatedAt)
		if err0 == nil && err1 == nil && t1.After(t0) {
			span = t1.Sub(t0)
		}
	}

	switch {
	case s.PromptEvalDuration > 0:
		m.ServerTTFTMs = ptr(ms(s.LoadDuration + s.PromptEvalDuration))
	case s.TotalDuration > 0 && span > 0 && span < s.TotalDuration:
		m.ServerTTFTMs = ptr(ms(s.TotalDuration - span))
	}

	switch {
	case s.EvalDuration > 0 && s.EvalCount > 0:
		m.DecodeTPS = ptr(round2(float64(s.EvalCount) / s.EvalDuration.Seconds()))
	case s.EvalCount > 1 && span > 0:
		m.DecodeTPS = ptr(round2(float64(s.EvalCount-1) / span.Seconds()))
	default:
		// Fall back to client-side chunk rate.
		n := 0
		for _, c := range r.Chunks {
			if c.Text != "" {
				n++
			}
		}
		if n > 1 && m.ClientTTFTMs != nil && m.ClientTTLTMs != nil && *m.ClientTTLTMs > *m.ClientTTFTMs {
			m.DecodeTPS = ptr(round2(float64(n-1) / ((*m.ClientTTLTMs - *m.ClientTTFTMs) / 1000)))
		}
	}
	return m
}

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

func ms(d time.Duration) float64 { return round2(float64(d.Microseconds()) / 1000) }

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func ptr[T any](v T) *T { return &v }

// Agg summarizes a set of samples.
type Agg struct {
	N    int     `json:"n"`
	Mean float64 `json:"mean"`
	P50  float64 `json:"p50"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
}

func aggregate(vals []float64) *Agg {
	if len(vals) == 0 {
		return nil
	}
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	sum := 0.0
	for _, v := range s {
		sum += v
	}
	var p50 float64
	if n := len(s); n%2 == 1 {
		p50 = s[n/2]
	} else {
		p50 = (s[n/2-1] + s[n/2]) / 2
	}
	return &Agg{N: len(s), Mean: round2(sum / float64(len(s))), P50: round2(p50), Min: round2(s[0]), Max: round2(s[len(s)-1])}
}
