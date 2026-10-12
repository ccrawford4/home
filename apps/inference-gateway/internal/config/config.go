// Package config loads the inference-gateway YAML configuration.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// GPUWildcard is the backend entry in models/tiers that matches any
// backend, static or dynamically registered, whose name starts with "gpu".
const GPUWildcard = "gpu"

// Config is the top-level configuration file.
type Config struct {
	Listen          string        `yaml:"listen"`
	QueueDepth      int           `yaml:"queueDepth"`
	RegistrationTTL time.Duration `yaml:"registrationTTL"`
	Backends        []Backend     `yaml:"backends"`
	Models          Catalog       `yaml:"models"`
	Tiers           Tiers         `yaml:"tiers"`
	Clients         []Client      `yaml:"clients"`
	// DemandURL, when set, receives GPU demand signals for the scaler.
	DemandURL string `yaml:"demandURL"`
	// DemandInterval throttles "gpu request" demand signals (default 30s).
	DemandInterval time.Duration `yaml:"demandInterval"`
}

// Backend is a statically configured inference server.
type Backend struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url"`
	OnPrem      bool   `yaml:"onPrem"`
	Concurrency int    `yaml:"concurrency"`
	// KeepAlive, when set, is injected as keep_alive into /api/chat and
	// /api/generate bodies that don't carry one. Any YAML scalar is accepted
	// (-1, 0, "5m") and forwarded as-is.
	KeepAlive any `yaml:"keepAlive"`
}

// Client is an API consumer. Its key lives in KEYS_DIR/<name>.
type Client struct {
	Name    string `yaml:"name"`
	Private bool   `yaml:"private"`
}

// CatalogEntry maps one model name to the backends (or "gpu") that serve it.
type CatalogEntry struct {
	Model    string
	Backends []string
}

// Catalog is the ordered models map. Order matters for tier resolution.
type Catalog []CatalogEntry

// UnmarshalYAML keeps the mapping order from the file.
func (c *Catalog) UnmarshalYAML(n *yaml.Node) error {
	entries, err := orderedLists(n)
	if err != nil {
		return fmt.Errorf("models: %w", err)
	}
	for _, e := range entries {
		*c = append(*c, CatalogEntry{Model: e.key, Backends: e.values})
	}
	return nil
}

// Lookup returns the backend entries for model.
func (c Catalog) Lookup(model string) ([]string, bool) {
	for _, e := range c {
		if e.Model == model {
			return e.Backends, true
		}
	}
	return nil, false
}

// Tier is a named, ordered list of backend entries.
type Tier struct {
	Name     string
	Backends []string
}

// Tiers is the ordered tiers map.
type Tiers []Tier

// UnmarshalYAML keeps the mapping order from the file.
func (t *Tiers) UnmarshalYAML(n *yaml.Node) error {
	entries, err := orderedLists(n)
	if err != nil {
		return fmt.Errorf("tiers: %w", err)
	}
	for _, e := range entries {
		*t = append(*t, Tier{Name: e.key, Backends: e.values})
	}
	return nil
}

// Lookup returns the backend entries for the tier name.
func (t Tiers) Lookup(name string) ([]string, bool) {
	for _, e := range t {
		if e.Name == name {
			return e.Backends, true
		}
	}
	return nil, false
}

type kv struct {
	key    string
	values []string
}

func orderedLists(n *yaml.Node) ([]kv, error) {
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: expected a mapping", n.Line)
	}
	var out []kv
	for i := 0; i+1 < len(n.Content); i += 2 {
		var e kv
		if err := n.Content[i].Decode(&e.key); err != nil {
			return nil, err
		}
		if err := n.Content[i+1].Decode(&e.values); err != nil {
			return nil, fmt.Errorf("%s: %w", e.key, err)
		}
		out = append(out, e)
	}
	return out, nil
}

// Load reads and validates the config file at path.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse decodes and validates YAML config bytes, applying defaults.
func Parse(b []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.QueueDepth <= 0 {
		c.QueueDepth = 16
	}
	if c.RegistrationTTL <= 0 {
		c.RegistrationTTL = 60 * time.Second
	}
	if c.DemandInterval <= 0 {
		c.DemandInterval = 30 * time.Second
	}
	return &c, c.validate()
}

func (c *Config) validate() error {
	names := map[string]bool{}
	for _, b := range c.Backends {
		if b.Name == "" || strings.Contains(b.Name, "/") {
			return fmt.Errorf("backend %q: name must be non-empty and contain no '/'", b.Name)
		}
		if names[b.Name] {
			return fmt.Errorf("backend %q: duplicate name", b.Name)
		}
		if b.Name == GPUWildcard {
			return fmt.Errorf("backend %q: name is reserved for the gpu wildcard; use e.g. %q", b.Name, GPUWildcard+"-l4")
		}
		if err := ValidateURL(b.URL); err != nil {
			return fmt.Errorf("backend %q: %w", b.Name, err)
		}
		if b.Concurrency < 0 {
			return fmt.Errorf("backend %q: concurrency must be >= 0", b.Name)
		}
		names[b.Name] = true
	}
	checkRefs := func(kind, key string, refs []string) error {
		if len(refs) == 0 {
			return fmt.Errorf("%s %q: empty backend list", kind, key)
		}
		for _, r := range refs {
			if r != GPUWildcard && !names[r] {
				return fmt.Errorf("%s %q: unknown backend %q", kind, key, r)
			}
		}
		return nil
	}
	for _, m := range c.Models {
		if err := checkRefs("model", m.Model, m.Backends); err != nil {
			return err
		}
	}
	for _, t := range c.Tiers {
		if _, clash := c.Models.Lookup(t.Name); clash {
			return fmt.Errorf("tier %q: clashes with a model name", t.Name)
		}
		if err := checkRefs("tier", t.Name, t.Backends); err != nil {
			return err
		}
	}
	if c.DemandURL != "" {
		if err := ValidateURL(c.DemandURL); err != nil {
			return fmt.Errorf("demandURL: %w", err)
		}
	}
	clients := map[string]bool{}
	for _, cl := range c.Clients {
		if cl.Name == "" || strings.ContainsAny(cl.Name, `/\`) || strings.HasPrefix(cl.Name, ".") {
			return fmt.Errorf("client %q: invalid name", cl.Name)
		}
		if clients[cl.Name] {
			return fmt.Errorf("client %q: duplicate name", cl.Name)
		}
		clients[cl.Name] = true
	}
	return nil
}

// ValidateURL checks that u is an absolute http(s) URL.
func ValidateURL(u string) error {
	p, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
		return fmt.Errorf("url %q must be absolute http(s)", u)
	}
	return nil
}
