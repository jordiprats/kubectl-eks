package karpenter

import (
	"github.com/jordiprats/kubectl-eks/pkg/data"
	"k8s.io/client-go/rest"
)

func GetAMIUsage(profile, region, clusterName, eksVersion string) ([]data.KarpenterAMIUsageInfo, error) {
	// Get NodeClaims to find current AMIs
	nodeClaims, err := GetNodeClaims(profile, region, clusterName)
	if err != nil {
		return nil, err
	}

	return buildAMIUsageResult(nodeClaims, profile, region, clusterName), nil
}

func GetAMIUsageWithConfig(restConfig *rest.Config, profile, region, clusterName, eksVersion string) ([]data.KarpenterAMIUsageInfo, error) {
	nodeClaims, err := GetNodeClaimsWithConfig(restConfig, profile, region, clusterName)
	if err != nil {
		return nil, err
	}

	return buildAMIUsageResult(nodeClaims, profile, region, clusterName), nil
}

func buildAMIUsageResult(nodeClaims []data.KarpenterNodeClaimInfo, profile, region, clusterName string) []data.KarpenterAMIUsageInfo {
	// Build map of NodePool -> AMIs in use
	nodePoolAMIs := make(map[string]map[string]int)
	for _, nc := range nodeClaims {
		if nc.NodePoolName != "" && nc.AMI != "" {
			if nodePoolAMIs[nc.NodePoolName] == nil {
				nodePoolAMIs[nc.NodePoolName] = make(map[string]int)
			}
			nodePoolAMIs[nc.NodePoolName][nc.AMI]++
		}
	}

	var result []data.KarpenterAMIUsageInfo

	for nodePoolName, amis := range nodePoolAMIs {
		for ami, count := range amis {
			info := data.KarpenterAMIUsageInfo{
				Profile:      profile,
				Region:       region,
				ClusterName:  clusterName,
				NodePoolName: nodePoolName,
				CurrentAMI:   ami,
				NodeCount:    count,
			}
			result = append(result, info)
		}
	}

	return result
}
