// Package sandbox manages sandbox lifecycle backed by Kubernetes Jobs.
package sandbox

import (
	"context"
	"fmt"
	log "log/slog"
	k8s "sandbox-api/k8s"
	"sync"
	"time"
)

// Sandbox represents a running sandbox environment.
type Sandbox struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Manager handles creating, retrieving, and deleting sandboxes.
type Manager struct {
	mu         sync.RWMutex
	sandboxes  map[string]*Sandbox
	kubeclient *k8s.KubeClient
}

type ManagerConfig struct {
	KubeConfig *k8s.KubeClientConfig
}

// NewManager returns a new Manager.
func NewManager(config *ManagerConfig) (*Manager, error) {
	kubeClient, err := k8s.NewKubeClient(config.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	return &Manager{sandboxes: make(map[string]*Sandbox), kubeclient: kubeClient}, nil
}

func (m *Manager) launchSandbox(id string, sb *Sandbox) error {
	// Lock the manager while we add the sandbox and kick off the corresponding job
	m.mu.Lock()
	m.sandboxes[id] = sb

	// Launch the k8s job
	job, err := m.kubeclient.CreateJob(&k8s.JobLaunchConfig{
		Name:      id,
		Namespace: "default",
	})

	// Debug log
	log.Debug("Launched job", "job", job, "sandbox_id", id, "error", err)

	// Unlock the mutex
	m.mu.Unlock()
	return err
}

// Create starts a new sandbox and returns it.
func (m *Manager) Create(ctx context.Context) (*Sandbox, error) {
	id := fmt.Sprintf("sb-%d", time.Now().UnixNano())
	sb := &Sandbox{
		ID:        id,
		Status:    "running",
		CreatedAt: time.Now().UTC(),
	}

	// Attempt to launch the sandbox
	if err := m.launchSandbox(id, sb); err != nil {
		return nil, fmt.Errorf("failed to launch sandbox: %w", err)
	}

	return sb, nil
}

// Get returns the sandbox with the given ID.
func (m *Manager) Get(ctx context.Context, id string) (*Sandbox, error) {
	m.mu.RLock()
	sb, ok := m.sandboxes[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("sandbox %q not found", id)
	}
	return sb, nil
}

// Delete removes the sandbox with the given ID.
func (m *Manager) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sandboxes[id]; !ok {
		return fmt.Errorf("sandbox %q not found", id)
	}
	delete(m.sandboxes, id)
	return nil
}

// List returns all sandboxes.
func (m *Manager) List(ctx context.Context) ([]*Sandbox, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Sandbox, 0, len(m.sandboxes))
	for _, sb := range m.sandboxes {
		out = append(out, sb)
	}
	return out, nil
}
