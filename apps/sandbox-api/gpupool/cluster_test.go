package gpupool

import (
	"context"
	"io"
	"log/slog"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func gpuNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{"accelerator": "nvidia-l4"}},
		Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
			{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
		}},
	}
}

func pod(name, node string, phase corev1.PodPhase, gpus int64, owner string) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: corev1.PodSpec{
			NodeName:   node,
			Containers: []corev1.Container{{Name: "c"}},
		},
		Status: corev1.PodStatus{Phase: phase},
	}
	if gpus > 0 {
		p.Spec.Containers[0].Resources.Limits = corev1.ResourceList{
			GPUResource: *resource.NewQuantity(gpus, resource.DecimalSI),
		}
	}
	if owner != "" {
		p.OwnerReferences = []metav1.OwnerReference{{Kind: owner, Name: "owner"}}
	}
	return p
}

func testCluster(dryRun bool, objs ...runtime.Object) (*KubeCluster, *fake.Clientset) {
	client := fake.NewClientset(objs...)
	cfg := Config{NodeLabelKey: "accelerator", NodeLabelValue: "nvidia-l4", DryRun: dryRun}
	return NewKubeCluster(client, cfg, slog.New(slog.NewTextHandler(io.Discard, nil))), client
}

func TestSnapshotCountsGPUWork(t *testing.T) {
	k, _ := testCluster(false,
		gpuNode("gpu-1"),
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "virgo"}},
		pod("pending-gpu", "", corev1.PodPending, 1, ""),
		pod("pending-cpu", "", corev1.PodPending, 0, ""),
		pod("sandbox", "gpu-1", corev1.PodRunning, 1, "Job"),
		pod("ollama", "gpu-1", corev1.PodRunning, 1, "DaemonSet"),
		pod("done", "gpu-1", corev1.PodSucceeded, 1, "Job"),
		pod("cpu-on-virgo", "virgo", corev1.PodRunning, 1, ""),
	)

	snap, err := k.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.PendingGPUPods != 1 {
		t.Errorf("PendingGPUPods = %d, want 1", snap.PendingGPUPods)
	}
	if snap.GPUPods != 1 {
		t.Errorf("GPUPods = %d, want 1 (DaemonSet and finished pods excluded)", snap.GPUPods)
	}
	if len(snap.Nodes) != 1 || snap.Nodes[0].Name != "gpu-1" || !snap.Nodes[0].Ready {
		t.Errorf("Nodes = %+v, want gpu-1 ready", snap.Nodes)
	}
}

func TestCordonAndUncordon(t *testing.T) {
	k, client := testCluster(false, gpuNode("gpu-1"))
	ctx := context.Background()

	if err := k.Cordon(ctx, "gpu-1"); err != nil {
		t.Fatal(err)
	}
	snap, _ := k.Snapshot(ctx)
	if n := snap.Nodes[0]; !n.Cordoned || !n.CordonedByScaler {
		t.Fatalf("after Cordon: %+v, want cordoned by the scaler", n)
	}

	if err := k.Uncordon(ctx, "gpu-1"); err != nil {
		t.Fatal(err)
	}
	node, _ := client.CoreV1().Nodes().Get(ctx, "gpu-1", metav1.GetOptions{})
	if node.Spec.Unschedulable {
		t.Fatal("node still unschedulable after Uncordon")
	}
	if _, ok := node.Annotations[cordonAnnotation]; ok {
		t.Fatal("cordon annotation left on the node")
	}
}

func TestDrainEvictsAllButDaemonSetPods(t *testing.T) {
	k, client := testCluster(false,
		gpuNode("gpu-1"),
		pod("web", "gpu-1", corev1.PodRunning, 0, "ReplicaSet"),
		pod("ollama", "gpu-1", corev1.PodRunning, 1, "DaemonSet"),
		pod("elsewhere", "virgo", corev1.PodRunning, 0, ""),
	)

	if err := k.Drain(context.Background(), "gpu-1"); err != nil {
		t.Fatal(err)
	}

	var evicted []string
	for _, a := range client.Actions() {
		if a.GetVerb() == "create" && a.GetSubresource() == "eviction" {
			evicted = append(evicted, a.(k8stesting.CreateAction).GetObject().(metav1.Object).GetName())
		}
	}
	if len(evicted) != 1 || evicted[0] != "web" {
		t.Fatalf("evicted = %v, want [web]", evicted)
	}
}

func TestDryRunMakesNoChanges(t *testing.T) {
	k, client := testCluster(true, gpuNode("gpu-1"), pod("web", "gpu-1", corev1.PodRunning, 0, ""))
	ctx := context.Background()

	for _, step := range []func(context.Context, string) error{k.Cordon, k.Drain, k.Uncordon, k.DeleteNode} {
		if err := step(ctx, "gpu-1"); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range client.Actions() {
		if a.GetVerb() != "list" && a.GetVerb() != "get" {
			t.Fatalf("dry run made a %s %s call", a.GetVerb(), a.GetResource().Resource)
		}
	}
}
