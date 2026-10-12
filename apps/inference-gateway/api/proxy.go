package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"inference-gateway/internal/demand"
	"inference-gateway/internal/limiter"
	"inference-gateway/internal/router"
)

// maxBodyBytes bounds request bodies (chat requests may carry base64 images).
const maxBodyBytes = 64 << 20

// hopHeaders are not forwarded in either direction.
var hopHeaders = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

// Proxy handles the model-routed endpoints: POST /api/chat, /api/generate,
// /api/show and /v1/chat/completions.
func (h *Handler) Proxy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := clientFrom(ctx)

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read body: "+err.Error())
		return
	}
	var payload map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	modelField := "model"
	model, _ := payload["model"].(string)
	if model == "" && r.URL.Path == "/api/show" {
		// Older Ollama clients send {"name": ...} to /api/show.
		modelField = "name"
		model, _ = payload["name"].(string)
	}
	if model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}
	info := infoFrom(ctx)
	info.set(func(i *reqInfo) { i.model = model })

	// Ask the scaler for a GPU node when this model could use one but none is
	// up. Fire-and-forget; the request carries on with what's available.
	if gm, ok := h.router.GPUModel(model); ok && !h.router.GPUAvailable() {
		h.demand.Signal(gm, demand.ReasonNoGPU)
	}

	cands, err := h.router.Resolve(model, isPrivate(client, r))
	if err != nil {
		var nb *router.NoBackendError
		if errors.As(err, &nb) {
			writeError(w, http.StatusServiceUnavailable, nb.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	busy := false
	for i, c := range cands {
		last := i == len(cands)-1
		if err := c.Backend.Limiter.Acquire(ctx); err != nil {
			if errors.Is(err, limiter.ErrBusy) {
				h.log.Warn("backend busy", "backend", c.Backend.Name, "model", c.Model, "client", client.Name)
				busy = true
				continue
			}
			return // client went away while queued
		}
		done := h.attempt(w, r, c, payload, body, modelField, last)
		c.Backend.Limiter.Release()
		if done {
			return
		}
	}
	if busy {
		writeError(w, http.StatusTooManyRequests, "backend busy")
		return
	}
	writeError(w, http.StatusBadGateway, fmt.Sprintf("all backends failed for model %s", model))
}

// attempt sends the request to one candidate. It returns false when the
// caller should try the next candidate: the connection failed, or the
// backend answered 5xx and this isn't the last candidate. Nothing has been
// written to w in that case.
func (h *Handler) attempt(w http.ResponseWriter, r *http.Request, c router.Candidate, payload map[string]any, body []byte, modelField string, last bool) bool {
	ctx := r.Context()
	b := c.Backend
	client := clientFrom(ctx)
	log := h.log.With("backend", b.Name, "model", c.Model, "client", client.Name)

	if b.IsGPU() {
		h.demand.Signal(c.Model, demand.ReasonGPURequest)
	}

	out, err := upstreamBody(r.URL.Path, c, payload, body, modelField)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return true
	}
	target := b.URL + r.URL.Path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, target, bytes.NewReader(out))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return true
	}
	copyHeaders(req.Header, r.Header)
	req.Header.Del("Authorization") // the client's gateway key never leaves the gateway
	req.Header.Del(PrivateHeader)
	req.Header.Del("Content-Length")
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.upstream.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return true // client gone
		}
		log.Warn("backend request failed", "error", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 && !last {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		log.Warn("backend returned server error; trying next", "status", resp.StatusCode, "body", strings.TrimSpace(string(snippet)))
		return false
	}

	infoFrom(ctx).set(func(i *reqInfo) { i.backend = b.Name })
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	flusher, _ := w.(http.Flusher)
	stats := &doneTracker{}
	buf := make([]byte, 32<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			stats.Write(buf[:n])
			if _, werr := w.Write(buf[:n]); werr != nil {
				log.Warn("client write failed", "error", werr)
				return true
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			if ctx.Err() == nil {
				log.Warn("backend stream error", "error", rerr)
			}
			return true
		}
	}
	stats.finish()
	if f := stats.final; f != nil {
		attrs := []any{"status", resp.StatusCode, "eval_count", f.EvalCount, "eval_duration", f.EvalDuration}
		if f.EvalDuration > 0 {
			attrs = append(attrs, "tok_s", float64(f.EvalCount)/(float64(f.EvalDuration)/1e9))
		}
		log.Info("inference complete", attrs...)
	}
	return true
}

// upstreamBody returns the body to send a candidate: the original bytes
// unless the model name changes (prefix/tier) or keep_alive is injected.
func upstreamBody(path string, c router.Candidate, payload map[string]any, body []byte, modelField string) ([]byte, error) {
	setModel := payload[modelField] != c.Model
	_, hasKA := payload["keep_alive"]
	injectKA := c.Backend.KeepAlive != nil && !hasKA && (path == "/api/chat" || path == "/api/generate")
	if !setModel && !injectKA {
		return body, nil
	}
	p := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		p[k] = v
	}
	if setModel {
		p[modelField] = c.Model
	}
	if injectKA {
		p["keep_alive"] = c.Backend.KeepAlive
	}
	return json.Marshal(p)
}

func copyHeaders(dst, src http.Header) {
	for k, vs := range src {
		dst[k] = append([]string(nil), vs...)
	}
	for _, k := range hopHeaders {
		dst.Del(k)
	}
}

// doneStats are the fields of Ollama's final (done:true) chunk we log.
type doneStats struct {
	Done         bool  `json:"done"`
	EvalCount    int64 `json:"eval_count"`
	EvalDuration int64 `json:"eval_duration"`
}

// doneTracker watches an NDJSON (or single JSON) response for the done:true
// object without altering it.
type doneTracker struct {
	line     []byte
	overflow bool
	final    *doneStats
}

const maxTrackedLine = 1 << 20

func (t *doneTracker) Write(p []byte) {
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			t.append(p)
			return
		}
		t.append(p[:i])
		t.check()
		p = p[i+1:]
	}
}

func (t *doneTracker) append(p []byte) {
	if t.overflow {
		return
	}
	if len(t.line)+len(p) > maxTrackedLine {
		t.overflow = true
		t.line = t.line[:0]
		return
	}
	t.line = append(t.line, p...)
}

func (t *doneTracker) check() {
	line := t.line
	t.line = t.line[:0]
	if t.overflow {
		t.overflow = false
		return
	}
	if !bytes.Contains(line, []byte(`"done"`)) {
		return
	}
	var s doneStats
	if json.Unmarshal(bytes.TrimSpace(line), &s) == nil && s.Done {
		t.final = &s
	}
}

func (t *doneTracker) finish() {
	if len(t.line) > 0 || t.overflow {
		t.check()
	}
}
