// Package api provides the HTTP handlers for inference-gateway.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"inference-gateway/internal/auth"
	"inference-gateway/internal/demand"
	"inference-gateway/internal/registry"
	"inference-gateway/internal/router"
)

// PrivateHeader forces a request to on-prem backends when set to "true".
const PrivateHeader = "X-Inference-Private"

// Handler holds the dependencies for the HTTP handlers.
type Handler struct {
	log      *slog.Logger
	router   *router.Router
	reg      *registry.Registry
	keys     *auth.Keys
	adminKey string
	upstream *http.Client
	catalog  []string
	demand   *demand.Signaller
	// TagsTimeout bounds each backend's /api/tags call when merging.
	TagsTimeout time.Duration
}

// NewHandler returns a new Handler. catalog is the ordered list of catalog
// model names advertised by /api/tags. An empty adminKey disables the
// /internal registration endpoints. dem may be nil (demand signalling off).
func NewHandler(log *slog.Logger, rt *router.Router, reg *registry.Registry, keys *auth.Keys, adminKey string, upstream *http.Client, catalog []string, dem *demand.Signaller) *Handler {
	return &Handler{
		log:         log,
		router:      rt,
		reg:         reg,
		keys:        keys,
		adminKey:    adminKey,
		upstream:    upstream,
		catalog:     catalog,
		demand:      dem,
		TagsTimeout: 5 * time.Second,
	}
}

// RegisterRoutes registers all routes on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	mux.Handle("POST /api/chat", h.logged(h.authed(h.Proxy)))
	mux.Handle("POST /api/generate", h.logged(h.authed(h.Proxy)))
	mux.Handle("POST /api/show", h.logged(h.authed(h.Proxy)))
	mux.Handle("POST /v1/chat/completions", h.logged(h.authed(h.Proxy)))
	mux.Handle("GET /api/tags", h.logged(h.authed(h.Tags)))
	mux.Handle("GET /api/version", h.logged(h.authed(h.Version)))
	notFound := func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	}
	mux.Handle("/api/", h.logged(h.authed(notFound)))
	mux.Handle("/v1/", h.logged(h.authed(notFound)))

	mux.Handle("POST /internal/backends", h.logged(h.admin(h.RegisterBackend)))
	mux.Handle("DELETE /internal/backends/{name}", h.logged(h.admin(h.RemoveBackend)))
}

// reqInfo collects routing details for the access log.
type reqInfo struct {
	mu      sync.Mutex
	client  string
	model   string
	backend string
}

func (i *reqInfo) set(f func(*reqInfo)) {
	i.mu.Lock()
	f(i)
	i.mu.Unlock()
}

type ctxKey int

const (
	clientKey ctxKey = iota
	infoKey
)

func clientFrom(ctx context.Context) auth.Client {
	c, _ := ctx.Value(clientKey).(auth.Client)
	return c
}

func infoFrom(ctx context.Context) *reqInfo {
	if i, ok := ctx.Value(infoKey).(*reqInfo); ok {
		return i
	}
	return &reqInfo{}
}

func (h *Handler) logged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		info := &reqInfo{}
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(ww, r.WithContext(context.WithValue(r.Context(), infoKey, info)))
		info.mu.Lock()
		defer info.mu.Unlock()
		attrs := []any{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.statusCode),
			slog.Duration("duration", time.Since(start)),
		}
		if info.client != "" {
			attrs = append(attrs, slog.String("client", info.client))
		}
		if info.model != "" {
			attrs = append(attrs, slog.String("model", info.model))
		}
		if info.backend != "" {
			attrs = append(attrs, slog.String("backend", info.backend))
		}
		h.log.Info("request", attrs...)
	})
}

func (h *Handler) authed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, ok := h.keys.Lookup(auth.BearerToken(r))
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="inference-gateway"`)
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		infoFrom(r.Context()).set(func(i *reqInfo) { i.client = c.Name })
		next(w, r.WithContext(context.WithValue(r.Context(), clientKey, c)))
	}
}

func (h *Handler) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.MatchAdmin(h.adminKey, auth.BearerToken(r)) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

// isPrivate applies the client's setting and the per-request header. The
// header can only make a request more private, never less.
func isPrivate(c auth.Client, r *http.Request) bool {
	if c.Private {
		return true
	}
	v := strings.TrimSpace(r.Header.Get(PrivateHeader))
	return strings.EqualFold(v, "true") || v == "1"
}

// Version handles GET /api/version.
func (h *Handler) Version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": "inference-gateway"})
}

// Tags handles GET /api/tags: catalog model names, then "<backend>/<model>"
// for every model each healthy backend (that the caller may use) reports.
func (h *Handler) Tags(w http.ResponseWriter, r *http.Request) {
	private := isPrivate(clientFrom(r.Context()), r)
	models := make([]map[string]any, 0, len(h.catalog))
	for _, m := range h.catalog {
		models = append(models, map[string]any{"name": m, "model": m})
	}

	var backends []*registry.Backend
	for _, b := range h.reg.All() {
		if b.Healthy() && (!private || b.OnPrem) {
			backends = append(backends, b)
		}
	}
	results := make([][]map[string]any, len(backends))
	var wg sync.WaitGroup
	for i, b := range backends {
		wg.Add(1)
		go func(i int, b *registry.Backend) {
			defer wg.Done()
			ms, err := h.backendTags(r.Context(), b)
			if err != nil {
				h.log.Warn("backend tags failed", "backend", b.Name, "error", err)
				return
			}
			results[i] = ms
		}(i, b)
	}
	wg.Wait()
	for _, ms := range results {
		models = append(models, ms...)
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (h *Handler) backendTags(ctx context.Context, b *registry.Backend) ([]map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, h.TagsTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.upstream.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	var body struct {
		Models []map[string]any `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(body.Models))
	for _, m := range body.Models {
		name, _ := m["name"].(string)
		if name == "" {
			name, _ = m["model"].(string)
		}
		if name == "" {
			continue
		}
		m["name"] = b.Name + "/" + name
		m["model"] = b.Name + "/" + name
		out = append(out, m)
	}
	return out, nil
}

// RegisterBackend handles POST /internal/backends.
func (h *Handler) RegisterBackend(w http.ResponseWriter, r *http.Request) {
	var reg registry.Registration
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&reg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := h.reg.Register(reg)
	if errors.Is(err, registry.ErrStatic) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":        b.Name,
		"url":         b.URL,
		"onPrem":      b.OnPrem,
		"concurrency": b.Concurrency,
	})
}

// RemoveBackend handles DELETE /internal/backends/{name}.
func (h *Handler) RemoveBackend(w http.ResponseWriter, r *http.Request) {
	ok, err := h.reg.Remove(r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "backend not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (w *responseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.statusCode = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
