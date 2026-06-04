package k8s

import (
	"context"
	"fmt"
	"log"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// $ k get pods -n fluent-bit
// NAME                  READY   STATUS    RESTARTS   AGE
// fluent-bit-47w89      1/1     Running   0          2m31s

// $ k get pods
// NAME                  READY   STATUS    RESTARTS   AGE
// fluent-bit-47w89      1/1     Running   0          2m58s

type K8SPodInfo struct {
	Name      string
	Namespace string
	Ready     string
	Status    string
	Restarts  int
	Age       metav1.Time
}

type K8SClusterPodList struct {
	AWSProfile  string
	Region      string
	ClusterName string
	Arn         string
	Version     string
	Pods        []K8SPodInfo
}

func GetPods(awsRegion, region, clusterName, arn, version, namespace string, allNamespaces bool) (*K8SClusterPodList, error) {
	podList := &K8SClusterPodList{
		AWSProfile:  awsRegion,
		Region:      region,
		ClusterName: clusterName,
		Arn:         arn,
		Version:     version,
		Pods:        []K8SPodInfo{},
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = clientcmd.RecommendedHomeFile // fallback to ~/.kube/config
	}

	// Load config
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Error loading kubeconfig from %s: %v", kubeconfig, err)
	}

	// Create Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating clientset: %v", err)
	}

	queryNamespace := namespace
	if allNamespaces {
		queryNamespace = ""
	}
	if namespace == "" && !allNamespaces {
		queryNamespace = "default"
	}

	// Pods
	pods, err := clientset.CoreV1().Pods(queryNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		infoEachPod := K8SPodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			Age:       pod.CreationTimestamp,
		}

		// Containers
		readyContainers := 0
		for _, container := range pod.Status.ContainerStatuses {
			if container.Ready {
				readyContainers++
			}
			if container.RestartCount > 0 {
				infoEachPod.Restarts = int(container.RestartCount)
			}
		}
		infoEachPod.Ready = fmt.Sprintf("%d/%d", readyContainers, len(pod.Status.ContainerStatuses))

		podList.Pods = append(podList.Pods, infoEachPod)
	}

	return podList, nil
}

func GetPodsWithConfig(restConfig *rest.Config, awsRegion, region, clusterName, arn, version, namespace string, allNamespaces bool) (*K8SClusterPodList, error) {
	podList := &K8SClusterPodList{
		AWSProfile:  awsRegion,
		Region:      region,
		ClusterName: clusterName,
		Arn:         arn,
		Version:     version,
		Pods:        []K8SPodInfo{},
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset: %w", err)
	}

	queryNamespace := namespace
	if allNamespaces {
		queryNamespace = ""
	}
	if namespace == "" && !allNamespaces {
		queryNamespace = "default"
	}

	pods, err := clientset.CoreV1().Pods(queryNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		infoEachPod := K8SPodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			Age:       pod.CreationTimestamp,
		}

		readyContainers := 0
		for _, container := range pod.Status.ContainerStatuses {
			if container.Ready {
				readyContainers++
			}
			if container.RestartCount > 0 {
				infoEachPod.Restarts = int(container.RestartCount)
			}
		}
		infoEachPod.Ready = fmt.Sprintf("%d/%d", readyContainers, len(pod.Status.ContainerStatuses))

		podList.Pods = append(podList.Pods, infoEachPod)
	}

	return podList, nil
}
