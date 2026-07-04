package k8s

import (
	log "log/slog"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"path/filepath"

	"k8s.io/client-go/util/homedir"
)

// Gets the Kubernetes cluster configuration based on whether the client is running in-cluster or out-of-cluster.
func getClusterConfig(inCluster bool) (*rest.Config, error) {
	if inCluster {
		// Initialize the in-cluster configuration
		return rest.InClusterConfig()
	} else {
		// Initialize the out-of-cluster configuration
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		return config, err
	}
}

// Creates a new KubeClient based on the provided KubeClientConfig.
func NewKubeClient(clientConfig *KubeClientConfig) (*KubeClient, error) {
	// Get the cluster configuration based on the clientConfig
	clusterConfig, err := getClusterConfig(clientConfig.inCluster)
	if err != nil {
		log.Error("Failed to get cluster configuration", err)
		return nil, err
	}

	// Create the Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Error("Failed to create Kubernetes clientset", err)
	}

	// Return the KubeClient instance
	return &KubeClient{
		clientset,
	}, nil
}
