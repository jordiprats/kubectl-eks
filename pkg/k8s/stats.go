package k8s

import (
	"context"
	"log"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type K8Sstats struct {
	AWSProfile            string
	Region                string
	ClusterName           string
	Arn                   string
	Version               string
	PodCount              int
	NodeCount             int
	NodesNotReady         int
	PodsNotRunning        int
	NamespaceCount        int
	PodsWithRestartsCount int
}

func GetK8sStats(awsRegion, region, clusterName, arn, version string) (*K8Sstats, error) {
	stats := &K8Sstats{
		AWSProfile:  awsRegion,
		Region:      region,
		ClusterName: clusterName,
		Arn:         arn,
		Version:     version,
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

	// Pods
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		stats.PodCount++
		if pod.Status.Phase != "Running" {
			stats.PodsNotRunning++
		}

		// Container restarts
		for _, container := range pod.Status.ContainerStatuses {
			if container.RestartCount > 0 {
				stats.PodsWithRestartsCount++
				break
			}
		}
	}

	// Nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	stats.NodeCount = len(nodes.Items)

	for _, node := range nodes.Items {
		if node.Status.Conditions != nil {
			for _, condition := range node.Status.Conditions {
				if condition.Type == "Ready" && condition.Status != "True" {
					stats.NodesNotReady++
				}
			}
		}
	}

	// Namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	stats.NamespaceCount = len(namespaces.Items)

	return stats, nil
}

func GetK8sStatsWithConfig(restConfig *rest.Config, awsRegion, region, clusterName, arn, version string) (*K8Sstats, error) {
	stats := &K8Sstats{
		AWSProfile:  awsRegion,
		Region:      region,
		ClusterName: clusterName,
		Arn:         arn,
		Version:     version,
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	// Pods
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		stats.PodCount++
		if pod.Status.Phase != "Running" {
			stats.PodsNotRunning++
		}

		for _, container := range pod.Status.ContainerStatuses {
			if container.RestartCount > 0 {
				stats.PodsWithRestartsCount++
				break
			}
		}
	}

	// Nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	stats.NodeCount = len(nodes.Items)

	for _, node := range nodes.Items {
		if node.Status.Conditions != nil {
			for _, condition := range node.Status.Conditions {
				if condition.Type == "Ready" && condition.Status != "True" {
					stats.NodesNotReady++
				}
			}
		}
	}

	// Namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	stats.NamespaceCount = len(namespaces.Items)

	return stats, nil
}
