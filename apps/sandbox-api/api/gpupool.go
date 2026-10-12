package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"sandbox-api/gpupool"
)

// GPUPool is the part of the GPU pool scaler the API exposes.
type GPUPool interface {
	Demand(model, reason string) gpupool.Status
	Status() gpupool.Status
}

// demandRequest is the optional body of POST /gpu-pool/demand.
type demandRequest struct {
	Model  string `json:"model"`
	Reason string `json:"reason"`
}

// RegisterGPUPoolRoutes registers the GPU pool routes. A nil pool means the
// scaler is disabled, and both routes answer 503.
func (h *Handler) RegisterGPUPoolRoutes(mux *http.ServeMux, pool GPUPool) {
	mux.Handle("POST /gpu-pool/demand", h.instrument("GPUPoolDemand", gpuPoolDemand(pool)))
	mux.Handle("GET /gpu-pool/status", h.instrument("GPUPoolStatus", gpuPoolStatus(pool)))
}

// gpuPoolDemand handles POST /gpu-pool/demand. The inference gateway calls it
// when a request needs a GPU backend, and the scaler counts every call as GPU
// activity. It returns at once with 202; it does not wait for a node.
func gpuPoolDemand(pool GPUPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if pool == nil {
			http.Error(w, "gpu pool scaler is disabled", http.StatusServiceUnavailable)
			return
		}
		var req demandRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusAccepted, pool.Demand(req.Model, req.Reason))
	}
}

// gpuPoolStatus handles GET /gpu-pool/status.
func gpuPoolStatus(pool GPUPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if pool == nil {
			http.Error(w, "gpu pool scaler is disabled", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, http.StatusOK, pool.Status())
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
