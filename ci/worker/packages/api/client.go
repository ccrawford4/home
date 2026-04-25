package api

import (
	"net/http"
)

type Server struct {
	mux *http.ServeMux
}

func NewServer() *Server {
	server := &Server{
		mux: http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) routes() {
	s.mux.HandleFunc("/terraform/plan", s.handleTerraformPlan)
}

func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}
