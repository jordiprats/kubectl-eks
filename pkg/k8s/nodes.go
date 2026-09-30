package k8s

import (
	"context"
	"fmt"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func GetNodesWithConfig(restConfig *rest.Config) ([]data.NodeInfo, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	runningPodsByNode, err := getRunningPodsByNode(clientset)
	if err != nil {
		return nil, fmt.Errorf("failed to list running pods: %w", err)
	}

	var nodeList []data.NodeInfo
	for _, node := range nodes.Items {
		labels := node.Labels

		// Compute type (EC2 or Fargate)
		compute := "EC2"
		if labels["eks.amazonaws.com/compute-type"] == "fargate" {
			compute = "Fargate"
		}

		// Instance type
		instanceType := labels["node.kubernetes.io/instance-type"]
		if instanceType == "" {
			instanceType = labels["beta.kubernetes.io/instance-type"]
		}

		// For Fargate, show capacity instead
		if compute == "Fargate" && instanceType == "" {
			cpu := node.Status.Capacity.Cpu().String()
			mem := node.Status.Capacity.Memory()
			memGi := mem.Value() / (1024 * 1024 * 1024)
			instanceType = fmt.Sprintf("%svCPU/%dGi", cpu, memGi)
		}

		// Managed by (nodegroup, Fargate profile, or Karpenter)
		managedBy := ""
		if ng := labels["eks.amazonaws.com/nodegroup"]; ng != "" {
			managedBy = "Nodegroup: " + ng
		} else if fp := labels["eks.amazonaws.com/fargate-profile"]; fp != "" {
			managedBy = "Fargate: " + fp
		} else if np := labels["karpenter.sh/nodepool"]; np != "" {
			managedBy = "Karpenter: " + np
		} else if labels["karpenter.sh/provisioner-name"] != "" {
			managedBy = "Karpenter: " + labels["karpenter.sh/provisioner-name"]
		} else {
			managedBy = "AWS"
		}

		status := getNodeStatus(node)

		nodeList = append(nodeList, data.NodeInfo{
			Name:               node.Name,
			InstanceType:       instanceType,
			Compute:            compute,
			ManagedBy:          managedBy,
			Created:            node.CreationTimestamp.Time,
			Status:             status,
			CPUCapacity:        getNodeResourceQuantity(node, true, corev1.ResourceCPU),
			CPUAllocatable:     getNodeResourceQuantity(node, false, corev1.ResourceCPU),
			CPUUsed:            getNodeResourceUsed(node, corev1.ResourceCPU),
			MemoryCapacity:     getNodeResourceQuantity(node, true, corev1.ResourceMemory),
			MemoryAllocatable:  getNodeResourceQuantity(node, false, corev1.ResourceMemory),
			MemoryUsed:         getNodeResourceUsed(node, corev1.ResourceMemory),
			PodsCapacity:       getNodeResourceQuantity(node, true, corev1.ResourcePods),
			PodsAllocatable:    getNodeResourceQuantity(node, false, corev1.ResourcePods),
			PodsUsed:           getNodeResourceUsed(node, corev1.ResourcePods),
			PodsRunning:        runningPodsByNode[node.Name],
			MemoryPressure:     getNodeConditionStatus(node, corev1.NodeMemoryPressure),
			DiskPressure:       getNodeConditionStatus(node, corev1.NodeDiskPressure),
			PIDPressure:        getNodeConditionStatus(node, corev1.NodePIDPressure),
			NetworkUnavailable: getNodeConditionStatus(node, corev1.NodeNetworkUnavailable),
		})
	}

	return nodeList, nil
}

func GetNodeCountWithConfig(restConfig *rest.Config) (int, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return 0, fmt.Errorf("failed to create clientset: %w", err)
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list nodes: %w", err)
	}

	return len(nodes.Items), nil
}

func GetNodes(configFlags *genericclioptions.ConfigFlags) ([]data.NodeInfo, error) {
	config, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	runningPodsByNode, err := getRunningPodsByNode(clientset)
	if err != nil {
		return nil, fmt.Errorf("failed to list running pods: %w", err)
	}

	var nodeList []data.NodeInfo
	for _, node := range nodes.Items {
		labels := node.Labels

		// Instance type
		instanceType := labels["node.kubernetes.io/instance-type"]
		if instanceType == "" {
			instanceType = labels["beta.kubernetes.io/instance-type"]
		}

		// Compute type (EC2 or Fargate)
		compute := "EC2"
		if labels["eks.amazonaws.com/compute-type"] == "fargate" {
			compute = "Fargate"
		}

		// Managed by (nodegroup, Fargate profile, or Karpenter)
		managedBy := ""
		if ng := labels["eks.amazonaws.com/nodegroup"]; ng != "" {
			managedBy = "Nodegroup: " + ng
		} else if fp := labels["eks.amazonaws.com/fargate-profile"]; fp != "" {
			managedBy = "Fargate: " + fp
		} else if np := labels["karpenter.sh/nodepool"]; np != "" {
			managedBy = "Karpenter: " + np
		} else if labels["karpenter.sh/provisioner-name"] != "" {
			// Older Karpenter versions used provisioner-name
			managedBy = "Karpenter: " + labels["karpenter.sh/provisioner-name"]
		} else {
			managedBy = "AWS"
		}

		status := getNodeStatus(node)

		nodeList = append(nodeList, data.NodeInfo{
			Name:               node.Name,
			InstanceType:       instanceType,
			Compute:            compute,
			ManagedBy:          managedBy,
			Created:            node.CreationTimestamp.Time,
			Status:             status,
			CPUCapacity:        getNodeResourceQuantity(node, true, corev1.ResourceCPU),
			CPUAllocatable:     getNodeResourceQuantity(node, false, corev1.ResourceCPU),
			CPUUsed:            getNodeResourceUsed(node, corev1.ResourceCPU),
			MemoryCapacity:     getNodeResourceQuantity(node, true, corev1.ResourceMemory),
			MemoryAllocatable:  getNodeResourceQuantity(node, false, corev1.ResourceMemory),
			MemoryUsed:         getNodeResourceUsed(node, corev1.ResourceMemory),
			PodsCapacity:       getNodeResourceQuantity(node, true, corev1.ResourcePods),
			PodsAllocatable:    getNodeResourceQuantity(node, false, corev1.ResourcePods),
			PodsUsed:           getNodeResourceUsed(node, corev1.ResourcePods),
			PodsRunning:        runningPodsByNode[node.Name],
			MemoryPressure:     getNodeConditionStatus(node, corev1.NodeMemoryPressure),
			DiskPressure:       getNodeConditionStatus(node, corev1.NodeDiskPressure),
			PIDPressure:        getNodeConditionStatus(node, corev1.NodePIDPressure),
			NetworkUnavailable: getNodeConditionStatus(node, corev1.NodeNetworkUnavailable),
		})
	}

	return nodeList, nil
}

func getNodeStatus(node corev1.Node) string {
	status := "Unknown"
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionTrue {
				status = "Ready"
			} else {
				status = "NotReady"
			}
			break
		}
	}

	if node.Spec.Unschedulable {
		status += ",SchedulingDisabled"
	}

	return status
}

func getNodeConditionStatus(node corev1.Node, conditionType corev1.NodeConditionType) string {
	for _, cond := range node.Status.Conditions {
		if cond.Type == conditionType {
			return string(cond.Status)
		}
	}

	return "Unknown"
}

func getNodeResourceQuantity(node corev1.Node, capacity bool, resourceName corev1.ResourceName) string {
	var quantity string

	if capacity {
		if q, ok := node.Status.Capacity[resourceName]; ok {
			quantity = q.String()
		}
	} else {
		if q, ok := node.Status.Allocatable[resourceName]; ok {
			quantity = q.String()
		}
	}

	if quantity == "" {
		return "-"
	}

	return quantity
}

func getNodeResourceUsed(node corev1.Node, resourceName corev1.ResourceName) string {
	capacity, capOK := node.Status.Capacity[resourceName]
	allocatable, allocOK := node.Status.Allocatable[resourceName]
	if !capOK || !allocOK {
		return "-"
	}

	used := capacity.DeepCopy()
	used.Sub(allocatable)
	if used.Sign() < 0 {
		return "0"
	}

	return used.String()
}

func getRunningPodsByNode(clientset *kubernetes.Clientset) (map[string]int, error) {
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{FieldSelector: "status.phase=Running"})
	if err != nil {
		return nil, err
	}

	runningPodsByNode := make(map[string]int)
	for _, pod := range pods.Items {
		if pod.Spec.NodeName == "" {
			continue
		}
		runningPodsByNode[pod.Spec.NodeName]++
	}

	return runningPodsByNode, nil
}
