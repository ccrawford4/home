package gpupool

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Status is the scaler's view of the pool after its last reconcile.
type Status struct {
	TargetSize     int       `json:"target_size"`
	MaxSize        int       `json:"max_size"`
	Stable         bool      `json:"stable"`
	PendingGPUPods int       `json:"pending_gpu_pods"`
	GPUPods        int       `json:"gpu_pods"`
	GPUNodes       []Node    `json:"gpu_nodes"`
	LastActivity   time.Time `json:"last_activity"`
	IdleFor        string    `json:"idle_for"`
	DryRun         bool      `json:"dry_run"`
	LastSync       time.Time `json:"last_sync"`
	LastError      string    `json:"last_error,omitempty"`
}

// Controller keeps the GPU pool between 0 and cfg.MaxSize nodes.
//
// It scales up when a pod requesting a GPU is pending or Demand is called,
// and scales down to 0 once no GPU pods have run and no demand has arrived
// for cfg.IdleTimeout. Inference served by the per-node Ollama DaemonSet is
// only visible through Demand, so the gateway reports every GPU request.
type Controller struct {
	cfg     Config
	mig     MIG
	cluster Cluster
	log     *slog.Logger
	now     func() time.Time
	kick    chan struct{}

	mu            sync.Mutex
	lastActivity  time.Time
	demandPending bool
	status        Status
}

// NewController returns a controller. The idle clock starts now, so a
// restart never scales down a node that was in use.
func NewController(cfg Config, mig MIG, cluster Cluster, log *slog.Logger) *Controller {
	c := &Controller{
		cfg:     cfg,
		mig:     mig,
		cluster: cluster,
		log:     log.With(slog.String("component", "gpu-pool")),
		now:     time.Now,
		kick:    make(chan struct{}, 1),
	}
	c.lastActivity = c.now()
	c.status = Status{MaxSize: cfg.MaxSize, DryRun: cfg.DryRun, LastActivity: c.lastActivity}
	return c
}

// Demand records that a client needs a GPU now and wakes the controller.
func (c *Controller) Demand(model, reason string) Status {
	c.mu.Lock()
	c.lastActivity = c.now()
	c.demandPending = true
	c.mu.Unlock()

	c.log.Info("gpu demand", slog.String("model", model), slog.String("reason", reason))
	select {
	case c.kick <- struct{}{}:
	default:
	}
	return c.Status()
}

// Status returns the state recorded by the last reconcile.
func (c *Controller) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.status
	st.LastActivity = c.lastActivity
	st.IdleFor = c.now().Sub(c.lastActivity).Truncate(time.Second).String()
	return st
}

// Run reconciles every cfg.SyncInterval and on each Demand until ctx ends.
func (c *Controller) Run(ctx context.Context) {
	c.log.Info("gpu pool scaler started",
		slog.String("mig", c.cfg.MIG), slog.String("zone", c.cfg.Zone), slog.String("project", c.cfg.Project),
		slog.Int("max_size", c.cfg.MaxSize), slog.Duration("idle_timeout", c.cfg.IdleTimeout),
		slog.Bool("dry_run", c.cfg.DryRun))

	ticker := time.NewTicker(c.cfg.SyncInterval)
	defer ticker.Stop()
	for {
		if err := c.Reconcile(ctx); err != nil && ctx.Err() == nil {
			c.log.Error("gpu pool reconcile failed", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-c.kick:
		}
	}
}

// Reconcile runs one pass: read the cluster and the group, then resize.
func (c *Controller) Reconcile(ctx context.Context) error {
	err := c.reconcile(ctx)
	c.mu.Lock()
	c.status.LastSync = c.now()
	c.status.LastError = ""
	if err != nil {
		c.status.LastError = err.Error()
	}
	c.mu.Unlock()
	return err
}

func (c *Controller) reconcile(ctx context.Context) error {
	snap, err := c.cluster.Snapshot(ctx)
	if err != nil {
		return err
	}
	state, err := c.mig.Get(ctx)
	if err != nil {
		return fmt.Errorf("reading gpu pool: %w", err)
	}

	now := c.now()
	c.mu.Lock()
	demand := c.demandPending
	c.demandPending = false
	if snap.PendingGPUPods > 0 || snap.GPUPods > 0 {
		c.lastActivity = now
	}
	lastActivity := c.lastActivity
	c.status.TargetSize = state.TargetSize
	c.status.Stable = state.Stable
	c.status.PendingGPUPods = snap.PendingGPUPods
	c.status.GPUPods = snap.GPUPods
	c.status.GPUNodes = snap.Nodes
	c.mu.Unlock()

	busy := demand || snap.PendingGPUPods > 0 || snap.GPUPods > 0
	if busy {
		c.uncordonOurs(ctx, snap.Nodes)
	}

	if want := c.desiredSize(state, snap, demand); want > state.TargetSize {
		c.log.Info("scaling gpu pool up", slog.Int("from", state.TargetSize), slog.Int("to", want),
			slog.Int("pending_gpu_pods", snap.PendingGPUPods), slog.Bool("demand", demand))
		if err := c.mig.Resize(ctx, want); err != nil {
			c.requeueDemand(demand)
			return fmt.Errorf("scaling gpu pool to %d: %w", want, err)
		}
		c.setTarget(want)
		return nil
	}

	if state.TargetSize == 0 {
		return c.removeStaleNodes(ctx, snap.Nodes)
	}

	if busy || !state.Stable || now.Sub(lastActivity) < c.cfg.IdleTimeout {
		return nil
	}
	return c.scaleDown(ctx, snap.Nodes, lastActivity, now.Sub(lastActivity))
}

// desiredSize is 1 when anything needs a GPU and the pool is empty. With a
// larger MaxSize it adds one node at a time while GPU pods stay pending and
// every node already asked for has joined.
func (c *Controller) desiredSize(state MIGState, snap Snapshot, demand bool) int {
	want := state.TargetSize
	switch {
	case want == 0 && (demand || snap.PendingGPUPods > 0):
		want = 1
	case snap.PendingGPUPods > 0 && state.Stable && readyNodes(snap.Nodes) >= want:
		want++
	}
	return min(want, c.cfg.MaxSize)
}

// scaleDown cordons and drains every GPU node, then sets the pool to 0. It
// backs off, uncordoning, if GPU work shows up while it runs.
func (c *Controller) scaleDown(ctx context.Context, nodes []Node, lastActivity time.Time, idle time.Duration) error {
	c.log.Info("gpu pool idle, scaling down", slog.Duration("idle_for", idle), slog.Int("nodes", len(nodes)))

	for _, n := range nodes {
		if n.Cordoned {
			continue
		}
		if err := c.cluster.Cordon(ctx, n.Name); err != nil {
			return fmt.Errorf("cordoning %s: %w", n.Name, err)
		}
	}

	// A GPU pod may have been scheduled between the snapshot and the cordon.
	snap, err := c.cluster.Snapshot(ctx)
	if err != nil {
		return err
	}
	if snap.GPUPods > 0 || snap.PendingGPUPods > 0 || c.activeSince(lastActivity) {
		c.log.Info("gpu work arrived while scaling down, keeping the pool")
		c.uncordonOurs(ctx, snap.Nodes)
		return nil
	}

	for _, n := range nodes {
		if err := c.cluster.Drain(ctx, n.Name); err != nil {
			return fmt.Errorf("draining %s: %w", n.Name, err)
		}
	}
	if c.activeSince(lastActivity) {
		c.log.Info("gpu demand arrived while draining, keeping the pool")
		c.uncordonOurs(ctx, snap.Nodes)
		return nil
	}

	if err := c.mig.Resize(ctx, 0); err != nil {
		return fmt.Errorf("scaling gpu pool to 0: %w", err)
	}
	c.setTarget(0)
	c.log.Info("gpu pool scaled to 0")

	for _, n := range nodes {
		if err := c.cluster.DeleteNode(ctx, n.Name); err != nil {
			return fmt.Errorf("deleting node %s: %w", n.Name, err)
		}
	}
	return nil
}

// removeStaleNodes deletes GPU nodes that are NotReady while the pool is
// empty, such as a node whose spot instance was reclaimed.
func (c *Controller) removeStaleNodes(ctx context.Context, nodes []Node) error {
	for _, n := range nodes {
		if n.Ready {
			continue
		}
		c.log.Info("removing gpu node left behind by an empty pool", slog.String("node", n.Name))
		if err := c.cluster.DeleteNode(ctx, n.Name); err != nil {
			return fmt.Errorf("deleting node %s: %w", n.Name, err)
		}
	}
	return nil
}

func (c *Controller) uncordonOurs(ctx context.Context, nodes []Node) {
	for _, n := range nodes {
		if !n.CordonedByScaler {
			continue
		}
		if err := c.cluster.Uncordon(ctx, n.Name); err != nil {
			c.log.Error("uncordoning gpu node", slog.String("node", n.Name), slog.String("error", err.Error()))
		}
	}
}

// activeSince reports whether demand arrived after t.
func (c *Controller) activeSince(t time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.demandPending || c.lastActivity.After(t)
}

func (c *Controller) requeueDemand(demand bool) {
	if !demand {
		return
	}
	c.mu.Lock()
	c.demandPending = true
	c.mu.Unlock()
}

func (c *Controller) setTarget(n int) {
	c.mu.Lock()
	c.status.TargetSize = n
	c.mu.Unlock()
}

func readyNodes(nodes []Node) int {
	n := 0
	for _, node := range nodes {
		if node.Ready {
			n++
		}
	}
	return n
}
