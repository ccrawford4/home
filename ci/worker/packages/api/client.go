package api

import (
	"fmt"
	"net/http"
)

type Server struct{}

type HttpMethod string

const (
	GET    HttpMethod = "GET"
	POST   HttpMethod = "POST"
	PUT    HttpMethod = "PUT"
	DELETE HttpMethod = "DELETE"
)

func NewServer() *Server {
	return &Server{}
}

func newRoute(method HttpMethod, path string, handlerFunc http.HandlerFunc) {
	fullPath := fmt.Sprintf("%s %s", method, path)
	http.HandleFunc(fullPath, handlerFunc)
}

func (s *Server) Start() {
	http.HandleFunc("/terraform/plan", HttpMethod.POST, s.handleTerraformPlan)
}
