// Package registry tracks the static and dynamically registered backends and
// their health.
package registry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"inference-gateway/internal/config"
	"inference-gateway/internal/limiter"
)

// Backend is one upstream Ollama-compatible server.
type Backend struct {
	Name        string
	URL         string
	OnPrem      bool
	Concurrency int
	KeepAlive   any
	Static      bool
	Limiter     *limiter.Limiter

	healthy atomic.Bool
	expires time.Time // dynamic only; guarded by Registry.mu
}

// Healthy reports the last health-check result.
func (b *Backend) Healthy() bool { return b.healthy.Load() }

// SetHealthy records a health-check result.
func (b *Backend) SetHealthy(v bool) { b.healthy.Store(v) }

// IsGPU reports whether the backend matches the "gpu" wildcard entry.
func (b *Backend) IsGPU() bool {
	return strings.HasPrefix(b.Name, config.GPUWildcard)
}

// Matches reports whether a models/tiers entry refers to this backend.
func (b *Backend) Matches(entry string) bool {
	if entry == config.GPUWildcard {
		return b.IsGPU()
	}
	return entry == b.Name
}

// Registration is the body of POST /internal/backends.
type Registration struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	OnPrem      bool   `json:"onPrem"`
	Concurrency int    `json:"concurrency"`
	KeepAlive   any    `json:"keepAlive,omitempty"`
}

// ErrStatic is returned when a registration targets a static backend.
var ErrStatic = errors.New("static backends cannot be modified")

// Registry holds all backends.
type Registry struct {
	mu         sync.Mutex
	static     []*Backend
	dynamic    map[string]*Backend
	ttl        time.Duration
	queueDepth int
	now        func() time.Time
	log        *slog.Logger
}

// New builds a registry from the static config. Static backends start healthy.
func New(cfg *config.Config, log *slog.Logger) *Registry {
	r := &Registry{
		dynamic:    map[string]*Backend{},
		ttl:        cfg.RegistrationTTL,
		queueDepth: cfg.QueueDepth,
		now:        time.Now,
		log:        log,
	}
	for _, b := range cfg.Backends {
		be := &Backend{
			Name:        b.Name,
			URL:         strings.TrimRight(b.URL, "/"),
			OnPrem:      b.OnPrem,
			Concurrency: b.Concurrency,
			KeepAlive:   b.KeepAlive,
			Static:      true,
			Limiter:     limiter.New(b.Concurrency, cfg.QueueDepth),
		}
		be.SetHealthy(true)
		r.static = append(r.static, be)
	}
	return r
}

// SetClock overrides the time source (for tests).
func (r *Registry) SetClock(now func() time.Time) {
	r.mu.Lock()
	r.now = now
	r.mu.Unlock()
}

// Register adds or refreshes a dynamic backend and resets its TTL.
func (r *Registry) Register(reg Registration) (*Backend, error) {
	if reg.Name == "" || strings.Contains(reg.Name, "/") {
		return nil, fmt.Errorf("name must be non-empty and contain no '/'")
	}
	if err := config.ValidateURL(reg.URL); err != nil {
		return nil, err
	}
	if reg.Concurrency < 0 {
		return nil, fmt.Errorf("concurrency must be >= 0")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.static {
		if s.Name == reg.Name {
			return nil, ErrStatic
		}
	}
	url := strings.TrimRight(reg.URL, "/")
	b, ok := r.dynamic[reg.Name]
	if !ok || b.URL != url || b.Concurrency != reg.Concurrency || b.OnPrem != reg.OnPrem ||
		fmt.Sprint(b.KeepAlive) != fmt.Sprint(reg.KeepAlive) {
		// New or changed: replace the entry. In-flight requests keep their
		// reference to the old Backend and release its limiter.
		b = &Backend{
			Name:        reg.Name,
			URL:         url,
			OnPrem:      reg.OnPrem,
			Concurrency: reg.Concurrency,
			KeepAlive:   reg.KeepAlive,
			Limiter:     limiter.New(reg.Concurrency, r.queueDepth),
		}
		r.dynamic[reg.Name] = b
		r.log.Info("backend registered", "backend", b.Name, "url", b.URL, "on_prem", b.OnPrem, "concurrency", b.Concurrency)
	}
	b.SetHealthy(true)
	b.expires = r.now().Add(r.ttl)
	return b, nil
}

// Remove deletes a dynamic backend. It reports whether one was removed.
func (r *Registry) Remove(name string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.static {
		if s.Name == name {
			return false, ErrStatic
		}
	}
	if _, ok := r.dynamic[name]; !ok {
		return false, nil
	}
	delete(r.dynamic, name)
	r.log.Info("backend removed", "backend", name)
	return true, nil
}

// All returns every live backend (static first, then dynamic sorted by name),
// dropping expired registrations.
func (r *Registry) All() []*Backend {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	out := append([]*Backend(nil), r.static...)
	var dyn []*Backend
	for name, b := range r.dynamic {
		if now.After(b.expires) {
			delete(r.dynamic, name)
			r.log.Warn("backend registration expired", "backend", name)
			continue
		}
		dyn = append(dyn, b)
	}
	sort.Slice(dyn, func(i, j int) bool { return dyn[i].Name < dyn[j].Name })
	return append(out, dyn...)
}

// Get returns a live backend by name.
func (r *Registry) Get(name string) (*Backend, bool) {
	for _, b := range r.All() {
		if b.Name == name {
			return b, true
		}
	}
	return nil, false
}

// Matching returns live backends that a models/tiers entry refers to.
func (r *Registry) Matching(entry string) []*Backend {
	var out []*Backend
	for _, b := range r.All() {
		if b.Matches(entry) {
			out = append(out, b)
		}
	}
	return out
}

// CheckHealth probes every backend's GET /api/version once.
func (r *Registry) CheckHealth(ctx context.Context, client *http.Client, timeout time.Duration) {
	var wg sync.WaitGroup
	for _, b := range r.All() {
		wg.Add(1)
		go func(b *Backend) {
			defer wg.Done()
			err := probe(ctx, client, b.URL+"/api/version", timeout)
			was := b.Healthy()
			b.SetHealthy(err == nil)
			if was && err != nil {
				r.log.Warn("backend unhealthy", "backend", b.Name, "error", err)
			} else if !was && err == nil {
				r.log.Info("backend healthy", "backend", b.Name)
			}
		}(b)
	}
	wg.Wait()
}

// RunHealthChecks polls backends every interval until ctx ends.
func (r *Registry) RunHealthChecks(ctx context.Context, client *http.Client, interval, timeout time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		r.CheckHealth(ctx, client, timeout)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func probe(ctx context.Context, client *http.Client, url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
