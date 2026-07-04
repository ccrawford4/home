// Package api provides a client for the sandbox API.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Sandbox represents a sandbox returned by the API.
type Sandbox struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Client is a client for the sandbox API.
type Client struct {
	BaseURL    string
	httpClient *http.Client
	log        *slog.Logger
	propagator propagation.TextMapPropagator
}

// NewClient returns a new Client targeting baseURL.
func NewClient(log *slog.Logger, baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		log:        log,
		propagator: otel.GetTextMapPropagator(),
	}
}

// Create calls POST /sandbox/create and returns the new sandbox.
func (c *Client) Create(ctx context.Context) (*Sandbox, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/sandbox/create", nil)
	if err != nil {
		return nil, err
	}
	return c.doSandbox(req)
}

// Get calls GET /sandbox/{id} and returns the sandbox.
func (c *Client) Get(ctx context.Context, id string) (*Sandbox, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/sandbox/"+id, nil)
	if err != nil {
		return nil, err
	}
	return c.doSandbox(req)
}

// Delete calls DELETE /sandbox/{id}.
func (c *Client) Delete(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, "/sandbox/"+id, nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// List calls GET /sandbox/list and returns all sandboxes.
func (c *Client) List(ctx context.Context) ([]*Sandbox, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/sandbox/list", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var list []*Sandbox
	return list, json.NewDecoder(resp.Body).Decode(&list)
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
	c.log.Debug("outgoing request", slog.String("method", method), slog.String("path", path))
	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	c.log.Debug("response received",
		slog.String("method", req.Method),
		slog.String("path", req.URL.Path),
		slog.Int("status", resp.StatusCode),
		slog.Duration("duration", time.Since(start)),
	)
	return resp, nil
}

func (c *Client) doSandbox(req *http.Request) (*Sandbox, error) {
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var sb Sandbox
	return &sb, json.NewDecoder(resp.Body).Decode(&sb)
}
