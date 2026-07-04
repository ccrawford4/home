package k8s

import (
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create object metadata for the job
func createObjectMeta(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}
}

// Create the job specification for the job
func createJobSpec(name string) *batchv1.JobSpec {
	return &batchv1.JobSpec{
		Template: v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:    name,
						Image:   "nginx",
						Command: strings.Split("echo Hello World", " "),
					},
				},
				RestartPolicy: v1.RestartPolicyOnFailure,
			},
		},
	}
}

// Create the job object based on the provided job configuration
func createJobObject(jobConfig *JobLaunchConfig) *batchv1.Job {
	objectMeta := createObjectMeta(jobConfig.Name, jobConfig.Namespace)
	jobSpec := createJobSpec(jobConfig.Name)

	return &batchv1.Job{
		ObjectMeta: objectMeta,
		Spec:       *jobSpec,
	}
}
