// Package ollama is a minimal client for the Ollama API as served by
// hailo-ollama (virgo). It records client-side receive timestamps for every
// streamed chunk so latency metrics can be computed (and recomputed) later.
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to an Ollama-compatible server.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a client for baseURL (e.g. http://virgo.virgo.svc.cluster.local:8000).
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: timeout},
	}
}

// Message is a chat message.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall is a function call returned by the model.
type ToolCall struct {
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

// Tool is a function definition offered to the model.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a callable function using JSON schema parameters.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatRequest is the body of POST /api/chat.
type ChatRequest struct {
	Model     string         `json:"model"`
	Messages  []Message      `json:"messages"`
	Stream    bool           `json:"stream"`
	Tools     []Tool         `json:"tools,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
	KeepAlive any            `json:"keep_alive,omitempty"`
}

// chunk is one NDJSON line (or the whole body when not streaming).
type chunk struct {
	CreatedAt          string   `json:"created_at"`
	Message            *Message `json:"message"`
	Response           string   `json:"response"`
	Done               bool     `json:"done"`
	TotalDuration      int64    `json:"total_duration"`
	LoadDuration       int64    `json:"load_duration"`
	PromptEvalCount    int      `json:"prompt_eval_count"`
	PromptEvalDuration int64    `json:"prompt_eval_duration"`
	EvalCount          int      `json:"eval_count"`
	EvalDuration       int64    `json:"eval_duration"`
	Error              string   `json:"error"`
}

// Chunk is a received chunk with its client-side receive offset.
type Chunk struct {
	ClientMs  float64 `json:"client_ms"`            // ms since the request was sent
	CreatedAt string  `json:"created_at,omitempty"` // server timestamp, as sent
	Text      string  `json:"text,omitempty"`
	ToolCall  bool    `json:"tool_call,omitempty"`
	Done      bool    `json:"done,omitempty"`
}

// Stats are the server-reported counters from the final chunk. Zero means
// the server did not report it (hailo-ollama 0.5.1 only sends
// total_duration and eval_count).
type Stats struct {
	TotalDuration      time.Duration `json:"total_duration"`
	LoadDuration       time.Duration `json:"load_duration"`
	PromptEvalCount    int           `json:"prompt_eval_count"`
	PromptEvalDuration time.Duration `json:"prompt_eval_duration"`
	EvalCount          int           `json:"eval_count"`
	EvalDuration       time.Duration `json:"eval_duration"`
}

// Result is everything observed for one request.
type Result struct {
	StartedAt     time.Time
	HTTPStatus    int
	ClientTotalMs float64
	Chunks        []Chunk
	Text          string
	ToolCalls     []ToolCall
	Stats         Stats
	Error         string
}

// Tags lists the model names the server has.
func (c *Client) Tags(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GET /api/tags: HTTP %d: %s", resp.StatusCode, b)
	}
	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(body.Models))
	for _, m := range body.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

// Unload asks the server to evict the model (keep_alive: 0) so the next
// request is a cold start.
func (c *Client) Unload(ctx context.Context, model string) error {
	body, _ := json.Marshal(map[string]any{"model": model, "keep_alive": 0, "stream": false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unload: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Chat sends a chat request. With r.Stream set, every NDJSON line is
// timestamped on receipt. Transport and HTTP errors are reported in
// Result.Error rather than returned, so a failed request is still recorded.
func (c *Client) Chat(ctx context.Context, r ChatRequest) *Result {
	res := &Result{}
	body, err := json.Marshal(r)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		res.Error = err.Error()
		return res
	}
	req.Header.Set("Content-Type", "application/json")

	res.StartedAt = time.Now()
	since := func() float64 { return float64(time.Since(res.StartedAt).Microseconds()) / 1000 }
	defer func() { res.ClientTotalMs = since() }()

	resp, err := c.HTTP.Do(req)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()
	res.HTTPStatus = resp.StatusCode

	var text strings.Builder
	handle := func(line []byte) {
		t := since()
		var ch chunk
		if err := json.Unmarshal(line, &ch); err != nil {
			if res.Error == "" {
				res.Error = "unparseable response: " + truncate(string(line), 200)
			}
			return
		}
		if ch.Error != "" {
			res.Error = ch.Error
		}
		ck := Chunk{ClientMs: t, CreatedAt: ch.CreatedAt, Done: ch.Done}
		if ch.Message != nil {
			ck.Text = ch.Message.Content
			if len(ch.Message.ToolCalls) > 0 {
				ck.ToolCall = true
				res.ToolCalls = append(res.ToolCalls, ch.Message.ToolCalls...)
			}
		} else {
			ck.Text = ch.Response
		}
		text.WriteString(ck.Text)
		res.Chunks = append(res.Chunks, ck)
		if ch.Done {
			res.Stats = Stats{
				TotalDuration:      time.Duration(ch.TotalDuration),
				LoadDuration:       time.Duration(ch.LoadDuration),
				PromptEvalCount:    ch.PromptEvalCount,
				PromptEvalDuration: time.Duration(ch.PromptEvalDuration),
				EvalCount:          ch.EvalCount,
				EvalDuration:       time.Duration(ch.EvalDuration),
			}
		}
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		handle(bytes.TrimSpace(b))
		if res.Error == "" {
			res.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(b), 200))
		}
		res.Text = text.String()
		return res
	}

	br := bufio.NewReaderSize(resp.Body, 64*1024)
	for {
		line, err := br.ReadBytes('\n')
		if l := bytes.TrimSpace(line); len(l) > 0 {
			handle(l)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && res.Error == "" {
				res.Error = err.Error()
			}
			break
		}
	}
	res.Text = text.String()
	return res
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
