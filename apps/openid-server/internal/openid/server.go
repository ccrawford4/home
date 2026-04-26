package openid

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
)

const (
	DiscoveryPath = "/.well-known/openid-configuration"
	JWKSPath      = "/openid/v1/jwks"
)

type RawGetter interface {
	GetRaw(ctx context.Context, rawPath string) ([]byte, error)
}

type Server struct {
	rawGetter RawGetter
}

func NewServer(rawGetter RawGetter) *Server {
	return &Server{
		rawGetter: rawGetter,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.Healthz)
	mux.HandleFunc("GET /issuer", s.Issuer)
	mux.HandleFunc("GET "+DiscoveryPath, s.Discovery)
	mux.HandleFunc("GET "+JWKSPath, s.JWKS)
	return mux
}

func (s *Server) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) Issuer(w http.ResponseWriter, r *http.Request) {
	body, err := s.rawGetter.GetRaw(r.Context(), DiscoveryPath)
	if err != nil {
		writeError(w, err)
		return
	}

	doc, err := ParseDiscoveryDocument(body)
	if err != nil {
		writeError(w, err)
		return
	}
	if doc.Issuer == "" {
		writeError(w, errors.New("discovery document did not include issuer"))
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintln(w, doc.Issuer)
}

func (s *Server) Discovery(w http.ResponseWriter, r *http.Request) {
	body, err := s.rawGetter.GetRaw(r.Context(), DiscoveryPath)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func (s *Server) JWKS(w http.ResponseWriter, r *http.Request) {
	body, err := s.rawGetter.GetRaw(r.Context(), JWKSPath)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func writeError(w http.ResponseWriter, err error) {
	log.Print(err)
	http.Error(w, err.Error(), http.StatusBadGateway)
}
