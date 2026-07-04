// Package sandbox manages sandbox lifecycle backed by Kubernetes Jobs.
package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	log "log/slog"
	k8s "sandbox-api/k8s"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	log        *slog.Logger
	tracer     trace.Tracer
}

type ManagerConfig struct {
	KubeConfig *k8s.KubeClientConfig
	Logger     *slog.Logger
}

// NewManager returns a new Manager.
func NewManager(config *ManagerConfig) (*Manager, error) {
	kubeClient, err := k8s.NewKubeClient(config.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	return &Manager{sandboxes: make(map[string]*Sandbox), kubeclient: kubeClient,
		log: config.Logger, tracer: otel.Tracer("sandbox-manager"),
	}, nil
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
	ctx, span := m.tracer.Start(ctx, "Manager.Create")
	defer span.End()

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

	m.mu.Lock()
	m.sandboxes[id] = sb
	m.mu.Unlock()

	m.log.InfoContext(ctx, "sandbox created", slog.String("sandbox_id", id))
	span.SetAttributes(attribute.String("sandbox.id", id))
	return sb, nil
}

// Get returns the sandbox with the given ID.
func (m *Manager) Get(ctx context.Context, id string) (*Sandbox, error) {
	ctx, span := m.tracer.Start(ctx, "Manager.Get",
		trace.WithAttributes(attribute.String("sandbox.id", id)),
	)
	defer span.End()

	m.mu.RLock()
	sb, ok := m.sandboxes[id]
	m.mu.RUnlock()
	if !ok {
		m.log.WarnContext(ctx, "sandbox not found", slog.String("sandbox_id", id))
		return nil, fmt.Errorf("sandbox %q not found", id)
	}
	m.log.InfoContext(ctx, "sandbox retrieved", slog.String("sandbox_id", id))
	return sb, nil
}

// Delete removes the sandbox with the given ID.
func (m *Manager) Delete(ctx context.Context, id string) error {
	ctx, span := m.tracer.Start(ctx, "Manager.Delete",
		trace.WithAttributes(attribute.String("sandbox.id", id)),
	)
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sandboxes[id]; !ok {
		m.log.WarnContext(ctx, "sandbox not found", slog.String("sandbox_id", id))
		return fmt.Errorf("sandbox %q not found", id)
	}
	delete(m.sandboxes, id)
	m.log.InfoContext(ctx, "sandbox deleted", slog.String("sandbox_id", id))
	return nil
}

// List returns all sandboxes.
func (m *Manager) List(ctx context.Context) ([]*Sandbox, error) {
	ctx, span := m.tracer.Start(ctx, "Manager.List")
	defer span.End()

	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Sandbox, 0, len(m.sandboxes))
	for _, sb := range m.sandboxes {
		out = append(out, sb)
	}
	m.log.InfoContext(ctx, "sandboxes listed", slog.Int("count", len(out)))
	span.SetAttributes(attribute.Int("sandbox.count", len(out)))
	return out, nil
}
