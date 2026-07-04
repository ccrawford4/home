// Package api provides the HTTP handlers for the sandbox API.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"sandbox-api/sandbox"
)

// Handler holds the dependencies for the HTTP handlers.
type Handler struct {
	log     *slog.Logger
	Manager *sandbox.Manager
}

// NewHandler returns a new Handler.
func NewHandler(log *slog.Logger, m *sandbox.Manager) *Handler {
	return &Handler{log: log, Manager: m}
}

// RegisterRoutes registers the sandbox API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("POST /sandbox/create", h.instrument("CreateSandbox", h.CreateSandbox))
	mux.Handle("GET /sandbox/list", h.instrument("ListSandboxes", h.ListSandboxes))
	mux.Handle("GET /sandbox/{id}", h.instrument("GetSandbox", h.GetSandbox))
	mux.Handle("DELETE /sandbox/{id}", h.instrument("DeleteSandbox", h.DeleteSandbox))
}

func (h *Handler) instrument(operation string, next http.HandlerFunc) http.Handler {
	return otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		ctx := r.Context()
		log := h.log.With(
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("operation", operation),
			slog.String("trace_id", traceID(ctx)),
			slog.String("span_id", spanID(ctx)),
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
		)

		log.Info("request started")
		next(ww, r.WithContext(ctx))
		log.Info("request completed",
			slog.Int("status", ww.statusCode),
			slog.Duration("duration", time.Since(start)),
		)
	}), operation)
}

// CreateSandbox handles POST /sandbox/create.
func (h *Handler) CreateSandbox(w http.ResponseWriter, r *http.Request) {
	sb, err := h.Manager.Create(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sb)
}

// GetSandbox handles GET /sandbox/{id}.
func (h *Handler) GetSandbox(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sb, err := h.Manager.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sb)
}

// DeleteSandbox handles DELETE /sandbox/{id}.
func (h *Handler) DeleteSandbox(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Manager.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListSandboxes handles GET /sandbox/list.
func (h *Handler) ListSandboxes(w http.ResponseWriter, r *http.Request) {
	list, err := h.Manager.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
