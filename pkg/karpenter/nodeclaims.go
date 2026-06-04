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

var nodeClaimGVR = schema.GroupVersionResource{
	Group:    "karpenter.sh",
	Version:  "v1",
	Resource: "nodeclaims",
}

func GetNodeClaims(profile, region, clusterName string) ([]data.KarpenterNodeClaimInfo, error) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	return getNodeClaimsWithConfig(config, profile, region, clusterName)
}

func GetNodeClaimsWithConfig(restConfig *rest.Config, profile, region, clusterName string) ([]data.KarpenterNodeClaimInfo, error) {
	return getNodeClaimsWithConfig(restConfig, profile, region, clusterName)
}

func getNodeClaimsWithConfig(config *rest.Config, profile, region, clusterName string) ([]data.KarpenterNodeClaimInfo, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	nodeClaims, err := dynamicClient.Resource(nodeClaimGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list NodeClaims: %w", err)
	}

	var result []data.KarpenterNodeClaimInfo

	for _, nc := range nodeClaims.Items {
		info := data.KarpenterNodeClaimInfo{
			Profile:     profile,
			Region:      region,
			ClusterName: clusterName,
			Name:        nc.GetName(),
			Age:         nc.GetCreationTimestamp().Time,
		}

		// Extract spec fields
		spec, found, err := unstructured.NestedMap(nc.Object, "spec")
		if err != nil || !found {
			continue
		}

		if nodeClassRef, ok := spec["nodeClassRef"].(map[string]interface{}); ok {
			if name, ok := nodeClassRef["name"].(string); ok {
				info.NodePoolName = name
			}
		}

		// Extract status fields
		status, found, err := unstructured.NestedMap(nc.Object, "status")
		if err != nil || !found {
			info.Status = "Unknown"
		} else {
			// Node name
			if nodeName, ok := status["nodeName"].(string); ok {
				info.NodeName = nodeName
			}

			// Instance type
			if instanceType, ok := status["instanceType"].(string); ok {
				info.InstanceType = instanceType
			}

			// Zone
			if zone, ok := status["zone"].(string); ok {
				info.Zone = zone
			}

			// Capacity type
			if capacityType, ok := status["capacityType"].(string); ok {
				info.CapacityType = capacityType
			}

			// AMI
			if imageID, ok := status["imageID"].(string); ok {
				info.AMI = imageID
			}

			// Conditions for status
			if conditions, ok := status["conditions"].([]interface{}); ok {
				for _, cond := range conditions {
					if condMap, ok := cond.(map[string]interface{}); ok {
						if condType, ok := condMap["type"].(string); ok && condType == "Ready" {
							if condStatus, ok := condMap["status"].(string); ok {
								if condStatus == "True" {
									info.Status = "Ready"
								} else {
									info.Status = "NotReady"
								}
							}
						}
						if condType, ok := condMap["type"].(string); ok && condType == "Drifted" {
							if condStatus, ok := condMap["status"].(string); ok && condStatus == "True" {
								info.Drifted = true
							}
						}
					}
				}
			}

			if info.Status == "" {
				info.Status = "Pending"
			}
		}

		result = append(result, info)
	}

	return result, nil
}
