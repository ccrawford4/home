package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sandbox-api/gpupool"
)

type fakePool struct{ model, reason string }

func (f *fakePool) Demand(model, reason string) gpupool.Status {
	f.model, f.reason = model, reason
	return gpupool.Status{TargetSize: 1, MaxSize: 1}
}

func (f *fakePool) Status() gpupool.Status { return gpupool.Status{MaxSize: 1} }

func newGPUPoolMux(pool GPUPool) *http.ServeMux {
	mux := http.NewServeMux()
	NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), nil).RegisterGPUPoolRoutes(mux, pool)
	return mux
}

func TestGPUPoolDemand(t *testing.T) {
	pool := &fakePool{}
	mux := newGPUPoolMux(pool)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"model":"qwen2.5-coder:7b","reason":"no gpu backend"}`)
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/gpu-pool/demand", body))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body)
	}
	if pool.model != "qwen2.5-coder:7b" || pool.reason != "no gpu backend" {
		t.Fatalf("Demand got (%q, %q)", pool.model, pool.reason)
	}
	var st gpupool.Status
	if err := json.NewDecoder(rec.Body).Decode(&st); err != nil || st.TargetSize != 1 {
		t.Fatalf("body = %+v, err %v, want target_size 1", st, err)
	}
}

func TestGPUPoolDemandWithoutBody(t *testing.T) {
	mux := newGPUPoolMux(&fakePool{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/gpu-pool/demand", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
}

func TestGPUPoolDemandRejectsBadJSON(t *testing.T) {
	mux := newGPUPoolMux(&fakePool{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/gpu-pool/demand", strings.NewReader("{")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGPUPoolDisabled(t *testing.T) {
	mux := newGPUPoolMux(nil)
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/gpu-pool/demand", nil),
		httptest.NewRequest(http.MethodGet, "/gpu-pool/status", nil),
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s = %d, want 503", req.Method, req.URL.Path, rec.Code)
		}
	}
}
