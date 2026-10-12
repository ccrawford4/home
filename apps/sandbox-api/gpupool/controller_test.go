package gpupool

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type fakeMIG struct {
	size    int
	stable  bool
	resizes []int
	err     error
}

func (m *fakeMIG) Get(context.Context) (MIGState, error) {
	return MIGState{TargetSize: m.size, Stable: m.stable}, nil
}

func (m *fakeMIG) Resize(_ context.Context, n int) error {
	if m.err != nil {
		return m.err
	}
	m.resizes = append(m.resizes, n)
	m.size = n
	return nil
}

type fakeCluster struct {
	snap    Snapshot
	actions []string
	// afterCordon, when set, replaces snap once a node is cordoned.
	afterCordon *Snapshot
	// onDrain runs during Drain, to simulate demand arriving mid-drain.
	onDrain func()
}

func (f *fakeCluster) Snapshot(context.Context) (Snapshot, error) { return f.snap, nil }

func (f *fakeCluster) Cordon(_ context.Context, n string) error {
	f.actions = append(f.actions, "cordon "+n)
	if f.afterCordon != nil {
		f.snap = *f.afterCordon
		return nil
	}
	for i := range f.snap.Nodes {
		if f.snap.Nodes[i].Name == n {
			f.snap.Nodes[i].Cordoned = true
			f.snap.Nodes[i].CordonedByScaler = true
		}
	}
	return nil
}

func (f *fakeCluster) Uncordon(_ context.Context, n string) error {
	f.actions = append(f.actions, "uncordon "+n)
	return nil
}

func (f *fakeCluster) Drain(_ context.Context, n string) error {
	f.actions = append(f.actions, "drain "+n)
	if f.onDrain != nil {
		f.onDrain()
	}
	return nil
}

func (f *fakeCluster) DeleteNode(_ context.Context, n string) error {
	f.actions = append(f.actions, "delete "+n)
	return nil
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestController(mig *fakeMIG, cluster *fakeCluster) (*Controller, *clock) {
	clk := &clock{t: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)}
	cfg := Config{MaxSize: 1, IdleTimeout: 10 * time.Minute, SyncInterval: 30 * time.Second}
	c := NewController(cfg, mig, cluster, slog.New(slog.NewTextHandler(io.Discard, nil)))
	c.now = clk.now
	c.lastActivity = clk.t
	return c, clk
}

func reconcile(t *testing.T, c *Controller) {
	t.Helper()
	if err := c.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestScalesUpForPendingGPUPod(t *testing.T) {
	mig := &fakeMIG{stable: true}
	c, _ := newTestController(mig, &fakeCluster{snap: Snapshot{PendingGPUPods: 1}})

	reconcile(t, c)

	if len(mig.resizes) != 1 || mig.resizes[0] != 1 {
		t.Fatalf("resizes = %v, want [1]", mig.resizes)
	}
}

func TestScalesUpOnDemand(t *testing.T) {
	mig := &fakeMIG{stable: true}
	c, _ := newTestController(mig, &fakeCluster{})

	c.Demand("qwen2.5-coder:7b", "no gpu backend")
	reconcile(t, c)

	if mig.size != 1 {
		t.Fatalf("size = %d, want 1", mig.size)
	}
}

func TestNeverExceedsMaxSize(t *testing.T) {
	mig := &fakeMIG{size: 1, stable: true}
	c, _ := newTestController(mig, &fakeCluster{snap: Snapshot{
		PendingGPUPods: 3,
		Nodes:          []Node{{Name: "gpu-1", Ready: true}},
	}})

	reconcile(t, c)

	if len(mig.resizes) != 0 {
		t.Fatalf("resizes = %v, want none at max size", mig.resizes)
	}
}

func TestFailedScaleUpKeepsDemand(t *testing.T) {
	mig := &fakeMIG{stable: true, err: errors.New("quota exceeded")}
	c, _ := newTestController(mig, &fakeCluster{})

	c.Demand("", "")
	if err := c.Reconcile(context.Background()); err == nil {
		t.Fatal("Reconcile succeeded, want the resize error")
	}
	if st := c.Status(); st.LastError == "" {
		t.Fatal("Status().LastError is empty after a failed resize")
	}

	mig.err = nil
	reconcile(t, c)
	if mig.size != 1 {
		t.Fatalf("size = %d, want 1 after retry", mig.size)
	}
}

func TestStaysUpWhileIdleLessThanTimeout(t *testing.T) {
	mig := &fakeMIG{size: 1, stable: true}
	cluster := &fakeCluster{snap: Snapshot{Nodes: []Node{{Name: "gpu-1", Ready: true}}}}
	c, clk := newTestController(mig, cluster)

	clk.advance(9 * time.Minute)
	reconcile(t, c)

	if len(mig.resizes) != 0 || len(cluster.actions) != 0 {
		t.Fatalf("resizes = %v, actions = %v, want none before the idle timeout", mig.resizes, cluster.actions)
	}
}

func TestScalesDownAfterIdleTimeout(t *testing.T) {
	mig := &fakeMIG{size: 1, stable: true}
	cluster := &fakeCluster{snap: Snapshot{Nodes: []Node{{Name: "gpu-1", Ready: true}}}}
	c, clk := newTestController(mig, cluster)

	clk.advance(10 * time.Minute)
	reconcile(t, c)

	if mig.size != 0 {
		t.Fatalf("size = %d, want 0", mig.size)
	}
	want := []string{"cordon gpu-1", "drain gpu-1", "delete gpu-1"}
	if !equal(cluster.actions, want) {
		t.Fatalf("actions = %v, want %v", cluster.actions, want)
	}
}

func TestGPUPodsKeepThePoolUp(t *testing.T) {
	mig := &fakeMIG{size: 1, stable: true}
	cluster := &fakeCluster{snap: Snapshot{GPUPods: 1, Nodes: []Node{{Name: "gpu-1", Ready: true}}}}
	c, clk := newTestController(mig, cluster)

	clk.advance(30 * time.Minute)
	reconcile(t, c)
	if mig.size != 1 {
		t.Fatal("scaled down while a GPU pod was running")
	}

	// The pod finishes; the idle clock starts from the last reconcile that saw it.
	cluster.snap.GPUPods = 0
	clk.advance(9 * time.Minute)
	reconcile(t, c)
	if mig.size != 1 {
		t.Fatal("scaled down before 10 minutes without GPU pods")
	}
	clk.advance(time.Minute)
	reconcile(t, c)
	if mig.size != 0 {
		t.Fatal("did not scale down 10 minutes after the last GPU pod")
	}
}

func TestDemandResetsIdleClock(t *testing.T) {
	mig := &fakeMIG{size: 1, stable: true}
	cluster := &fakeCluster{snap: Snapshot{Nodes: []Node{{Name: "gpu-1", Ready: true}}}}
	c, clk := newTestController(mig, cluster)

	clk.advance(8 * time.Minute)
	c.Demand("", "")
	clk.advance(8 * time.Minute)
	reconcile(t, c)

	if mig.size != 1 {
		t.Fatal("scaled down 8 minutes after a demand call")
	}
}

func TestNoScaleDownWhileGroupUnstable(t *testing.T) {
	mig := &fakeMIG{size: 1, stable: false}
	c, clk := newTestController(mig, &fakeCluster{})

	clk.advance(time.Hour)
	reconcile(t, c)

	if mig.size != 1 {
		t.Fatal("scaled down while the group was still creating its instance")
	}
}

func TestBacksOffWhenGPUPodLandsDuringCordon(t *testing.T) {
	mig := &fakeMIG{size: 1, stable: true}
	cluster := &fakeCluster{
		snap: Snapshot{Nodes: []Node{{Name: "gpu-1", Ready: true}}},
		afterCordon: &Snapshot{
			GPUPods: 1,
			Nodes:   []Node{{Name: "gpu-1", Ready: true, Cordoned: true, CordonedByScaler: true}},
		},
	}
	c, clk := newTestController(mig, cluster)

	clk.advance(10 * time.Minute)
	reconcile(t, c)

	if mig.size != 1 {
		t.Fatal("scaled down although a GPU pod was scheduled")
	}
	want := []string{"cordon gpu-1", "uncordon gpu-1"}
	if !equal(cluster.actions, want) {
		t.Fatalf("actions = %v, want %v", cluster.actions, want)
	}
}

func TestBacksOffWhenDemandArrivesDuringDrain(t *testing.T) {
	mig := &fakeMIG{size: 1, stable: true}
	cluster := &fakeCluster{snap: Snapshot{Nodes: []Node{{Name: "gpu-1", Ready: true}}}}
	c, clk := newTestController(mig, cluster)
	cluster.onDrain = func() { c.Demand("", "") }

	clk.advance(10 * time.Minute)
	reconcile(t, c)

	if mig.size != 1 {
		t.Fatal("scaled down although demand arrived while draining")
	}
	want := []string{"cordon gpu-1", "drain gpu-1", "uncordon gpu-1"}
	if !equal(cluster.actions, want) {
		t.Fatalf("actions = %v, want %v", cluster.actions, want)
	}
}

func TestActivityUncordonsNodesTheScalerCordoned(t *testing.T) {
	mig := &fakeMIG{size: 1, stable: true}
	cluster := &fakeCluster{snap: Snapshot{
		PendingGPUPods: 1,
		Nodes: []Node{
			{Name: "gpu-1", Ready: true, Cordoned: true, CordonedByScaler: true},
			{Name: "gpu-2", Ready: true, Cordoned: true},
		},
	}}
	c, _ := newTestController(mig, cluster)

	reconcile(t, c)

	want := []string{"uncordon gpu-1"}
	if !equal(cluster.actions, want) {
		t.Fatalf("actions = %v, want %v (a node cordoned by hand stays cordoned)", cluster.actions, want)
	}
}

func TestRemovesStaleNodesWhenPoolIsEmpty(t *testing.T) {
	mig := &fakeMIG{size: 0, stable: true}
	cluster := &fakeCluster{snap: Snapshot{Nodes: []Node{
		{Name: "gpu-old", Ready: false},
		{Name: "gpu-live", Ready: true},
	}}}
	c, _ := newTestController(mig, cluster)

	reconcile(t, c)

	if want := []string{"delete gpu-old"}; !equal(cluster.actions, want) {
		t.Fatalf("actions = %v, want %v", cluster.actions, want)
	}
}
