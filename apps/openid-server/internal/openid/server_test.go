package openid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerEndpoints(t *testing.T) {
	source := &fakeRawGetter{
		responses: map[string][]byte{
			DiscoveryPath: []byte(`{"issuer":"https://kubernetes.default.svc","jwks_uri":"https://kubernetes.default.svc/openid/v1/jwks"}`),
			JWKSPath:      []byte(`{"keys":[{"kid":"local"}]}`),
		},
	}
	server := NewServer(source)

	t.Run("issuer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/issuer", nil)
		rec := httptest.NewRecorder()

		server.Issuer(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if got, want := rec.Body.String(), "https://kubernetes.default.svc\n"; got != want {
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
		if got, want := rec.Body.String(), `{"issuer":"https://kubernetes.default.svc","jwks_uri":"https://kubernetes.default.svc/openid/v1/jwks"}`; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})

	t.Run("jwks", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, JWKSPath, nil)
		rec := httptest.NewRecorder()

		server.JWKS(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if got, want := rec.Body.String(), `{"keys":[{"kid":"local"}]}`; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})
}

type fakeRawGetter struct {
	responses map[string][]byte
}

func (g *fakeRawGetter) GetRaw(_ context.Context, rawPath string) ([]byte, error) {
	return g.responses[rawPath], nil
}
