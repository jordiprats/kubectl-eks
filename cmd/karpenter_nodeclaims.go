package cmd

import (
	"log"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/karpenter"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var karpenterNodeClaimsCmd = &cobra.Command{
	Use:     "nodeclaims",
	Aliases: []string{"nc", "nodeclaim"},
	Short:   "List Karpenter NodeClaims across clusters",
	Long: `List active Karpenter NodeClaims across all clusters that match a filter.

Shows provisioning status, instance type, AMI, capacity type, zone,
and associated NodePool for each NodeClaim.`,
	Example: `  # List NodeClaims for current cluster
  kubectl eks karpenter nodeclaims

  # List NodeClaims across clusters matching filter
  kubectl eks karpenter nodeclaims --cluster-contains prod

  # List NodeClaims with wide output
  kubectl eks karpenter nodeclaims -o wide`,
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

		allNodeClaims := []data.KarpenterNodeClaimInfo{}

		for _, clusterInfo := range clusterList {
			restConfig, err := GetRestConfigForCluster(clusterInfo)
			if err != nil {
				if verbose {
					log.Printf("Warning: Failed to get kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
				}
				continue
			}

			nodeClaims, err := karpenter.GetNodeClaimsWithConfig(restConfig, clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
			if err != nil {
				if verbose {
					log.Printf("Warning: Failed to get NodeClaims from cluster %s: %v", clusterInfo.ClusterName, err)
				}
				continue
			}

			allNodeClaims = append(allNodeClaims, nodeClaims...)
		}

		if nodepoolContains != "" {
			filtered := allNodeClaims[:0]
			for _, nc := range allNodeClaims {
				if strings.Contains(strings.ToLower(nc.NodePoolName), strings.ToLower(nodepoolContains)) {
					filtered = append(filtered, nc)
				}
			}
			allNodeClaims = filtered
		}

		printutils.PrintKarpenterNodeClaims(noHeaders, output == "wide", allNodeClaims...)

		saveCacheToDisk()
	},
}

func init() {
	karpenterNodeClaimsCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	karpenterNodeClaimsCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	karpenterNodeClaimsCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	karpenterNodeClaimsCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	karpenterNodeClaimsCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	karpenterNodeClaimsCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	karpenterNodeClaimsCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	karpenterNodeClaimsCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	karpenterNodeClaimsCmd.Flags().StringP("output", "o", "", "Output format: wide")
	karpenterNodeClaimsCmd.Flags().StringP("nodepool-contains", "m", "", "Filter by NodePool name substring")

	karpenterCmd.AddCommand(karpenterNodeClaimsCmd)
}
