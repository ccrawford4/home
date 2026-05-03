package openid

import (
	"encoding/json"
	"testing"
)

func TestParseDiscoveryDocument(t *testing.T) {
	doc, err := ParseDiscoveryDocument([]byte(`{"issuer":"https://kubernetes.default.svc","jwks_uri":"https://kubernetes.default.svc/openid/v1/jwks"}`))
	if err != nil {
		t.Fatal(err)
	}

	if got, want := doc.Issuer, "https://kubernetes.default.svc"; got != want {
		t.Fatalf("issuer = %q, want %q", got, want)
	}
	if got, want := doc.JWKSURI, "https://kubernetes.default.svc/openid/v1/jwks"; got != want {
		t.Fatalf("jwks_uri = %q, want %q", got, want)
	}
}

func TestRewriteDiscoveryDocument(t *testing.T) {
	body, err := RewriteDiscoveryDocument([]byte(`{"issuer":"https://kubernetes.default.svc","jwks_uri":"https://kubernetes.default.svc/openid/v1/jwks"}`), "https://openid.calum.sh/")
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["issuer"] != "https://openid.calum.sh" {
		t.Fatalf("issuer = %v, want %q", got["issuer"], "https://openid.calum.sh")
	}
	if got["jwks_uri"] != "https://openid.calum.sh/openid/v1/jwks" {
		t.Fatalf("jwks_uri = %v, want %q", got["jwks_uri"], "https://openid.calum.sh/openid/v1/jwks")
	}
}
