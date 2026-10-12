// Package api provides the HTTP handlers for virgo-bench.
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"virgo-bench/internal/bench"
	"virgo-bench/internal/ollama"
	"virgo-bench/internal/store"
)

// Handler holds the dependencies for the HTTP handlers.
type Handler struct {
	log    *slog.Logger
	store  store.Store
	runner *bench.Runner
	client *ollama.Client
	ui     fs.FS
}

// NewHandler returns a new Handler. ui is the built frontend (may be empty).
func NewHandler(log *slog.Logger, s store.Store, r *bench.Runner, c *ollama.Client, ui fs.FS) *Handler {
	return &Handler{log: log, store: s, runner: r, client: c, ui: ui}
}

// RegisterRoutes registers the API and UI routes on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.Handle("GET /api/models", h.logged(h.ListModels))
	mux.Handle("GET /api/runs", h.logged(h.ListRuns))
	mux.Handle("POST /api/runs", h.logged(h.CreateRun))
	mux.Handle("GET /api/runs/{id}", h.logged(h.GetRun))
	mux.Handle("DELETE /api/runs/{id}", h.logged(h.DeleteRun))
	mux.Handle("/", h.spa())
}

func (h *Handler) logged(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next(ww, r)
		h.log.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.statusCode),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

// ListModels handles GET /api/models by proxying virgo's /api/tags.
func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.client.Tags(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

// ListRuns handles GET /api/runs.
func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	runs, err := h.store.ListRuns(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

// CreateRun handles POST /api/runs: stores a queued run and enqueues it.
func (h *Handler) CreateRun(w http.ResponseWriter, r *http.Request) {
	var cfg bench.Config
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := cfg.Normalize(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	raw, _ := json.Marshal(cfg)
	run := &store.Run{Model: cfg.Model, Status: store.StatusQueued, Config: raw, CreatedAt: time.Now().UTC()}
	if err := h.store.CreateRun(r.Context(), run); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := h.runner.Enqueue(run.ID); err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

// GetRun handles GET /api/runs/{id}; ?chunks=0 omits raw chunk timestamps.
func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	run, err := h.store.GetRun(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	reqs, err := h.store.ListRequests(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if r.URL.Query().Get("chunks") == "0" {
		for i := range reqs {
			reqs[i].Chunks = nil
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "requests": reqs})
}

// DeleteRun handles DELETE /api/runs/{id}. A running run can't be deleted.
func (h *Handler) DeleteRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	run, err := h.store.GetRun(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if run.Status == store.StatusRunning {
		writeError(w, http.StatusConflict, errors.New("run is in progress"))
		return
	}
	if err := h.store.DeleteRun(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// spa serves the built frontend, falling back to index.html for client routes.
func (h *Handler) spa() http.Handler {
	files := http.FileServer(http.FS(h.ui))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			if _, err := fs.Stat(h.ui, p); err == nil {
				files.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(h.ui, "index.html")
		if err != nil {
			http.Error(w, "UI not built; run `npm run build` in web/", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeError(w, http.StatusInternalServerError, err)
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
