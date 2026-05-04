package kubernetes

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestRawClientGetRaw(t *testing.T) {
	tokenFile := writeTempToken(t, "test-token")
	client, err := NewRawClient(RawClientOptions{
		APIServerURL: "http://kubernetes.test",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got, want := r.URL.String(), "http://kubernetes.test/openid/v1/jwks"; got != want {
				t.Fatalf("url = %q, want %q", got, want)
			}
			if got, want := r.Header.Get("Authorization"), "Bearer test-token"; got != want {
				t.Fatalf("Authorization header = %q, want %q", got, want)
			}
			if got, want := r.Header.Get("Accept"), "application/jwk-set+json, application/json, */*"; got != want {
				t.Fatalf("Accept header = %q, want %q", got, want)
			}
			if got, want := r.Header.Get("X-Test-Header"), "test-value"; got != want {
				t.Fatalf("X-Test-Header = %q, want %q", got, want)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`{"keys":[]}`)),
			}, nil
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	client.tokenSource.tokenFile = tokenFile

	body, err := client.GetRaw(context.Background(), RawRequest{
		Path:   "/openid/v1/jwks",
		Accept: "application/jwk-set+json, application/json, */*",
		Header: http.Header{"X-Test-Header": []string{"test-value"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), `{"keys":[]}`; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func writeTempToken(t *testing.T, token string) string {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "token-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(token); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
