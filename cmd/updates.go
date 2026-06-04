package cmd

import (
	"fmt"
	"log"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var updatesCmd = &cobra.Command{
	Use:   "updates",
	Short: "Check for available Kubernetes and add-on updates",
	Long: `Check for available Kubernetes version updates and EKS add-on updates.

Displays current versions and available updates for:
  - Kubernetes control plane
  - EKS managed add-ons (VPC CNI, CoreDNS, kube-proxy, etc.)
  - Platform version

When cluster filters are provided, queries multiple clusters.
Without filters, queries the current cluster context.`,
	Run: func(cmd *cobra.Command, args []string) {
		noHeaders, _ := cmd.Flags().GetBool("no-headers")
		refresh, _ := cmd.Flags().GetBool("refresh")

		// Get filter flags
		profile, _ := cmd.Flags().GetString("profile")
		profileContains, _ := cmd.Flags().GetString("profile-contains")
		nameContains, _ := cmd.Flags().GetString("cluster-contains")
		nameNotContains, _ := cmd.Flags().GetString("cluster-not-contains")
		region, _ := cmd.Flags().GetString("region")
		version, _ := cmd.Flags().GetString("version")

		// Check if any filter is specified
		hasFilters := profile != "" || profileContains != "" || nameContains != "" ||
			nameNotContains != "" || region != "" || version != ""

		var clusterList []data.ClusterInfo

		if hasFilters {
			loadCacheFromDisk()
			if CachedData == nil {
				CachedData = &data.KubeCtlEksCache{
					ClusterByARN: make(map[string]data.ClusterInfo),
					ClusterList:  make(map[string]map[string][]data.ClusterInfo),
				}
			}
			if CachedData.ClusterList == nil {
				CachedData.ClusterList = make(map[string]map[string][]data.ClusterInfo)
			}

			var err error
			clusterList, err = LoadClusterList([]string{}, profile, profileContains, nameContains, nameNotContains, region, version, refresh)
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

		if len(clusterList) == 0 {
			fmt.Println("No clusters found matching the specified filters")
			return
		}

		allUpdates := []eks.EKSUpdateInfo{}
		for _, clusterInfo := range clusterList {
			updateList, err := eks.GetEKSUpdates(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
			if err != nil {
				fmt.Fprintf(log.Writer(), "Error listing updates for cluster %s: %s\n", clusterInfo.ClusterName, err.Error())
				continue
			}
			allUpdates = append(allUpdates, updateList...)
		}

		printutils.PrintUpdates(noHeaders, allUpdates...)

		if hasFilters {
			saveCacheToDisk()
		}
	},
}

func init() {
	updatesCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	updatesCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	updatesCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	updatesCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	updatesCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	updatesCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	updatesCmd.Flags().StringP("version", "v", "", "Filter by EKS version")

	rootCmd.AddCommand(updatesCmd)
}
