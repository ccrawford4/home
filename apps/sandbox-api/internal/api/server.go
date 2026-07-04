package api

import (
	"encoding/json"
	"net/http"

	"sandbox-api/internal/sandbox"
)

// Server holds the application dependencies.
type Server struct {
	manager *sandbox.Manager
}

// NewServer constructs a Server with all dependencies wired up.
func NewServer() *Server {
	return &Server{manager: sandbox.NewManager()}
}

// Routes returns the HTTP mux with all routes registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /sandbox/create", s.handleCreate)
	mux.HandleFunc("GET /sandbox/list", s.handleList)
	mux.HandleFunc("GET /sandbox/{id}", s.handleGet)
	mux.HandleFunc("DELETE /sandbox/{id}", s.handleDelete)
	return mux
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	sb := s.manager.Create()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sb) //nolint:errcheck
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sb, err := s.manager.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sb) //nolint:errcheck
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.manager.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	sandboxes := s.manager.List()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sandboxes) //nolint:errcheck
}
