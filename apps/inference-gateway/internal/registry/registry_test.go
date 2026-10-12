package registry

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"inference-gateway/internal/config"
)

func TestHealthChecks(t *testing.T) {
	var up atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" || !up.Load() {
			w.WriteHeader(500)
			return
		}
		w.Write([]byte(`{"version":"x"}`))
	}))
	defer srv.Close()
	cfg := &config.Config{QueueDepth: 1, Backends: []config.Backend{{Name: "hailo", URL: srv.URL}}}
	r := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	b, _ := r.Get("hailo")
	if !b.Healthy() {
		t.Fatal("static backends start healthy")
	}
	for _, step := range []struct {
		up, want bool
	}{{false, false}, {true, true}, {false, false}} {
		up.Store(step.up)
		r.CheckHealth(t.Context(), srv.Client(), 2e9)
		if b.Healthy() != step.want {
			t.Fatalf("up=%v: healthy=%v", step.up, b.Healthy())
		}
	}
}
