package gpupool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// cordonAnnotation marks a node the scaler cordoned, so it only ever
// uncordons nodes it cordoned itself.
const cordonAnnotation = "sandbox-api.calum.sh/gpu-pool-cordoned"

// Node is a GPU node as the scaler sees it.
type Node struct {
	Name     string `json:"name"`
	Ready    bool   `json:"ready"`
	Cordoned bool   `json:"cordoned"`
	// CordonedByScaler is true when the scaler set the cordon.
	CordonedByScaler bool `json:"-"`
}

// Snapshot is the cluster state one reconcile acts on.
type Snapshot struct {
	// PendingGPUPods counts unscheduled pods that request a GPU.
	PendingGPUPods int
	// GPUPods counts pods on GPU nodes that request a GPU, not counting
	// DaemonSet pods (the per-node Ollama and device plugin) or finished pods.
	GPUPods int
	Nodes   []Node
}

// Cluster is the Kubernetes side of the scaler.
type Cluster interface {
	Snapshot(ctx context.Context) (Snapshot, error)
	Cordon(ctx context.Context, node string) error
	Uncordon(ctx context.Context, node string) error
	// Drain evicts every pod on the node except DaemonSet and mirror pods.
	Drain(ctx context.Context, node string) error
	DeleteNode(ctx context.Context, node string) error
}

// KubeCluster implements Cluster with client-go.
type KubeCluster struct {
	client   kubernetes.Interface
	selector string
	dryRun   bool
	log      *slog.Logger
}

// NewKubeCluster returns a Cluster for the nodes matching cfg's node label.
// With cfg.DryRun it reads the cluster but only logs changes.
func NewKubeCluster(client kubernetes.Interface, cfg Config, log *slog.Logger) *KubeCluster {
	return &KubeCluster{
		client:   client,
		selector: cfg.NodeLabelKey + "=" + cfg.NodeLabelValue,
		dryRun:   cfg.DryRun,
		log:      log,
	}
}

func (k *KubeCluster) Snapshot(ctx context.Context) (Snapshot, error) {
	var snap Snapshot

	pending, err := k.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Pending,spec.nodeName=",
	})
	if err != nil {
		return snap, fmt.Errorf("listing pending pods: %w", err)
	}
	for i := range pending.Items {
		p := &pending.Items[i]
		if p.Status.Phase == corev1.PodPending && p.Spec.NodeName == "" && requestsGPU(p) {
			snap.PendingGPUPods++
		}
	}

	nodes, err := k.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: k.selector})
	if err != nil {
		return snap, fmt.Errorf("listing gpu nodes: %w", err)
	}
	for _, n := range nodes.Items {
		snap.Nodes = append(snap.Nodes, Node{
			Name:             n.Name,
			Ready:            nodeReady(&n),
			Cordoned:         n.Spec.Unschedulable,
			CordonedByScaler: n.Spec.Unschedulable && n.Annotations[cordonAnnotation] == "true",
		})

		pods, err := k.podsOn(ctx, n.Name)
		if err != nil {
			return snap, err
		}
		for i := range pods {
			p := &pods[i]
			if requestsGPU(p) && !isDaemonSetPod(p) && !isFinished(p) {
				snap.GPUPods++
			}
		}
	}
	return snap, nil
}

func (k *KubeCluster) Cordon(ctx context.Context, node string) error {
	if k.dryRun {
		k.log.Info("dry run: would cordon gpu node", slog.String("node", node))
		return nil
	}
	return k.patchNode(ctx, node, map[string]any{
		"metadata": map[string]any{"annotations": map[string]any{cordonAnnotation: "true"}},
		"spec":     map[string]any{"unschedulable": true},
	})
}

func (k *KubeCluster) Uncordon(ctx context.Context, node string) error {
	if k.dryRun {
		k.log.Info("dry run: would uncordon gpu node", slog.String("node", node))
		return nil
	}
	return k.patchNode(ctx, node, map[string]any{
		"metadata": map[string]any{"annotations": map[string]any{cordonAnnotation: nil}},
		"spec":     map[string]any{"unschedulable": false},
	})
}

func (k *KubeCluster) Drain(ctx context.Context, node string) error {
	pods, err := k.podsOn(ctx, node)
	if err != nil {
		return err
	}
	for i := range pods {
		p := &pods[i]
		if isDaemonSetPod(p) || isMirrorPod(p) || isFinished(p) {
			continue
		}
		if k.dryRun {
			k.log.Info("dry run: would evict pod", slog.String("node", node),
				slog.String("namespace", p.Namespace), slog.String("pod", p.Name))
			continue
		}
		err := k.client.PolicyV1().Evictions(p.Namespace).Evict(ctx, &policyv1.Eviction{
			ObjectMeta: metav1.ObjectMeta{Name: p.Name, Namespace: p.Namespace},
		})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("evicting %s/%s: %w", p.Namespace, p.Name, err)
		}
	}
	return nil
}

func (k *KubeCluster) DeleteNode(ctx context.Context, node string) error {
	if k.dryRun {
		k.log.Info("dry run: would delete gpu node", slog.String("node", node))
		return nil
	}
	err := k.client.CoreV1().Nodes().Delete(ctx, node, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (k *KubeCluster) podsOn(ctx context.Context, node string) ([]corev1.Pod, error) {
	pods, err := k.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + node,
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods on %s: %w", node, err)
	}
	var out []corev1.Pod
	for _, p := range pods.Items {
		if p.Spec.NodeName == node {
			out = append(out, p)
		}
	}
	return out, nil
}

func (k *KubeCluster) patchNode(ctx context.Context, node string, patch map[string]any) error {
	data, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	_, err = k.client.CoreV1().Nodes().Patch(ctx, node, types.StrategicMergePatchType, data, metav1.PatchOptions{})
	return err
}

func requestsGPU(p *corev1.Pod) bool {
	containers := append(append([]corev1.Container{}, p.Spec.InitContainers...), p.Spec.Containers...)
	for _, c := range containers {
		for _, list := range []corev1.ResourceList{c.Resources.Requests, c.Resources.Limits} {
			if q, ok := list[GPUResource]; ok && !q.IsZero() {
				return true
			}
		}
	}
	return false
}

func isDaemonSetPod(p *corev1.Pod) bool {
	for _, ref := range p.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

func isMirrorPod(p *corev1.Pod) bool {
	_, ok := p.Annotations[corev1.MirrorPodAnnotationKey]
	return ok
}

func isFinished(p *corev1.Pod) bool {
	return p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed
}

func nodeReady(n *corev1.Node) bool {
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
