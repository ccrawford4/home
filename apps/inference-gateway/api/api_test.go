package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"inference-gateway/internal/auth"
	"inference-gateway/internal/config"
	"inference-gateway/internal/demand"
	"inference-gateway/internal/registry"
	"inference-gateway/internal/router"
)

const (
	publicKey  = "public-key"
	privateKey = "private-key"
	adminKey   = "admin-key"
)

// fakeBackend is a stub Ollama server that records what it receives.
type fakeBackend struct {
	*httptest.Server
	mu    sync.Mutex
	reqs  []recorded
	tags  []string
	reply func(w http.ResponseWriter, r *http.Request, body map[string]any)
}

type recorded struct {
	path string
	raw  []byte
	body map[string]any
	auth string
}

func newFake(t *testing.T) *fakeBackend {
	t.Helper()
	f := &fakeBackend{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			switch r.URL.Path {
			case "/api/version":
				w.Write([]byte(`{"version":"fake"}`))
			case "/api/tags":
				var ms []map[string]any
				for _, n := range f.tags {
					ms = append(ms, map[string]any{"name": n, "model": n, "size": 1})
				}
				json.NewEncoder(w).Encode(map[string]any{"models": ms})
			default:
				http.NotFound(w, r)
			}
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(raw, &body)
		f.mu.Lock()
		f.reqs = append(f.reqs, recorded{path: r.URL.Path, raw: raw, body: body, auth: r.Header.Get("Authorization")})
		reply := f.reply
		f.mu.Unlock()
		if reply != nil {
			reply(w, r, body)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		json.NewEncoder(w).Encode(map[string]any{"model": body["model"], "done": true, "eval_count": 10, "eval_duration": 1e9})
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeBackend) requests() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.reqs...)
}

// deadURL returns a URL nothing listens on.
func deadURL(t *testing.T) string {
	s := httptest.NewServer(http.NotFoundHandler())
	u := s.URL
	s.Close()
	return u
}

type gateway struct {
	*httptest.Server
	reg *registry.Registry
}

func newGateway(t *testing.T, yaml string, urls map[string]string) *gateway {
	return newGatewayDemand(t, yaml, urls, nil)
}

func newGatewayDemand(t *testing.T, yaml string, urls map[string]string, dem *demand.Signaller) *gateway {
	t.Helper()
	for k, v := range urls {
		yaml = strings.ReplaceAll(yaml, "{"+k+"}", v)
	}
	cfg, err := config.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	keys := auth.New(cfg.Clients, map[string]string{"public": publicKey, "private": privateKey})
	reg := registry.New(cfg, log)
	var catalog []string
	for _, m := range cfg.Models {
		catalog = append(catalog, m.Model)
	}
	mux := http.NewServeMux()
	NewHandler(log, router.New(cfg, reg), reg, keys, adminKey, http.DefaultClient, catalog, dem).RegisterRoutes(mux)
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return &gateway{Server: s, reg: reg}
}

func (g *gateway) do(t *testing.T, method, path, key string, body any, hdr map[string]string) (*http.Response, []byte) {
	t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		j, _ := json.Marshal(b)
		rd = bytes.NewReader(j)
	}
	req, _ := http.NewRequest(method, g.URL+path, rd)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp, out
}

const twoBackendCfg = `
queueDepth: 4
registrationTTL: 60s
backends:
  - name: hailo
    url: {hailo}
    onPrem: true
    concurrency: 1
    keepAlive: -1
  - name: cloud
    url: {cloud}
    onPrem: false
models:
  qwen2:1.5b: [hailo]
  shared:1b: [cloud, hailo]
  cloudonly:1b: [cloud]
  qwen2.5-coder:7b: [gpu]
tiers:
  fast: [gpu, hailo]
  local: [hailo]
clients:
  - name: public
    private: false
  - name: private
    private: true
`

func TestAuth(t *testing.T) {
	hailo, cloud := newFake(t), newFake(t)
	g := newGateway(t, twoBackendCfg, map[string]string{"hailo": hailo.URL, "cloud": cloud.URL})
	tests := []struct {
		name, method, path, key string
		body                    any
		want                    int
	}{
		{"healthz needs no auth", "GET", "/healthz", "", nil, 200},
		{"missing key", "GET", "/api/version", "", nil, 401},
		{"wrong key", "GET", "/api/version", "nope", nil, 401},
		{"admin key is not a client key", "GET", "/api/version", adminKey, nil, 401},
		{"good key", "GET", "/api/version", publicKey, nil, 200},
		{"chat without key", "POST", "/api/chat", "", map[string]any{"model": "qwen2:1.5b"}, 401},
		{"openai without key", "POST", "/v1/chat/completions", "", map[string]any{"model": "qwen2:1.5b"}, 401},
		{"unknown api path without key", "GET", "/api/ps", "", nil, 401},
		{"unknown api path with key", "GET", "/api/ps", publicKey, nil, 404},
		{"internal with client key", "POST", "/internal/backends", publicKey, map[string]any{"name": "gpu-a", "url": hailo.URL}, 401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := g.do(t, tt.method, tt.path, tt.key, tt.body, nil)
			if resp.StatusCode != tt.want {
				t.Fatalf("status %d, want %d (%s)", resp.StatusCode, tt.want, body)
			}
		})
	}
	if n := len(hailo.requests()) + len(cloud.requests()); n != 0 {
		t.Fatalf("unauthenticated requests reached backends: %d", n)
	}
	if _, body := g.do(t, "GET", "/api/version", publicKey, nil, nil); !strings.Contains(string(body), `"inference-gateway"`) {
		t.Fatalf("version body %s", body)
	}
}

func TestRouting(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		header      map[string]string
		path        string
		model       string
		register    *registry.Registration // optional dynamic backend; URL "{gpu}" is replaced
		gpuDead     bool
		gpu500      bool
		hailoDown   bool
		wantStatus  int
		wantBackend string // "hailo", "cloud", "gpu"
		wantModel   string
		wantErr     string
	}{
		{name: "plain model", key: publicKey, model: "qwen2:1.5b", wantStatus: 200, wantBackend: "hailo", wantModel: "qwen2:1.5b"},
		{name: "public client prefers first listed", key: publicKey, model: "shared:1b", wantStatus: 200, wantBackend: "cloud", wantModel: "shared:1b"},
		{name: "private client skips off-prem", key: privateKey, model: "shared:1b", wantStatus: 200, wantBackend: "hailo", wantModel: "shared:1b"},
		{name: "header forces private", key: publicKey, header: map[string]string{PrivateHeader: "true"}, model: "shared:1b", wantStatus: 200, wantBackend: "hailo", wantModel: "shared:1b"},
		{name: "header cannot un-private", key: privateKey, header: map[string]string{PrivateHeader: "false"}, model: "shared:1b", wantStatus: 200, wantBackend: "hailo", wantModel: "shared:1b"},
		{name: "private client, off-prem only model", key: privateKey, model: "cloudonly:1b", wantStatus: 503, wantErr: "no backend available for model cloudonly:1b (private request"},
		{name: "header private, off-prem only model", key: publicKey, header: map[string]string{PrivateHeader: "true"}, model: "cloudonly:1b", wantStatus: 503, wantErr: "private request"},
		{name: "unknown model", key: publicKey, model: "llama3:70b", wantStatus: 503, wantErr: "no backend available for model llama3:70b"},
		{name: "unhealthy only backend", key: publicKey, model: "qwen2:1.5b", hailoDown: true, wantStatus: 503, wantErr: "no backend available for model qwen2:1.5b"},
		{name: "gpu model with no gpu registered", key: publicKey, model: "qwen2.5-coder:7b", wantStatus: 503, wantErr: "no backend available"},
		{name: "prefix pins backend", key: publicKey, model: "hailo/qwen2:1.5b", wantStatus: 200, wantBackend: "hailo", wantModel: "qwen2:1.5b"},
		{name: "prefix pins uncatalogued model", key: publicKey, model: "cloud/mistral:7b", wantStatus: 200, wantBackend: "cloud", wantModel: "mistral:7b"},
		{name: "prefix still subject to privacy", key: privateKey, model: "cloud/mistral:7b", wantStatus: 503, wantErr: "private request"},
		{name: "unknown prefix is a plain model", key: publicKey, model: "nobody/x", wantStatus: 503, wantErr: "model nobody/x"},
		{name: "local tier", key: publicKey, model: "local", wantStatus: 200, wantBackend: "hailo", wantModel: "qwen2:1.5b"},
		{name: "fast tier without gpu falls to hailo", key: publicKey, model: "fast", wantStatus: 200, wantBackend: "hailo", wantModel: "qwen2:1.5b"},
		{name: "fast tier uses gpu", key: publicKey, model: "fast",
			register: &registry.Registration{Name: "gpu-a", URL: "{gpu}", Concurrency: 2}, wantStatus: 200, wantBackend: "gpu", wantModel: "qwen2.5-coder:7b"},
		{name: "fast tier, gpu connection fails", key: publicKey, model: "fast", gpuDead: true,
			register: &registry.Registration{Name: "gpu-a", URL: "{gpu}"}, wantStatus: 200, wantBackend: "hailo", wantModel: "qwen2:1.5b"},
		{name: "fast tier, gpu returns 500", key: publicKey, model: "fast", gpu500: true,
			register: &registry.Registration{Name: "gpu-a", URL: "{gpu}"}, wantStatus: 200, wantBackend: "hailo", wantModel: "qwen2:1.5b"},
		{name: "fast tier, private client skips off-prem gpu", key: privateKey, model: "fast",
			register: &registry.Registration{Name: "gpu-a", URL: "{gpu}"}, wantStatus: 200, wantBackend: "hailo", wantModel: "qwen2:1.5b"},
		{name: "fast tier, on-prem gpu allowed for private", key: privateKey, model: "fast",
			register: &registry.Registration{Name: "gpu-a", URL: "{gpu}", OnPrem: true}, wantStatus: 200, wantBackend: "gpu", wantModel: "qwen2.5-coder:7b"},
		{name: "gpu wildcard in catalog", key: publicKey, model: "qwen2.5-coder:7b",
			register: &registry.Registration{Name: "gpu-a", URL: "{gpu}"}, wantStatus: 200, wantBackend: "gpu", wantModel: "qwen2.5-coder:7b"},
		{name: "openai endpoint routes by model", key: publicKey, path: "/v1/chat/completions", model: "hailo/qwen2:1.5b", wantStatus: 200, wantBackend: "hailo", wantModel: "qwen2:1.5b"},
		{name: "show endpoint routes by model", key: publicKey, path: "/api/show", model: "shared:1b", wantStatus: 200, wantBackend: "cloud", wantModel: "shared:1b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hailo, cloud, gpu := newFake(t), newFake(t), newFake(t)
			gpuURL := gpu.URL
			if tt.gpuDead {
				gpuURL = deadURL(t)
			}
			if tt.gpu500 {
				gpu.reply = func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
					http.Error(w, `{"error":"boom"}`, 500)
				}
			}
			g := newGateway(t, twoBackendCfg, map[string]string{"hailo": hailo.URL, "cloud": cloud.URL})
			if tt.hailoDown {
				b, _ := g.reg.Get("hailo")
				b.SetHealthy(false)
			}
			if tt.register != nil {
				reg := *tt.register
				reg.URL = gpuURL
				if resp, body := g.do(t, "POST", "/internal/backends", adminKey, reg, nil); resp.StatusCode != 200 {
					t.Fatalf("register: %d %s", resp.StatusCode, body)
				}
			}
			path := tt.path
			if path == "" {
				path = "/api/chat"
			}
			resp, body := g.do(t, "POST", path, tt.key, map[string]any{"model": tt.model, "messages": []any{}}, tt.header)
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status %d, want %d (%s)", resp.StatusCode, tt.wantStatus, body)
			}
			if tt.wantErr != "" && !strings.Contains(string(body), tt.wantErr) {
				t.Fatalf("body %s, want error containing %q", body, tt.wantErr)
			}
			got := map[string][]recorded{"hailo": hailo.requests(), "cloud": cloud.requests(), "gpu": gpu.requests()}
			for name, reqs := range got {
				served := 0
				for _, r := range reqs {
					if r.auth != "" {
						t.Errorf("%s received Authorization header %q", name, r.auth)
					}
					if name == tt.wantBackend {
						served++
						if r.body["model"] != tt.wantModel {
							t.Errorf("%s got model %v, want %s", name, r.body["model"], tt.wantModel)
						}
						if r.path != path {
							t.Errorf("%s got path %s, want %s", name, r.path, path)
						}
					}
				}
				switch {
				case name == tt.wantBackend && served != 1:
					t.Errorf("%s served %d requests, want 1", name, served)
				case name == "gpu" && tt.gpu500:
					// the failed attempt is expected
				case name != tt.wantBackend && len(reqs) != 0:
					t.Errorf("%s unexpectedly received %d requests", name, len(reqs))
				}
			}
			// Privacy invariant: a private request never touches off-prem.
			private := tt.key == privateKey || (tt.header != nil && tt.header[PrivateHeader] == "true")
			if private && len(cloud.requests()) > 0 {
				t.Fatal("private request reached off-prem backend")
			}
		})
	}
}

func TestLastCandidate5xxPassesThroughAndAllDead502(t *testing.T) {
	hailo, cloud := newFake(t), newFake(t)
	cloud.reply = func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		http.Error(w, `{"error":"cloud down"}`, 500)
	}
	hailo.reply = cloud.reply
	g := newGateway(t, twoBackendCfg, map[string]string{"hailo": hailo.URL, "cloud": cloud.URL})
	resp, body := g.do(t, "POST", "/api/chat", publicKey, map[string]any{"model": "shared:1b"}, nil)
	if resp.StatusCode != 500 || !strings.Contains(string(body), "cloud down") {
		t.Fatalf("got %d %s; want last backend's 500 passed through", resp.StatusCode, body)
	}
	if len(cloud.requests()) != 1 || len(hailo.requests()) != 1 {
		t.Fatalf("want one attempt per backend, got cloud=%d hailo=%d", len(cloud.requests()), len(hailo.requests()))
	}

	g2 := newGateway(t, twoBackendCfg, map[string]string{"hailo": deadURL(t), "cloud": deadURL(t)})
	resp, body = g2.do(t, "POST", "/api/chat", publicKey, map[string]any{"model": "shared:1b"}, nil)
	if resp.StatusCode != 502 || !strings.Contains(string(body), "all backends failed") {
		t.Fatalf("got %d %s; want 502", resp.StatusCode, body)
	}
}

func TestKeepAliveInjection(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		model   string
		body    string
		wantKA  any // nil = must be absent
		wantRaw bool
	}{
		{name: "chat injected", path: "/api/chat", model: "qwen2:1.5b", body: `{"model":"qwen2:1.5b","messages":[]}`, wantKA: float64(-1)},
		{name: "generate injected", path: "/api/generate", model: "qwen2:1.5b", body: `{"model":"qwen2:1.5b","prompt":"hi"}`, wantKA: float64(-1)},
		{name: "client value kept", path: "/api/chat", body: `{"model":"qwen2:1.5b","keep_alive":"5m"}`, wantKA: "5m", wantRaw: true},
		{name: "client zero kept", path: "/api/generate", body: `{"model":"qwen2:1.5b","keep_alive":0}`, wantKA: float64(0), wantRaw: true},
		{name: "show not injected", path: "/api/show", body: `{"model":"qwen2:1.5b"}`, wantRaw: true},
		{name: "openai not injected", path: "/v1/chat/completions", body: `{"model":"qwen2:1.5b","messages":[]}`, wantRaw: true},
		{name: "backend without keepAlive", path: "/api/chat", body: `{"model":"cloudonly:1b", "messages": [] }`, wantRaw: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hailo, cloud := newFake(t), newFake(t)
			g := newGateway(t, twoBackendCfg, map[string]string{"hailo": hailo.URL, "cloud": cloud.URL})
			resp, out := g.do(t, "POST", tt.path, publicKey, tt.body, nil)
			if resp.StatusCode != 200 {
				t.Fatalf("status %d %s", resp.StatusCode, out)
			}
			reqs := append(hailo.requests(), cloud.requests()...)
			if len(reqs) != 1 {
				t.Fatalf("want 1 upstream request, got %d", len(reqs))
			}
			ka, ok := reqs[0].body["keep_alive"]
			if tt.wantKA == nil && ok {
				t.Fatalf("keep_alive unexpectedly present: %v", ka)
			}
			if tt.wantKA != nil && ka != tt.wantKA {
				t.Fatalf("keep_alive = %#v, want %#v", ka, tt.wantKA)
			}
			if tt.wantRaw && string(reqs[0].raw) != tt.body {
				t.Fatalf("body altered: %s, want %s", reqs[0].raw, tt.body)
			}
		})
	}
}

func TestStreamingFlushedAndUnmodified(t *testing.T) {
	hailo, cloud := newFake(t), newFake(t)
	chunks := []string{
		`{"model":"qwen2:1.5b","message":{"role":"assistant","content":"Hel"},"done":false}` + "\n",
		`{"model":"qwen2:1.5b","message":{"role":"assistant","content":"lo"},"done":false}` + "\n",
		`{"model":"qwen2:1.5b","message":{"role":"assistant","content":""},"done":true,"eval_count":2,"eval_duration":500000000}` + "\n",
	}
	step := make(chan struct{})
	hailo.reply = func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		for i, c := range chunks {
			if i > 0 {
				<-step // only continue once the client has seen the previous chunk
			}
			io.WriteString(w, c)
			w.(http.Flusher).Flush()
		}
	}
	g := newGateway(t, twoBackendCfg, map[string]string{"hailo": hailo.URL, "cloud": cloud.URL})

	req, _ := http.NewRequest("POST", g.URL+"/api/chat", strings.NewReader(`{"model":"qwen2:1.5b","messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+publicKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("content-type %q", ct)
	}
	rd := bufio.NewReader(resp.Body)
	var got strings.Builder
	for i := range chunks {
		lineCh := make(chan string, 1)
		go func() {
			l, _ := rd.ReadString('\n')
			lineCh <- l
		}()
		select {
		case l := <-lineCh:
			if l != chunks[i] {
				t.Fatalf("chunk %d = %q, want %q", i, l, chunks[i])
			}
			got.WriteString(l)
		case <-time.After(3 * time.Second):
			t.Fatalf("chunk %d not flushed to client", i)
		}
		if i < len(chunks)-1 {
			step <- struct{}{}
		}
	}
	if rest, _ := io.ReadAll(rd); len(rest) != 0 {
		t.Fatalf("unexpected trailing bytes %q", rest)
	}
	if got.String() != strings.Join(chunks, "") {
		t.Fatal("stream altered")
	}
}

func TestDoneTracker(t *testing.T) {
	tests := []struct {
		name   string
		writes []string
		want   *doneStats
	}{
		{"split across writes", []string{`{"done":false}` + "\n" + `{"done":tr`, `ue,"eval_count":40,"eval_dura`, `tion":2000000000}` + "\n"}, &doneStats{Done: true, EvalCount: 40, EvalDuration: 2e9}},
		{"non-streamed single object without newline", []string{`{"model":"m","done":true,"eval_count":5,"eval_duration":1000000000}`}, &doneStats{Done: true, EvalCount: 5, EvalDuration: 1e9}},
		{"no done chunk", []string{`{"done":false}` + "\n"}, nil},
		{"openai body", []string{`{"choices":[]}`}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d doneTracker
			for _, w := range tt.writes {
				d.Write([]byte(w))
			}
			d.finish()
			if (d.final == nil) != (tt.want == nil) || (d.final != nil && *d.final != *tt.want) {
				t.Fatalf("got %+v, want %+v", d.final, tt.want)
			}
		})
	}
}

func TestQueueOverflow429(t *testing.T) {
	hailo, cloud := newFake(t), newFake(t)
	release := make(chan struct{})
	hailo.reply = func(w http.ResponseWriter, _ *http.Request, body map[string]any) {
		<-release
		json.NewEncoder(w).Encode(map[string]any{"done": true})
	}
	cfg := strings.Replace(twoBackendCfg, "queueDepth: 4", "queueDepth: 1", 1)
	g := newGateway(t, cfg, map[string]string{"hailo": hailo.URL, "cloud": cloud.URL})
	b, _ := g.reg.Get("hailo")

	statuses := make(chan int, 2)
	send := func() {
		resp, _ := g.do(t, "POST", "/api/chat", publicKey, map[string]any{"model": "qwen2:1.5b"}, nil)
		statuses <- resp.StatusCode
	}
	waitFor := func(active, waiting int) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if a, w := b.Limiter.Stats(); a == active && w == waiting {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		a, w := b.Limiter.Stats()
		t.Fatalf("limiter active=%d waiting=%d, want %d/%d", a, w, active, waiting)
	}
	go send()
	waitFor(1, 0)
	go send()
	waitFor(1, 1)

	resp, body := g.do(t, "POST", "/api/chat", publicKey, map[string]any{"model": "qwen2:1.5b"}, nil)
	if resp.StatusCode != 429 || !strings.Contains(string(body), "backend busy") {
		t.Fatalf("got %d %s, want 429 backend busy", resp.StatusCode, body)
	}
	close(release)
	for range 2 {
		if s := <-statuses; s != 200 {
			t.Fatalf("queued request got %d", s)
		}
	}
	waitFor(0, 0)
}

func TestBusyBackendFallsThroughToNextCandidate(t *testing.T) {
	hailo, cloud := newFake(t), newFake(t)
	cfg := strings.Replace(twoBackendCfg, "queueDepth: 4", "queueDepth: 1", 1)
	cfg = strings.Replace(cfg, "  fast: [gpu, hailo]", "  fast: [gpu, hailo]\n  onprem-first: [hailo, cloud]", 1)
	g := newGateway(t, cfg, map[string]string{"hailo": hailo.URL, "cloud": cloud.URL})
	b, _ := g.reg.Get("hailo")
	// Take hailo's only slot and fill its one-deep queue.
	if err := b.Limiter.Acquire(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer b.Limiter.Release()
	ctx, cancel := context.WithCancel(t.Context())
	queued := make(chan error, 1)
	go func() { queued <- b.Limiter.Acquire(ctx) }()
	for {
		if _, w := b.Limiter.Stats(); w == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	resp, body := g.do(t, "POST", "/api/chat", publicKey, map[string]any{"model": "onprem-first"}, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("got %d %s, want fallback to cloud", resp.StatusCode, body)
	}
	if len(hailo.requests()) != 0 || len(cloud.requests()) != 1 || cloud.requests()[0].body["model"] != "shared:1b" {
		t.Fatalf("hailo=%d cloud=%+v", len(hailo.requests()), cloud.requests())
	}
	// A plain model whose only backend is busy gets 429.
	resp, _ = g.do(t, "POST", "/api/chat", publicKey, map[string]any{"model": "qwen2:1.5b"}, nil)
	if resp.StatusCode != 429 {
		t.Fatalf("got %d, want 429", resp.StatusCode)
	}
	cancel()
	if err := <-queued; err == nil {
		b.Limiter.Release()
	}
}

func TestDynamicRegistrationAndTTL(t *testing.T) {
	hailo, cloud, gpu := newFake(t), newFake(t), newFake(t)
	g := newGateway(t, twoBackendCfg, map[string]string{"hailo": hailo.URL, "cloud": cloud.URL})
	now := time.Now()
	var mu sync.Mutex
	g.reg.SetClock(func() time.Time { mu.Lock(); defer mu.Unlock(); return now })
	advance := func(d time.Duration) { mu.Lock(); now = now.Add(d); mu.Unlock() }
	chat := func() int {
		resp, _ := g.do(t, "POST", "/api/chat", publicKey, map[string]any{"model": "qwen2.5-coder:7b"}, nil)
		return resp.StatusCode
	}
	reg := map[string]any{"name": "gpu-abc", "url": gpu.URL, "onPrem": false, "concurrency": 4}

	steps := []struct {
		name   string
		action func() (int, string)
		want   int
	}{
		{"not registered yet", func() (int, string) { return chat(), "" }, 503},
		{"register without admin key", func() (int, string) {
			r, b := g.do(t, "POST", "/internal/backends", "", reg, nil)
			return r.StatusCode, string(b)
		}, 401},
		{"register", func() (int, string) {
			r, b := g.do(t, "POST", "/internal/backends", adminKey, reg, nil)
			return r.StatusCode, string(b)
		}, 200},
		{"routes to gpu", func() (int, string) { return chat(), "" }, 200},
		{"static cannot be overwritten", func() (int, string) {
			r, b := g.do(t, "POST", "/internal/backends", adminKey, map[string]any{"name": "hailo", "url": gpu.URL}, nil)
			return r.StatusCode, string(b)
		}, 409},
		{"static cannot be deleted", func() (int, string) {
			r, b := g.do(t, "DELETE", "/internal/backends/hailo", adminKey, nil, nil)
			return r.StatusCode, string(b)
		}, 409},
		{"bad url rejected", func() (int, string) {
			r, b := g.do(t, "POST", "/internal/backends", adminKey, map[string]any{"name": "gpu-x", "url": "ftp://x"}, nil)
			return r.StatusCode, string(b)
		}, 400},
		{"heartbeat before expiry", func() (int, string) {
			advance(50 * time.Second)
			r, b := g.do(t, "POST", "/internal/backends", adminKey, reg, nil)
			return r.StatusCode, string(b)
		}, 200},
		{"still alive after original TTL", func() (int, string) { advance(50 * time.Second); return chat(), "" }, 200},
		{"expired without heartbeat", func() (int, string) { advance(11 * time.Second); return chat(), "" }, 503},
		{"re-register", func() (int, string) {
			r, b := g.do(t, "POST", "/internal/backends", adminKey, reg, nil)
			return r.StatusCode, string(b)
		}, 200},
		{"delete", func() (int, string) {
			r, b := g.do(t, "DELETE", "/internal/backends/gpu-abc", adminKey, nil, nil)
			return r.StatusCode, string(b)
		}, 204},
		{"gone after delete", func() (int, string) { return chat(), "" }, 503},
		{"delete unknown", func() (int, string) {
			r, b := g.do(t, "DELETE", "/internal/backends/gpu-abc", adminKey, nil, nil)
			return r.StatusCode, string(b)
		}, 404},
	}
	for _, s := range steps {
		if got, body := s.action(); got != s.want {
			t.Fatalf("%s: status %d, want %d %s", s.name, got, s.want, body)
		}
	}
	if n := len(gpu.requests()); n != 2 {
		t.Fatalf("gpu served %d chats, want 2", n)
	}
}

func TestTagsMerge(t *testing.T) {
	hailo, cloud, gpu := newFake(t), newFake(t), newFake(t)
	hailo.tags = []string{"qwen2:1.5b", "qwen2.5-coder:1.5b"}
	cloud.tags = []string{"mistral:7b"}
	gpu.tags = []string{"qwen2.5-coder:7b"}
	g := newGateway(t, twoBackendCfg, map[string]string{"hailo": hailo.URL, "cloud": cloud.URL})
	if r, b := g.do(t, "POST", "/internal/backends", adminKey, map[string]any{"name": "gpu-1", "url": gpu.URL}, nil); r.StatusCode != 200 {
		t.Fatalf("register %d %s", r.StatusCode, b)
	}
	names := func(key string, hdr map[string]string) []string {
		resp, body := g.do(t, "GET", "/api/tags", key, nil, hdr)
		if resp.StatusCode != 200 {
			t.Fatalf("tags %d %s", resp.StatusCode, body)
		}
		var out struct {
			Models []struct {
				Name  string `json:"name"`
				Model string `json:"model"`
			} `json:"models"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		var ns []string
		for _, m := range out.Models {
			if m.Name != m.Model {
				t.Fatalf("name %q != model %q", m.Name, m.Model)
			}
			ns = append(ns, m.Name)
		}
		return ns
	}
	catalog := []string{"qwen2:1.5b", "shared:1b", "cloudonly:1b", "qwen2.5-coder:7b"}

	tests := []struct {
		name  string
		key   string
		hdr   map[string]string
		setup func()
		want  []string
	}{
		{"public sees all healthy backends", publicKey, nil, nil,
			append(append([]string{}, catalog...), "hailo/qwen2:1.5b", "hailo/qwen2.5-coder:1.5b", "cloud/mistral:7b", "gpu-1/qwen2.5-coder:7b")},
		{"private sees only on-prem prefixed models", privateKey, nil, nil,
			append(append([]string{}, catalog...), "hailo/qwen2:1.5b", "hailo/qwen2.5-coder:1.5b")},
		{"unhealthy backend omitted", publicKey, nil, func() { b, _ := g.reg.Get("cloud"); b.SetHealthy(false) },
			append(append([]string{}, catalog...), "hailo/qwen2:1.5b", "hailo/qwen2.5-coder:1.5b", "gpu-1/qwen2.5-coder:7b")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			got := names(tt.key, tt.hdr)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("got %v\nwant %v", got, tt.want)
			}
		})
	}
}

func TestBadRequests(t *testing.T) {
	hailo, cloud := newFake(t), newFake(t)
	g := newGateway(t, twoBackendCfg, map[string]string{"hailo": hailo.URL, "cloud": cloud.URL})
	tests := []struct {
		name, path, body string
		want             int
	}{
		{"invalid json", "/api/chat", "{", 400},
		{"missing model", "/api/chat", `{"messages":[]}`, 400},
		{"show accepts legacy name field", "/api/show", `{"name":"hailo/qwen2:1.5b"}`, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := g.do(t, "POST", tt.path, publicKey, tt.body, nil)
			if resp.StatusCode != tt.want {
				t.Fatalf("status %d, want %d (%s)", resp.StatusCode, tt.want, body)
			}
		})
	}
	reqs := hailo.requests()
	if len(reqs) != 1 || reqs[0].body["name"] != "qwen2:1.5b" {
		t.Fatalf("legacy show request not rewritten: %+v", reqs)
	}
}

func TestDemandSignals(t *testing.T) {
	type call struct{ model, reason string }
	var mu sync.Mutex
	var calls []call
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]string
		json.NewDecoder(r.Body).Decode(&m)
		mu.Lock()
		calls = append(calls, call{m["model"], m["reason"]})
		mu.Unlock()
	}))
	defer sink.Close()
	hailo, cloud, gpu := newFake(t), newFake(t), newFake(t)
	dem := demand.New(sink.URL, 30*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	now := time.Now()
	dem.SetClock(func() time.Time { return now })
	g := newGatewayDemand(t, twoBackendCfg, map[string]string{"hailo": hailo.URL, "cloud": cloud.URL}, dem)
	chat := func(model string) int {
		resp, _ := g.do(t, "POST", "/api/chat", publicKey, map[string]any{"model": model}, nil)
		dem.Wait()
		return resp.StatusCode
	}

	steps := []struct {
		name   string
		before func()
		model  string
		status int
		want   []call // cumulative
	}{
		{name: "non-gpu model sends nothing", model: "qwen2:1.5b", status: 200},
		{name: "gpu model, none registered", model: "qwen2.5-coder:7b", status: 503,
			want: []call{{"qwen2.5-coder:7b", demand.ReasonNoGPU}}},
		{name: "repeat within 5s throttled", model: "qwen2.5-coder:7b", status: 503,
			want: []call{{"qwen2.5-coder:7b", demand.ReasonNoGPU}}},
		{name: "tier with gpu entry after 5s, served by hailo", before: func() { now = now.Add(5 * time.Second) }, model: "fast", status: 200,
			want: []call{{"qwen2.5-coder:7b", demand.ReasonNoGPU}, {"qwen2.5-coder:7b", demand.ReasonNoGPU}}},
		{name: "routed to gpu", before: func() {
			now = now.Add(5 * time.Second)
			if r, b := g.do(t, "POST", "/internal/backends", adminKey, map[string]any{"name": "gpu-1", "url": gpu.URL}, nil); r.StatusCode != 200 {
				t.Fatalf("register %d %s", r.StatusCode, b)
			}
		}, model: "fast", status: 200,
			want: []call{{"qwen2.5-coder:7b", demand.ReasonNoGPU}, {"qwen2.5-coder:7b", demand.ReasonNoGPU}, {"qwen2.5-coder:7b", demand.ReasonGPURequest}}},
		{name: "gpu request throttled within interval", before: func() { now = now.Add(29 * time.Second) }, model: "gpu-1/llama3:8b", status: 200,
			want: []call{{"qwen2.5-coder:7b", demand.ReasonNoGPU}, {"qwen2.5-coder:7b", demand.ReasonNoGPU}, {"qwen2.5-coder:7b", demand.ReasonGPURequest}}},
		{name: "gpu request after interval", before: func() { now = now.Add(time.Second) }, model: "gpu-1/llama3:8b", status: 200,
			want: []call{{"qwen2.5-coder:7b", demand.ReasonNoGPU}, {"qwen2.5-coder:7b", demand.ReasonNoGPU}, {"qwen2.5-coder:7b", demand.ReasonGPURequest}, {"llama3:8b", demand.ReasonGPURequest}}},
		{name: "unhealthy gpu counts as none", before: func() {
			now = now.Add(5 * time.Second)
			b, _ := g.reg.Get("gpu-1")
			b.SetHealthy(false)
		}, model: "qwen2.5-coder:7b", status: 503,
			want: []call{{"qwen2.5-coder:7b", demand.ReasonNoGPU}, {"qwen2.5-coder:7b", demand.ReasonNoGPU}, {"qwen2.5-coder:7b", demand.ReasonGPURequest}, {"llama3:8b", demand.ReasonGPURequest}, {"qwen2.5-coder:7b", demand.ReasonNoGPU}}},
	}
	for _, s := range steps {
		if s.before != nil {
			s.before()
		}
		if got := chat(s.model); got != s.status {
			t.Fatalf("%s: status %d, want %d", s.name, got, s.status)
		}
		mu.Lock()
		got := fmt.Sprint(calls)
		mu.Unlock()
		if got != fmt.Sprint(s.want) {
			t.Fatalf("%s: calls %s, want %s", s.name, got, fmt.Sprint(s.want))
		}
	}
}

func TestDemandEndpointDownDoesNotAffectRequests(t *testing.T) {
	hailo, cloud := newFake(t), newFake(t)
	dem := demand.New(deadURL(t), time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	g := newGatewayDemand(t, twoBackendCfg, map[string]string{"hailo": hailo.URL, "cloud": cloud.URL}, dem)
	if resp, body := g.do(t, "POST", "/api/chat", publicKey, map[string]any{"model": "fast"}, nil); resp.StatusCode != 200 {
		t.Fatalf("got %d %s", resp.StatusCode, body)
	}
	dem.Wait()
}
