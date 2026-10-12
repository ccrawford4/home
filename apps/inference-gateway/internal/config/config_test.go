package config

import (
	"strings"
	"testing"
	"time"
)

const sample = `
listen: ":9090"
queueDepth: 8
registrationTTL: 30s
backends:
  - name: hailo
    url: http://virgo.virgo.svc.cluster.local:8000
    onPrem: true
    concurrency: 1
    keepAlive: -1
models:
  qwen2.5-coder:1.5b: [hailo]
  qwen2:1.5b: [hailo]
  qwen2.5-coder:7b: [gpu]
tiers:
  fast: [gpu, hailo]
  local: [hailo]
clients:
  - name: sandbox-agents
  - name: personal-agent
    private: true
`

func TestParse(t *testing.T) {
	c, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != ":9090" || c.QueueDepth != 8 || c.RegistrationTTL != 30*time.Second {
		t.Fatalf("scalars: %+v", c)
	}
	var order []string
	for _, m := range c.Models {
		order = append(order, m.Model)
	}
	if strings.Join(order, ",") != "qwen2.5-coder:1.5b,qwen2:1.5b,qwen2.5-coder:7b" {
		t.Fatalf("catalog order lost: %v", order)
	}
	if c.Tiers[0].Name != "fast" || strings.Join(c.Tiers[0].Backends, ",") != "gpu,hailo" {
		t.Fatalf("tiers: %+v", c.Tiers)
	}
	if c.Backends[0].KeepAlive != -1 {
		t.Fatalf("keepAlive %#v", c.Backends[0].KeepAlive)
	}
	if !c.Clients[1].Private || c.Clients[0].Private {
		t.Fatalf("clients %+v", c.Clients)
	}

	d, err := Parse([]byte("backends: []"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Listen != ":8080" || d.QueueDepth != 16 || d.RegistrationTTL != time.Minute || d.DemandInterval != 30*time.Second || d.DemandURL != "" {
		t.Fatalf("defaults: %+v", d)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct{ name, yaml, want string }{
		{"unknown backend in model", "models:\n  m: [nope]", "unknown backend"},
		{"unknown backend in tier", "tiers:\n  t: [nope]", "unknown backend"},
		{"tier clashes with model", "backends: [{name: a, url: 'http://a'}]\nmodels:\n  x: [a]\ntiers:\n  x: [a]", "clashes"},
		{"bare gpu name reserved", "backends: [{name: gpu, url: 'http://a'}]", "reserved"},
		{"bad url", "backends: [{name: a, url: 'a:b'}]", "http"},
		{"duplicate backend", "backends: [{name: a, url: 'http://a'}, {name: a, url: 'http://b'}]", "duplicate"},
		{"bad demandURL", "demandURL: not-a-url", "demandURL"},
		{"slash in client", "clients: [{name: ../x}]", "invalid name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want error containing %q", err, tt.want)
			}
		})
	}
}
