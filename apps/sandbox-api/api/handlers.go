// Package api provides the HTTP handlers for the sandbox API.
package api

import (
	"encoding/json"
	"net/http"

	"sandbox-api/sandbox"
)

// Handler holds the dependencies for the HTTP handlers.
type Handler struct {
	Manager *sandbox.Manager
}

// NewHandler returns a new Handler.
func NewHandler(m *sandbox.Manager) *Handler {
	return &Handler{Manager: m}
}

// RegisterRoutes registers the sandbox API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /sandbox/create", h.CreateSandbox)
	mux.HandleFunc("GET /sandbox/list", h.ListSandboxes)
	mux.HandleFunc("GET /sandbox/{id}", h.GetSandbox)
	mux.HandleFunc("DELETE /sandbox/{id}", h.DeleteSandbox)
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
