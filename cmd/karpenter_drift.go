package cmd

import (
	"log"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/karpenter"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var karpenterDriftCmd = &cobra.Command{
	Use:   "drift",
	Short: "List drifted Karpenter nodes and NodeClaims",
	Long: `List nodes and NodeClaims currently in drifted state across clusters.

Drift occurs when NodeClaims no longer match their NodePool requirements
due to configuration changes, AMI updates, or other factors.`,
	Example: `  # List drifted resources for current cluster
  kubectl eks karpenter drift

  # List drifted resources across clusters matching filter
  kubectl eks karpenter drift --cluster-contains prod`,
	Run: func(cmd *cobra.Command, args []string) {
		refresh, _ := cmd.Flags().GetBool("refresh")
		profile, _ := cmd.Flags().GetString("profile")
		profileContains, _ := cmd.Flags().GetString("profile-contains")
		profileNotContains, _ := cmd.Flags().GetString("profile-not-contains")
		nameContains, _ := cmd.Flags().GetString("cluster-contains")
		nameNotContains, _ := cmd.Flags().GetString("cluster-not-contains")
		region, _ := cmd.Flags().GetString("region")
		version, _ := cmd.Flags().GetString("version")
		noHeaders, _ := cmd.Flags().GetBool("no-headers")
		nodepoolContains, _ := cmd.Flags().GetString("nodepool-contains")

		hasFilters := profile != "" || profileContains != "" || profileNotContains != "" || nameContains != "" ||
			nameNotContains != "" || region != "" || version != ""

		var clusterList []data.ClusterInfo
		var err error

		if hasFilters {
			loadCacheFromDisk()
			if CachedData == nil {
				CachedData = &data.KubeCtlEksCache{
					ClusterByARN: make(map[string]data.ClusterInfo),
					ClusterList:  make(map[string]map[string][]data.ClusterInfo),
				}
			}
			clusterList, err = LoadClusterList([]string{}, profile, profileContains, profileNotContains, nameContains, nameNotContains, region, version, refresh)
			if err != nil {
				log.Fatalf("Error loading cluster list: %v", err)
			}
		} else {
			clusterInfo, err := GetCurrentClusterInfo()
			if err != nil {
				log.Fatalf("Error getting current cluster info: %v", err)
			}
			clusterList = []data.ClusterInfo{clusterInfo}
		}

		allDriftedResources := []data.KarpenterDriftInfo{}

		for _, clusterInfo := range clusterList {
			restConfig, err := GetRestConfigForCluster(clusterInfo)
			if err != nil {
				if verbose {
					log.Printf("Warning: Failed to get kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
				}
				continue
			}

			driftedResources, err := karpenter.GetDriftedResourcesWithConfig(restConfig, clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
			if err != nil {
				if verbose {
					log.Printf("Warning: Failed to get drifted resources from cluster %s: %v", clusterInfo.ClusterName, err)
				}
				continue
			}

			allDriftedResources = append(allDriftedResources, driftedResources...)
		}

		if nodepoolContains != "" {
			filtered := allDriftedResources[:0]
			for _, d := range allDriftedResources {
				if strings.Contains(strings.ToLower(d.NodePoolName), strings.ToLower(nodepoolContains)) {
					filtered = append(filtered, d)
				}
			}
			allDriftedResources = filtered
		}

		printutils.PrintKarpenterDrift(noHeaders, allDriftedResources...)

		saveCacheToDisk()
	},
}

func init() {
	karpenterDriftCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	karpenterDriftCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	karpenterDriftCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	karpenterDriftCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	karpenterDriftCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	karpenterDriftCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	karpenterDriftCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	karpenterDriftCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	karpenterDriftCmd.Flags().StringP("nodepool-contains", "m", "", "Filter by NodePool name substring")

	karpenterCmd.AddCommand(karpenterDriftCmd)
}
