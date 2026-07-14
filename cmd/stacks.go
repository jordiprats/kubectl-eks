package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		watchInterval, _ := cmd.Flags().GetDuration("watch")

		// Get filter flags
		profile, _ := cmd.Flags().GetString("profile")
		profileContains, _ := cmd.Flags().GetString("profile-contains")
		profileNotContains, _ := cmd.Flags().GetString("profile-not-contains")
		nameContains, _ := cmd.Flags().GetString("cluster-contains")
		nameNotContains, _ := cmd.Flags().GetString("cluster-not-contains")
		region, _ := cmd.Flags().GetString("region")
		version, _ := cmd.Flags().GetString("version")

		if watchInterval > 0 && !printutils.IsTTY() {
			log.Fatal("--watch requires an interactive terminal")
		}

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

		collectStacks := func() []cf.StackInfo {
			var allStacks []cf.StackInfo
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
				for i := range stackList {
					stackList[i].Profile = clusterInfo.AWSProfile
					stackList[i].Region = clusterInfo.Region
					stackList[i].ClusterName = clusterInfo.ClusterName
				}
				allStacks = append(allStacks, stackList...)
			}
			return allStacks
		}

		if watchInterval > 0 {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			effectiveInterval := watchInterval
			for {
				start := time.Now()
				stacks := collectStacks()
				elapsed := time.Since(start)

				printutils.ClearScreen()
				fmt.Printf("Every %s: kubectl eks stacks (last: %s)\n\n", effectiveInterval, time.Now().Format("15:04:05"))
				printutils.PrintStacksColored(stacks)

				nextInterval := watchInterval
				if twice := 2 * elapsed; twice > nextInterval {
					nextInterval = twice
				}
				effectiveInterval = nextInterval

				timer := time.NewTimer(nextInterval)
				select {
				case <-sigCh:
					timer.Stop()
					fmt.Println()
					return
				case <-timer.C:
				}
			}
		} else {
			printutils.PrintStacks(noHeaders, collectStacks()...)

			if hasFilters {
				saveCacheToDisk()
			}
		}
	},
}

func init() {
	stacksCmd.Flags().String("name", "", "Search for a specific stack name")
	stacksCmd.Flags().BoolP("by-parameter", "b", false, "Filter stacks by ClusterName parameter instead of stack name")
	stacksCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	stacksCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	stacksCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	stacksCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	stacksCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	stacksCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	stacksCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	stacksCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	stacksCmd.Flags().DurationP("watch", "w", 0, "Watch mode: refresh every interval (default 30s, e.g. -w 5s)")
	stacksCmd.Flags().Lookup("watch").NoOptDefVal = "30s"

	rootCmd.AddCommand(stacksCmd)
}
