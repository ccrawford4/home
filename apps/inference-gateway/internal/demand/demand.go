// Package demand sends fire-and-forget GPU demand signals to the scaler.
package demand

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Reasons sent to the scaler.
const (
	ReasonNoGPU      = "no gpu backend"
	ReasonGPURequest = "gpu request"
)

// noGPUInterval throttles ReasonNoGPU signals.
const noGPUInterval = 5 * time.Second

// Signaller posts {"model","reason"} to a URL, throttled per reason. A nil
// Signaller (or one with an empty URL) does nothing.
type Signaller struct {
	url      string
	interval time.Duration
	client   *http.Client
	log      *slog.Logger

	mu   sync.Mutex
	last map[string]time.Time
	now  func() time.Time
	wg   sync.WaitGroup
}

// New returns a Signaller, or nil when url is empty. interval throttles
// ReasonGPURequest signals.
func New(url string, interval time.Duration, log *slog.Logger) *Signaller {
	if url == "" {
		return nil
	}
	return &Signaller{
		url:      url,
		interval: interval,
		client:   &http.Client{Timeout: 2 * time.Second},
		log:      log,
		last:     map[string]time.Time{},
		now:      time.Now,
	}
}

// SetClock overrides the time source (for tests).
func (s *Signaller) SetClock(now func() time.Time) {
	s.mu.Lock()
	s.now = now
	s.mu.Unlock()
}

// Signal sends a demand signal in the background unless one with the same
// reason was sent within its throttle window. It never blocks.
func (s *Signaller) Signal(model, reason string) {
	if s == nil {
		return
	}
	window := s.interval
	if reason == ReasonNoGPU {
		window = noGPUInterval
	}
	s.mu.Lock()
	now := s.now()
	if last, ok := s.last[reason]; ok && now.Sub(last) < window {
		s.mu.Unlock()
		return
	}
	s.last[reason] = now
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.post(model, reason)
	}()
}

func (s *Signaller) post(model, reason string) {
	body, _ := json.Marshal(map[string]string{"model": model, "reason": reason})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		s.log.Warn("demand signal failed", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		s.log.Warn("demand signal failed", "model", model, "reason", reason, "error", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		s.log.Warn("demand signal rejected", "model", model, "reason", reason, "status", resp.StatusCode)
		return
	}
	s.log.Info("demand signal sent", "model", model, "reason", reason)
}

// Wait blocks until in-flight signals finish (for tests and shutdown).
func (s *Signaller) Wait() {
	if s != nil {
		s.wg.Wait()
	}
}
