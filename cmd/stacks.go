package cmd

import (
	"fmt"
	"log"

	"github.com/jordiprats/kubectl-eks/pkg/cf"
	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var stacksCmd = &cobra.Command{
	Use:   "stacks",
	Short: "List CloudFormation stacks associated with EKS clusters",
	Long: `List CloudFormation stacks related to EKS clusters and their resources.

Shows stack name, status, creation time, and parameters. Useful for
identifying stacks managing EKS node groups, VPC resources, or other
EKS-related infrastructure.

By default, shows stacks for the current cluster. Use filters to query
stacks across multiple clusters or search by stack name/parameters.

When cluster filters are provided, queries multiple clusters.
Without filters, queries the current cluster context.`,
	Run: func(cmd *cobra.Command, args []string) {
		searchName, _ := cmd.Flags().GetString("name")
		paramFilter, _ := cmd.Flags().GetBool("by-parameter")
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

		allStacks := []cf.StackInfo{}
		for _, clusterInfo := range clusterList {
			clusterSearchName := searchName
			if clusterSearchName == "" {
				clusterSearchName = clusterInfo.ClusterName
			}

			var stackList []cf.StackInfo
			var err error
			if paramFilter {
				stackList, err = cf.GetStacksByParameter("ClusterName", clusterSearchName, clusterInfo.AWSProfile, clusterInfo.Region)
			} else {
				stackList, err = cf.GetStacks(clusterSearchName, clusterInfo.AWSProfile, clusterInfo.Region)
			}

			if err != nil {
				fmt.Fprintf(log.Writer(), "Error getting CF stacks for cluster %s: %v\n", clusterInfo.ClusterName, err)
				continue
			}
			allStacks = append(allStacks, stackList...)
		}

		printutils.PrintStacks(noHeaders, allStacks...)

		if hasFilters {
			saveCacheToDisk()
		}
	},
}

func init() {
	stacksCmd.Flags().String("name", "", "Search for a specific stack name")
	stacksCmd.Flags().BoolP("by-parameter", "b", false, "Filter stacks by ClusterName parameter instead of stack name")
	stacksCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	stacksCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	stacksCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	stacksCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	stacksCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	stacksCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	stacksCmd.Flags().StringP("version", "v", "", "Filter by EKS version")

	rootCmd.AddCommand(stacksCmd)
}
