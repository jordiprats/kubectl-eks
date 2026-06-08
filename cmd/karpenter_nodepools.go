package cmd

import (
	"log"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/karpenter"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var karpenterNodePoolsCmd = &cobra.Command{
	Use:     "nodepools",
	Aliases: []string{"np", "nodepool"},
	Short:   "List Karpenter NodePools across clusters",
	Long: `List Karpenter NodePools across all clusters that match a filter.

Shows instance type constraints, resource limits, disruption settings,
and associated NodeClass information.`,
	Example: `  # List NodePools for current cluster
  kubectl eks karpenter nodepools

  # List NodePools across clusters matching filter
  kubectl eks karpenter nodepools --cluster-contains prod

  # List NodePools with wide output
  kubectl eks karpenter nodepools -o wide`,
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
		output, _ := cmd.Flags().GetString("output")

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

		allNodePools := []data.KarpenterNodePoolInfo{}

		for _, clusterInfo := range clusterList {
			restConfig, err := GetRestConfigForCluster(clusterInfo)
			if err != nil {
				if verbose {
					log.Printf("Warning: Failed to get kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
				}
				continue
			}

			nodePools, err := karpenter.GetNodePoolsWithConfig(restConfig, clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
			if err != nil {
				if verbose {
					log.Printf("Warning: Failed to get NodePools from cluster %s: %v", clusterInfo.ClusterName, err)
				}
				continue
			}

			allNodePools = append(allNodePools, nodePools...)
		}

		printutils.PrintKarpenterNodePools(noHeaders, output == "wide", allNodePools...)

		saveCacheToDisk()
	},
}

func init() {
	karpenterNodePoolsCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	karpenterNodePoolsCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	karpenterNodePoolsCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	karpenterNodePoolsCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	karpenterNodePoolsCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	karpenterNodePoolsCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	karpenterNodePoolsCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	karpenterNodePoolsCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	karpenterNodePoolsCmd.Flags().StringP("output", "o", "", "Output format: wide")

	karpenterCmd.AddCommand(karpenterNodePoolsCmd)
}
