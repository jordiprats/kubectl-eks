package karpenter

import (
	"context"
	"fmt"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var nodePoolGVR = schema.GroupVersionResource{
	Group:    "karpenter.sh",
	Version:  "v1",
	Resource: "nodepools",
}

func GetNodePools(profile, region, clusterName string) ([]data.KarpenterNodePoolInfo, error) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	return getNodePoolsWithConfig(config, profile, region, clusterName)
}

func GetNodePoolsWithConfig(restConfig *rest.Config, profile, region, clusterName string) ([]data.KarpenterNodePoolInfo, error) {
	return getNodePoolsWithConfig(restConfig, profile, region, clusterName)
}

func getNodePoolsWithConfig(config *rest.Config, profile, region, clusterName string) ([]data.KarpenterNodePoolInfo, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	nodePools, err := dynamicClient.Resource(nodePoolGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list NodePools: %w", err)
	}

	var result []data.KarpenterNodePoolInfo

	for _, np := range nodePools.Items {
		info := data.KarpenterNodePoolInfo{
			Profile:     profile,
			Region:      region,
			ClusterName: clusterName,
			Name:        np.GetName(),
		}

		// Extract spec fields
		spec, found, err := unstructured.NestedMap(np.Object, "spec")
		if err != nil || !found {
			continue
		}

		// NodeClass reference
		if template, ok := spec["template"].(map[string]interface{}); ok {
			if nodeClassRef, ok := template["spec"].(map[string]interface{}); ok {
				if ncr, ok := nodeClassRef["nodeClassRef"].(map[string]interface{}); ok {
					if name, ok := ncr["name"].(string); ok {
						info.NodeClassName = name
					}
				}

				// expireAfter lives here in Karpenter v1
				if expireAfter, ok := nodeClassRef["expireAfter"].(string); ok {
					info.ExpireAfter = expireAfter
				}

				// Requirements
				if requirements, ok := nodeClassRef["requirements"].([]interface{}); ok {
					for _, req := range requirements {
						if reqMap, ok := req.(map[string]interface{}); ok {
							key, _ := reqMap["key"].(string)
							values, _ := reqMap["values"].([]interface{})

							switch key {
							case "node.kubernetes.io/instance-type":
								for _, v := range values {
									if str, ok := v.(string); ok {
										info.InstanceTypes = append(info.InstanceTypes, str)
									}
								}
							case "karpenter.sh/capacity-type":
								for _, v := range values {
									if str, ok := v.(string); ok {
										info.CapacityTypes = append(info.CapacityTypes, str)
									}
								}
							case "topology.kubernetes.io/zone":
								for _, v := range values {
									if str, ok := v.(string); ok {
										info.Zones = append(info.Zones, str)
									}
								}
							}
						}
					}
				}
			}
		}

		// Limits
		if limits, ok := spec["limits"].(map[string]interface{}); ok {
			if cpu, ok := limits["cpu"].(string); ok {
				info.CPULimit = cpu
			}
			if memory, ok := limits["memory"].(string); ok {
				info.MemoryLimit = memory
			}
		}

		// Disruption settings
		if disruption, ok := spec["disruption"].(map[string]interface{}); ok {
			if consolidation, ok := disruption["consolidationPolicy"].(string); ok {
				info.ConsolidationMode = consolidation
			}
			// expireAfter fallback for Karpenter v1beta1 (v1 puts it in spec.template.spec)
			if info.ExpireAfter == "" {
				if expireAfter, ok := disruption["expireAfter"].(string); ok {
					info.ExpireAfter = expireAfter
				}
			}
		}

		// Weight
		if weight, ok := spec["weight"].(int64); ok {
			info.Weight = int32(weight)
		}

		result = append(result, info)
	}

	return result, nil
}
