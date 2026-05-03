package openid

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

const (
	DiscoveryPath = "/.well-known/openid-configuration"
	JWKSPath      = "/openid/v1/jwks"
)

type RawGetter interface {
	GetRaw(ctx context.Context, rawPath string) ([]byte, error)
}

type ServerOptions struct {
	PublicIssuerURL string
	JWKSJSON        []byte
}

type Server struct {
	rawGetter       RawGetter
	publicIssuerURL string
	jwksJSON        []byte
}

func NewServer(rawGetter RawGetter, options ServerOptions) *Server {
	return &Server{
		rawGetter:       rawGetter,
		publicIssuerURL: strings.TrimRight(options.PublicIssuerURL, "/"),
		jwksJSON:        options.JWKSJSON,
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
	issuerURL := s.issuerURL(r)
	if issuerURL == "" {
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
		issuerURL = doc.Issuer
	}
	if issuerURL == "" {
		writeError(w, errors.New("discovery document did not include issuer"))
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintln(w, issuerURL)
}

func (s *Server) Discovery(w http.ResponseWriter, r *http.Request) {
	body, err := s.rawGetter.GetRaw(r.Context(), DiscoveryPath)
	if err != nil {
		writeError(w, err)
		return
	}

	updatedBody, err := RewriteDiscoveryDocument(body, s.issuerURL(r))
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(updatedBody)
}

func (s *Server) JWKS(w http.ResponseWriter, r *http.Request) {
	body := s.jwksJSON
	if len(body) == 0 {
		var err error
		body, err = s.rawGetter.GetRaw(r.Context(), JWKSPath)
		if err != nil {
			writeError(w, err)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func (s *Server) issuerURL(r *http.Request) string {
	if s.publicIssuerURL != "" {
		return s.publicIssuerURL
	}

	scheme := "http"
	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	} else if r.TLS != nil {
		scheme = "https"
	}
	if r.Host == "" {
		return ""
	}
	return scheme + "://" + r.Host
}

func writeError(w http.ResponseWriter, err error) {
	log.Print(err)
	http.Error(w, err.Error(), http.StatusBadGateway)
}
