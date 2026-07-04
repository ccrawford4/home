package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Sandbox represents a sandbox environment returned by the API.
type Sandbox struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// Client is an HTTP client for the Sandbox API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new Client targeting the given base URL.
func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, httpClient: &http.Client{}}
}

// Create sends POST /sandbox/create and returns the new sandbox.
func (c *Client) Create() (*Sandbox, error) {
	resp, err := c.httpClient.Post(c.baseURL+"/sandbox/create", "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var s Sandbox
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Get sends GET /sandbox/<id> and returns the sandbox.
func (c *Client) Get(id string) (*Sandbox, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/sandbox/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("sandbox %q not found", id)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var s Sandbox
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Delete sends DELETE /sandbox/<id>.
func (c *Client) Delete(id string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/sandbox/"+id, nil)
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

// List sends GET /sandbox/list and returns all sandboxes.
func (c *Client) List() ([]Sandbox, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/sandbox/list")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var sandboxes []Sandbox
	if err := json.NewDecoder(resp.Body).Decode(&sandboxes); err != nil {
		return nil, err
	}
	return sandboxes, nil
}
