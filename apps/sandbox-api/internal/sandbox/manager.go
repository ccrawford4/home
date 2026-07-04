package sandbox

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// Status represents the lifecycle state of a sandbox.
type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusComplete Status = "complete"
)

// Sandbox holds metadata for a single sandbox environment.
type Sandbox struct {
	ID     string `json:"id"`
	Status Status `json:"status"`
}

// Manager maintains the in-memory collection of sandboxes.
// In V1 this will be replaced by Kubernetes Job orchestration.
type Manager struct {
	mu       sync.RWMutex
	sandboxes map[string]*Sandbox
}

// NewManager returns a ready-to-use Manager.
func NewManager() *Manager {
	return &Manager{sandboxes: make(map[string]*Sandbox)}
}

// Create allocates a new sandbox and returns it.
func (m *Manager) Create() *Sandbox {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := &Sandbox{ID: uuid.NewString(), Status: StatusPending}
	m.sandboxes[s.ID] = s
	return s
}

// Get returns the sandbox with the given ID.
func (m *Manager) Get(id string) (*Sandbox, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sandboxes[id]
	if !ok {
		return nil, fmt.Errorf("sandbox %q not found", id)
	}
	return s, nil
}

// Delete removes the sandbox with the given ID.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sandboxes[id]; !ok {
		return fmt.Errorf("sandbox %q not found", id)
	}
	delete(m.sandboxes, id)
	return nil
}

// List returns all sandboxes.
func (m *Manager) List() []*Sandbox {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Sandbox, 0, len(m.sandboxes))
	for _, s := range m.sandboxes {
		result = append(result, s)
	}
	return result
}
