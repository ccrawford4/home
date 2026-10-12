// Package router turns a requested model name into an ordered list of
// backend candidates.
package router

import (
	"fmt"
	"strings"

	"inference-gateway/internal/config"
	"inference-gateway/internal/registry"
)

// Candidate is one backend to try and the model name to send it.
type Candidate struct {
	Backend *registry.Backend
	Model   string
}

// NoBackendError means no healthy, permitted backend can serve the request.
type NoBackendError struct {
	Model   string
	Privacy bool // privacy filtering removed at least one otherwise-usable candidate
}

func (e *NoBackendError) Error() string {
	if e.Privacy {
		return fmt.Sprintf("no backend available for model %s (private request: only on-prem backends allowed)", e.Model)
	}
	return fmt.Sprintf("no backend available for model %s", e.Model)
}

// Router resolves model names using the catalog, tiers and live registry.
type Router struct {
	catalog config.Catalog
	tiers   config.Tiers
	reg     *registry.Registry
}

// New returns a Router.
func New(cfg *config.Config, reg *registry.Registry) *Router {
	return &Router{catalog: cfg.Models, tiers: cfg.Tiers, reg: reg}
}

// Resolve returns candidates in the order they should be tried. Resolution:
//
//   - "<backend>/<model>" where <backend> is a known backend: pinned to it,
//     forwarding <model>.
//   - a tier name: walk the tier's entries in order; each entry expands to the
//     matching backends ("gpu" = every registered gpu* backend, by name), and
//     each backend is sent the first model in the catalog (file order) that
//     lists it.
//   - otherwise a plain model: the backends listed for it in the catalog.
//
// Candidates are then filtered by privacy (private → onPrem only) and health.
func (r *Router) Resolve(model string, private bool) ([]Candidate, error) {
	var cands []Candidate
	if name, rest, ok := strings.Cut(model, "/"); ok && rest != "" {
		if b, found := r.reg.Get(name); found {
			cands = []Candidate{{Backend: b, Model: rest}}
			return filter(model, cands, private)
		}
	}
	if entries, ok := r.tiers.Lookup(model); ok {
		seen := map[string]bool{}
		for _, e := range entries {
			for _, b := range r.reg.Matching(e) {
				if seen[b.Name] {
					continue
				}
				seen[b.Name] = true
				if m, ok := r.firstModelFor(b); ok {
					cands = append(cands, Candidate{Backend: b, Model: m})
				}
			}
		}
		return filter(model, cands, private)
	}
	if entries, ok := r.catalog.Lookup(model); ok {
		seen := map[string]bool{}
		for _, e := range entries {
			for _, b := range r.reg.Matching(e) {
				if !seen[b.Name] {
					seen[b.Name] = true
					cands = append(cands, Candidate{Backend: b, Model: model})
				}
			}
		}
	}
	return filter(model, cands, private)
}

// GPUModel reports whether model's tier or catalog entry includes the "gpu"
// wildcard and, if so, the model a gpu backend would be sent: the model
// itself, or for a tier the first catalog model listing "gpu".
func (r *Router) GPUModel(model string) (string, bool) {
	if entries, ok := r.tiers.Lookup(model); ok {
		for _, e := range entries {
			if e == config.GPUWildcard {
				for _, c := range r.catalog {
					for _, ce := range c.Backends {
						if ce == config.GPUWildcard {
							return c.Model, true
						}
					}
				}
				return "", false
			}
		}
		return "", false
	}
	entries, _ := r.catalog.Lookup(model)
	for _, e := range entries {
		if e == config.GPUWildcard {
			return model, true
		}
	}
	return "", false
}

// GPUAvailable reports whether any registered gpu* backend is healthy.
func (r *Router) GPUAvailable() bool {
	for _, b := range r.reg.Matching(config.GPUWildcard) {
		if b.Healthy() {
			return true
		}
	}
	return false
}

func (r *Router) firstModelFor(b *registry.Backend) (string, bool) {
	for _, e := range r.catalog {
		for _, entry := range e.Backends {
			if b.Matches(entry) {
				return e.Model, true
			}
		}
	}
	return "", false
}

func filter(model string, cands []Candidate, private bool) ([]Candidate, error) {
	var out []Candidate
	privacyDropped := false
	for _, c := range cands {
		if private && !c.Backend.OnPrem {
			privacyDropped = true
			continue
		}
		if !c.Backend.Healthy() {
			continue
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, &NoBackendError{Model: model, Privacy: privacyDropped}
	}
	return out, nil
}
