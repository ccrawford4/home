// Package store persists benchmark runs and their individual requests.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrNotFound is returned when a run does not exist.
var ErrNotFound = errors.New("not found")

// Run statuses.
const (
	StatusQueued  = "queued"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// Run is one benchmark suite execution against one model.
type Run struct {
	ID         int64           `json:"id"`
	Model      string          `json:"model"`
	Status     string          `json:"status"`
	Progress   string          `json:"progress"`
	Config     json.RawMessage `json:"config"`
	Summary    json.RawMessage `json:"summary,omitempty"`
	Error      string          `json:"error,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
}

// Request is a single model call made during a run. Metric fields are nil
// when they could not be measured.
type Request struct {
	ID            int64           `json:"id"`
	RunID         int64           `json:"run_id"`
	Phase         string          `json:"phase"` // cold, warmup, warm, multiturn, tools
	Seq           int             `json:"seq"`
	Prompt        string          `json:"prompt"`
	Response      string          `json:"response"`
	HTTPStatus    int             `json:"http_status"`
	Error         string          `json:"error,omitempty"`
	ClientTTFTMs  *float64        `json:"client_ttft_ms"`
	ClientTTLTMs  *float64        `json:"client_ttlt_ms"`
	ServerTTFTMs  *float64        `json:"server_ttft_ms"`
	ServerTotalMs *float64        `json:"server_total_ms"`
	EvalCount     *int            `json:"eval_count"`
	DecodeTPS     *float64        `json:"decode_tps"`
	Passed        *bool           `json:"passed"`
	ToolCalls     json.RawMessage `json:"tool_calls,omitempty"`
	Chunks        json.RawMessage `json:"chunks,omitempty"`
	StartedAt     time.Time       `json:"started_at"`
}

// Store is the persistence layer.
type Store interface {
	CreateRun(ctx context.Context, r *Run) error
	UpdateRun(ctx context.Context, r *Run) error
	GetRun(ctx context.Context, id int64) (*Run, error)
	ListRuns(ctx context.Context, limit int) ([]Run, error)
	DeleteRun(ctx context.Context, id int64) error
	// RunsWithStatus returns runs in the given status, oldest first.
	RunsWithStatus(ctx context.Context, status string) ([]Run, error)
	AddRequest(ctx context.Context, q *Request) error
	ListRequests(ctx context.Context, runID int64) ([]Request, error)
	Close() error
}
