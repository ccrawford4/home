package k8s

import "k8s.io/client-go/kubernetes"

// KubeClient is a wrapper around the Kubernetes clientset.
type KubeClient struct {
	clientset *kubernetes.Clientset
}

// KubeClientConfig holds the configuration for creating a KubeClient.
type KubeClientConfig struct {
	InCluster bool // Indicates whether the client is running inside or outside a Kubernetes cluster.
}

// The JobLaunch input params
type JobLaunchConfig struct {
	Name      string // The name of the job to be launched.
	Namespace string // The namespace in which to launch the job.
}
