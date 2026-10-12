// Package auth resolves bearer keys to configured clients.
package auth

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"inference-gateway/internal/config"
)

// Client is an authenticated API consumer.
type Client struct {
	Name    string
	Private bool
}

type entry struct {
	client Client
	key    []byte
}

// Keys maps bearer keys to clients.
type Keys struct {
	entries []entry
}

// New builds Keys from an explicit client→key map (used by tests).
func New(clients []config.Client, keys map[string]string) *Keys {
	k := &Keys{}
	for _, c := range clients {
		key := strings.TrimSpace(keys[c.Name])
		if key == "" {
			continue
		}
		k.entries = append(k.entries, entry{client: Client{Name: c.Name, Private: c.Private}, key: []byte(key)})
	}
	return k
}

// LoadDir reads dir/<client name> for every configured client. Clients
// without a key file are logged and cannot authenticate.
func LoadDir(dir string, clients []config.Client, log *slog.Logger) *Keys {
	m := map[string]string{}
	for _, c := range clients {
		b, err := os.ReadFile(filepath.Join(dir, c.Name))
		if err != nil {
			log.Warn("no key for client; it cannot authenticate", "client", c.Name, "error", err)
			continue
		}
		if strings.TrimSpace(string(b)) == "" {
			log.Warn("empty key file for client; it cannot authenticate", "client", c.Name)
			continue
		}
		m[c.Name] = string(b)
	}
	return New(clients, m)
}

// Lookup returns the client owning key.
func (k *Keys) Lookup(key string) (Client, bool) {
	if key == "" {
		return Client{}, false
	}
	var found Client
	ok := false
	for _, e := range k.entries {
		if subtle.ConstantTimeCompare(e.key, []byte(key)) == 1 {
			found, ok = e.client, true
		}
	}
	return found, ok
}

// Len returns how many clients can authenticate.
func (k *Keys) Len() int { return len(k.entries) }

// BearerToken extracts the token from an Authorization: Bearer header.
func BearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// LoadAdminKey reads the admin key file. An empty result disables the
// registration endpoints.
func LoadAdminKey(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(b))
	if key == "" {
		return "", fmt.Errorf("admin key file %s is empty", path)
	}
	return key, nil
}

// MatchAdmin compares a presented token with the admin key.
func MatchAdmin(adminKey, token string) bool {
	return adminKey != "" && subtle.ConstantTimeCompare([]byte(adminKey), []byte(token)) == 1
}
