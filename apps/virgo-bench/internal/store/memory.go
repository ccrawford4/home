package store

import (
	"context"
	"sort"
	"sync"
)

// Memory is an in-process Store for local development and tests.
type Memory struct {
	mu       sync.Mutex
	runs     map[int64]Run
	requests map[int64][]Request
	nextRun  int64
	nextReq  int64
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{runs: map[int64]Run{}, requests: map[int64][]Request{}}
}

func (m *Memory) CreateRun(_ context.Context, r *Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextRun++
	r.ID = m.nextRun
	m.runs[r.ID] = *r
	return nil
}

func (m *Memory) UpdateRun(_ context.Context, r *Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.runs[r.ID]; !ok {
		return ErrNotFound
	}
	m.runs[r.ID] = *r
	return nil
}

func (m *Memory) GetRun(_ context.Context, id int64) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &r, nil
}

func (m *Memory) ListRuns(_ context.Context, limit int) ([]Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Run, 0, len(m.runs))
	for _, r := range m.runs {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *Memory) DeleteRun(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.runs[id]; !ok {
		return ErrNotFound
	}
	delete(m.runs, id)
	delete(m.requests, id)
	return nil
}

func (m *Memory) RunsWithStatus(_ context.Context, status string) ([]Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Run
	for _, r := range m.runs {
		if r.Status == status {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *Memory) AddRequest(_ context.Context, q *Request) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextReq++
	q.ID = m.nextReq
	m.requests[q.RunID] = append(m.requests[q.RunID], *q)
	return nil
}

func (m *Memory) ListRequests(_ context.Context, runID int64) ([]Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Request(nil), m.requests[runID]...), nil
}

func (m *Memory) Close() error { return nil }
