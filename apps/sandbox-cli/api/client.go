// Package api provides a client for the sandbox API.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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
}

// NewClient returns a new Client targeting baseURL.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Create calls POST /sandbox/create and returns the new sandbox.
func (c *Client) Create(ctx context.Context) (*Sandbox, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/sandbox/create", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var sb Sandbox
	return &sb, json.NewDecoder(resp.Body).Decode(&sb)
}

// Get calls GET /sandbox/{id} and returns the sandbox.
func (c *Client) Get(ctx context.Context, id string) (*Sandbox, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/sandbox/"+id, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var sb Sandbox
	return &sb, json.NewDecoder(resp.Body).Decode(&sb)
}

// Delete calls DELETE /sandbox/{id}.
func (c *Client) Delete(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.BaseURL+"/sandbox/"+id, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/sandbox/list", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
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
