package k8s

import (
	"context"
	log "log/slog"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	clusterConfig, err := getClusterConfig(clientConfig.InCluster)
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

// Create a new job
func (kc *KubeClient) CreateJob(jobConfig *JobLaunchConfig) (*batchv1.Job, error) {
	jobs := kc.clientset.BatchV1().Jobs(jobConfig.Namespace)
	jobObject := createJobObject(jobConfig)

	response, err := jobs.Create(context.Background(), jobObject, metav1.CreateOptions{})
	if err != nil {
		log.Error("Failed to create job", err)
		return nil, err
	}

	return response, err
}
