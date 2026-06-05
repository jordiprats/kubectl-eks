package cmd

import (
	"fmt"
	"log"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var fargateProfilesCmd = &cobra.Command{
	Use:     "fargate-profiles",
	Aliases: []string{"fp", "fargate"},
	Short:   "List EKS Fargate profiles and their selectors",
	Long: `List EKS Fargate profiles with namespace selectors and configuration.

Displays Fargate profile name, status, pod execution role ARN, subnets,
and namespace/label selectors that determine which pods run on Fargate.

When cluster filters are provided, queries multiple clusters.
Without filters, queries the current cluster context.`,
	Run: func(cmd *cobra.Command, args []string) {
		noHeaders, _ := cmd.Flags().GetBool("no-headers")
		refresh, _ := cmd.Flags().GetBool("refresh")

		// Get filter flags
		profile, _ := cmd.Flags().GetString("profile")
		profileContains, _ := cmd.Flags().GetString("profile-contains")
		profileNotContains, _ := cmd.Flags().GetString("profile-not-contains")
		nameContains, _ := cmd.Flags().GetString("cluster-contains")
		nameNotContains, _ := cmd.Flags().GetString("cluster-not-contains")
		region, _ := cmd.Flags().GetString("region")
		version, _ := cmd.Flags().GetString("version")

		// Check if any filter is specified
		hasFilters := profile != "" || profileContains != "" || profileNotContains != "" || nameContains != "" ||
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

		if len(clusterList) == 0 {
			fmt.Println("No clusters found matching the specified filters")
			return
		}

		allProfiles := []eks.FargateProfileInfo{}
		for _, clusterInfo := range clusterList {
			profileList, err := eks.GetEKSFargateProfiles(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
			if err != nil {
				fmt.Fprintf(log.Writer(), "Error listing Fargate profiles for cluster %s: %s\n", clusterInfo.ClusterName, err.Error())
				continue
			}
			allProfiles = append(allProfiles, profileList...)
		}

		printutils.PrintFargateProfiles(noHeaders, allProfiles...)

		if hasFilters {
			saveCacheToDisk()
		}
	},
}

func init() {
	fargateProfilesCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	fargateProfilesCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	fargateProfilesCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	fargateProfilesCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	fargateProfilesCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	fargateProfilesCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	fargateProfilesCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	fargateProfilesCmd.Flags().StringP("version", "v", "", "Filter by EKS version")

	rootCmd.AddCommand(fargateProfilesCmd)
}
