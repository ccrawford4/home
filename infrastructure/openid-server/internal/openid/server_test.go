package openid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"openid-proxy/internal/kubernetes"
)

func TestServerEndpoints(t *testing.T) {
	source := &fakeRawGetter{
		responses: map[string][]byte{
			DiscoveryPath: []byte(`{"issuer":"https://kubernetes.default.svc","jwks_uri":"https://kubernetes.default.svc/openid/v1/jwks"}`),
			JWKSPath:      []byte(`{"keys":[{"kid":"local"}]}`),
		},
	}
	server := NewServer(source, ServerOptions{})

	t.Run("issuer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/issuer", nil)
		rec := httptest.NewRecorder()

		server.Issuer(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if got, want := rec.Body.String(), "http://example.com\n"; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})

	t.Run("discovery", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, DiscoveryPath, nil)
		rec := httptest.NewRecorder()

		server.Discovery(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got["issuer"] != "http://example.com" {
			t.Fatalf("issuer = %v, want %q", got["issuer"], "http://example.com")
		}
		if got["jwks_uri"] != "http://example.com/openid/v1/jwks" {
			t.Fatalf("jwks_uri = %v, want %q", got["jwks_uri"], "http://example.com/openid/v1/jwks")
		}
		if got, want := source.requests[len(source.requests)-1].Accept, AcceptJSON; got != want {
			t.Fatalf("Accept = %q, want %q", got, want)
		}
	})

	t.Run("jwks", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, JWKSPath, nil)
		rec := httptest.NewRecorder()

		server.JWKS(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if got, want := rec.Header().Get("Content-Type"), "application/jwk-set+json"; got != want {
			t.Fatalf("Content-Type = %q, want %q", got, want)
		}
		if got, want := rec.Body.String(), `{"keys":[{"kid":"local"}]}`; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
		if got, want := source.requests[len(source.requests)-1].Accept, AcceptJWKSet; got != want {
			t.Fatalf("Accept = %q, want %q", got, want)
		}
	})
}

func TestServerEndpointsWithPublicIssuerOverride(t *testing.T) {
	server := NewServer(&fakeRawGetter{responses: map[string][]byte{
		DiscoveryPath: []byte(`{"issuer":"https://kubernetes.default.svc","jwks_uri":"https://kubernetes.default.svc/openid/v1/jwks"}`),
		JWKSPath:      []byte(`{"keys":[{"kid":"cluster"}]}`),
	}}, ServerOptions{
		PublicIssuerURL: "https://openid.calum.sh/",
	})

	t.Run("issuer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/issuer", nil)
		rec := httptest.NewRecorder()

		server.Issuer(rec, req)

		if got, want := rec.Body.String(), "https://openid.calum.sh\n"; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})

	t.Run("jwks", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, JWKSPath, nil)
		rec := httptest.NewRecorder()

		server.JWKS(rec, req)

		if got, want := rec.Header().Get("Content-Type"), "application/jwk-set+json"; got != want {
			t.Fatalf("Content-Type = %q, want %q", got, want)
		}
		if got, want := rec.Body.String(), `{"keys":[{"kid":"cluster"}]}`; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})
}

type fakeRawGetter struct {
	responses map[string][]byte
	requests  []kubernetes.RawRequest
}

func (g *fakeRawGetter) GetRaw(_ context.Context, req kubernetes.RawRequest) ([]byte, error) {
	g.requests = append(g.requests, req)
	return g.responses[req.Path], nil
}
