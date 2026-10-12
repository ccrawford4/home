package demand

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type sink struct {
	*httptest.Server
	mu  sync.Mutex
	got []map[string]string
}

func newSink(t *testing.T) *sink {
	s := &sink{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]string
		json.NewDecoder(r.Body).Decode(&m)
		s.mu.Lock()
		s.got = append(s.got, m)
		s.mu.Unlock()
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *sink) calls() []map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]string(nil), s.got...)
}

func TestThrottle(t *testing.T) {
	type step struct {
		advance time.Duration
		reason  string
	}
	tests := []struct {
		name  string
		steps []step
		want  int
	}{
		{"gpu request throttled to interval", []step{{0, ReasonGPURequest}, {10 * time.Second, ReasonGPURequest}, {19 * time.Second, ReasonGPURequest}, {time.Second, ReasonGPURequest}}, 2},
		{"no gpu throttled to 5s", []step{{0, ReasonNoGPU}, {4 * time.Second, ReasonNoGPU}, {time.Second, ReasonNoGPU}, {time.Second, ReasonNoGPU}}, 2},
		{"reasons throttled independently", []step{{0, ReasonNoGPU}, {0, ReasonGPURequest}, {0, ReasonNoGPU}, {0, ReasonGPURequest}}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := newSink(t)
			s := New(k.URL, 30*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
			now := time.Now()
			s.SetClock(func() time.Time { return now })
			for _, st := range tt.steps {
				now = now.Add(st.advance)
				s.Signal("qwen2.5-coder:7b", st.reason)
				s.Wait()
			}
			calls := k.calls()
			if len(calls) != tt.want {
				t.Fatalf("got %d calls, want %d: %v", len(calls), tt.want, calls)
			}
			if calls[0]["model"] != "qwen2.5-coder:7b" || calls[0]["reason"] != tt.steps[0].reason {
				t.Fatalf("body %v", calls[0])
			}
		})
	}
}

func TestNilAndUnreachableNeverBlock(t *testing.T) {
	var s *Signaller = New("", time.Second, nil)
	s.Signal("m", ReasonNoGPU) // nil: no-op
	s.Wait()

	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL
	dead.Close()
	u := New(url, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	start := time.Now()
	u.Signal("m", ReasonNoGPU)
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("Signal blocked")
	}
	u.Wait()
}
