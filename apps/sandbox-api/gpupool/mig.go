package gpupool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"

	"golang.org/x/oauth2/google"
)

const computeScope = "https://www.googleapis.com/auth/compute"

// MIGState is the part of a managed instance group the scaler reads.
type MIGState struct {
	TargetSize int
	// Stable is false while GCE is still creating, deleting or recreating
	// instances in the group.
	Stable bool
}

// MIG reads and resizes a managed instance group.
type MIG interface {
	Get(ctx context.Context) (MIGState, error)
	Resize(ctx context.Context, size int) error
}

// GCEMIG talks to the Compute Engine instanceGroupManagers API.
type GCEMIG struct {
	client  *http.Client
	baseURL string
}

// NewGCEMIG returns a client for the zonal group cfg.MIG. It authenticates
// with Application Default Credentials, which on GKE or with workload
// identity federation resolve to the pod's GCP service account.
func NewGCEMIG(ctx context.Context, cfg Config) (*GCEMIG, error) {
	client, err := google.DefaultClient(ctx, computeScope)
	if err != nil {
		return nil, fmt.Errorf("loading GCP credentials: %w", err)
	}
	return newGCEMIG(client, "https://compute.googleapis.com", cfg), nil
}

func newGCEMIG(client *http.Client, endpoint string, cfg Config) *GCEMIG {
	return &GCEMIG{
		client: client,
		baseURL: fmt.Sprintf("%s/compute/v1/projects/%s/zones/%s/instanceGroupManagers/%s",
			endpoint, url.PathEscape(cfg.Project), url.PathEscape(cfg.Zone), url.PathEscape(cfg.MIG)),
	}
}

// Get returns the group's target size and whether it is stable.
func (m *GCEMIG) Get(ctx context.Context) (MIGState, error) {
	var body struct {
		TargetSize int `json:"targetSize"`
		Status     struct {
			IsStable bool `json:"isStable"`
		} `json:"status"`
	}
	if err := m.do(ctx, http.MethodGet, m.baseURL, &body); err != nil {
		return MIGState{}, err
	}
	return MIGState{TargetSize: body.TargetSize, Stable: body.Status.IsStable}, nil
}

// Resize sets the group's target size. It returns once GCE has accepted the
// operation; the next Get shows the new target size.
func (m *GCEMIG) Resize(ctx context.Context, size int) error {
	return m.do(ctx, http.MethodPost, fmt.Sprintf("%s/resize?size=%d", m.baseURL, size), nil)
}

func (m *GCEMIG) do(ctx context.Context, method, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s %s: %s: %s", method, u, resp.Status, msg)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// dryRunMIG keeps the group's size in memory and logs resizes.
type dryRunMIG struct {
	log  *slog.Logger
	mu   sync.Mutex
	size int
}

// NewDryRunMIG returns a MIG that starts at size 0 and never calls GCE.
func NewDryRunMIG(log *slog.Logger) MIG {
	return &dryRunMIG{log: log}
}

func (m *dryRunMIG) Get(context.Context) (MIGState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return MIGState{TargetSize: m.size, Stable: true}, nil
}

func (m *dryRunMIG) Resize(_ context.Context, size int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.log.Info("dry run: would resize gpu pool", slog.Int("from", m.size), slog.Int("to", size))
	m.size = size
	return nil
}
