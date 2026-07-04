package k8s

import "k8s.io/client-go/kubernetes"

// KubeClient is a wrapper around the Kubernetes clientset.
type KubeClient struct {
	clientset *kubernetes.Clientset
}

// KubeClientConfig holds the configuration for creating a KubeClient.
type KubeClientConfig struct {
	inCluster bool // Indicates whether the client is running inside or outside a Kubernetes cluster.
}
