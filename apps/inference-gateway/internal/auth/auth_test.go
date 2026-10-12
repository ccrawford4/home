package auth

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"inference-gateway/internal/config"
)

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "agents"), []byte("  k1\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "personal"), []byte("k2"), 0o600)
	os.WriteFile(filepath.Join(dir, "empty"), []byte("\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "stranger"), []byte("k3"), 0o600)
	clients := []config.Client{{Name: "agents"}, {Name: "personal", Private: true}, {Name: "empty"}, {Name: "missing"}}
	k := LoadDir(dir, clients, slog.New(slog.NewTextHandler(io.Discard, nil)))

	tests := []struct {
		key, want string
		private   bool
	}{
		{"k1", "agents", false},
		{"k2", "personal", true},
		{"k3", "", false}, // file present but client not configured
		{"", "", false},
		{"k1 ", "", false},
	}
	for _, tt := range tests {
		c, ok := k.Lookup(tt.key)
		if ok != (tt.want != "") || c.Name != tt.want || c.Private != tt.private {
			t.Errorf("Lookup(%q) = %+v, %v; want %q private=%v", tt.key, c, ok, tt.want, tt.private)
		}
	}
	if k.Len() != 2 {
		t.Fatalf("Len = %d, want 2", k.Len())
	}
}

func TestBearerToken(t *testing.T) {
	for h, want := range map[string]string{"Bearer abc": "abc", "bearer  abc ": "abc", "Basic abc": "", "": "", "Bearer ": ""} {
		r, _ := http.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", h)
		if got := BearerToken(r); got != want {
			t.Errorf("BearerToken(%q) = %q, want %q", h, got, want)
		}
	}
}
