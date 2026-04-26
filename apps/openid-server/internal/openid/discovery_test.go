package openid

import "testing"

func TestParseDiscoveryDocument(t *testing.T) {
	doc, err := ParseDiscoveryDocument([]byte(`{"issuer":"https://kubernetes.default.svc","jwks_uri":"https://kubernetes.default.svc/openid/v1/jwks"}`))
	if err != nil {
		t.Fatal(err)
	}

	if got, want := doc.Issuer, "https://kubernetes.default.svc"; got != want {
		t.Fatalf("issuer = %q, want %q", got, want)
	}
}
