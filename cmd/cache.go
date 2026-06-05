package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/awsconfig"
	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the local cluster cache",
	Long: `Manage the local cluster cache used to speed up commands like 'use' and 'list'.

The cache stores cluster metadata (names, ARNs, profiles, regions) in
~/.kube/.kubectl-eks-cache so that repeated operations can skip expensive
AWS API calls.`,
}

var cacheRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh the local cluster cache from AWS",
	Long: `Fetch all EKS clusters from all configured AWS profiles and regions,
populating the local cache. Subsequent commands like 'use' and 'list' will
be significantly faster.

Use --profile or --region to limit the refresh scope.`,
	Run: func(cmd *cobra.Command, args []string) {
		profile, _ := cmd.Flags().GetString("profile")
		profileContains, _ := cmd.Flags().GetString("profile-contains")
		profileNotContains, _ := cmd.Flags().GetString("profile-not-contains")
		region, _ := cmd.Flags().GetString("region")

		loadCacheFromDisk()
		if CachedData == nil {
			CachedData = &data.KubeCtlEksCache{
				ClusterByARN: make(map[string]data.ClusterInfo),
				ClusterList:  make(map[string]map[string][]data.ClusterInfo),
			}
		}

		// Clear the cluster list portion so everything is re-fetched
		CachedData.ClusterList = make(map[string]map[string][]data.ClusterInfo)

		clusterCount := 0
		profileCount := 0

		awsProfiles := awsconfig.GetAWSProfilesWithEKSHints()
		for _, profileDetails := range awsProfiles {
			if profile != "" && profile != profileDetails.Name {
				continue
			}
			if profileContains != "" && !strings.Contains(profileDetails.Name, profileContains) {
				continue
			}
			if profileNotContains != "" && strings.Contains(profileDetails.Name, profileNotContains) {
				continue
			}

			profileCount++
			for _, hintRegion := range profileDetails.HintEKSRegions {
				if region != "" && region != hintRegion {
					continue
				}

				fmt.Fprintf(os.Stderr, "Loading clusters: profile=%s region=%s\n", profileDetails.Name, hintRegion)
				loadClusters(profileDetails.Name, hintRegion)

				if clusters, ok := CachedData.ClusterList[profileDetails.Name][hintRegion]; ok {
					for _, c := range clusters {
						clusterCount++
						CachedData.ClusterByARN[c.Arn] = c
					}
				}
			}
		}

		saveCacheToDisk()
		fmt.Fprintf(os.Stderr, "Cache refreshed: %d clusters from %d profiles\n", clusterCount, profileCount)
	},
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the local cluster cache",
	Long:  `Remove the local cache file (~/.kube/.kubectl-eks-cache). The next command that needs cluster data will fetch it from AWS.`,
	Run: func(cmd *cobra.Command, args []string) {
		configFile := HomeDir + "/.kube/.kubectl-eks-cache"
		if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Failed to remove cache file: %s\n", err.Error())
			os.Exit(1)
		}
		CachedData = nil
		fmt.Println("Cache cleared")
	},
}

var cacheShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show cached clusters",
	Long:  `Display all clusters currently stored in the local cache.`,
	Run: func(cmd *cobra.Command, args []string) {
		loadCacheFromDisk()
		if CachedData == nil {
			fmt.Println("Cache is empty")
			return
		}

		clusters := []data.ClusterInfo{}

		// Collect unique clusters from ClusterByARN
		seen := make(map[string]bool)
		for arn, info := range CachedData.ClusterByARN {
			if !seen[arn] {
				seen[arn] = true
				clusters = append(clusters, info)
			}
		}

		// Also include clusters from ClusterList that might not be in ClusterByARN
		for _, regions := range CachedData.ClusterList {
			for _, clusterList := range regions {
				for _, c := range clusterList {
					if c.Arn != "" && !seen[c.Arn] {
						seen[c.Arn] = true
						clusters = append(clusters, c)
					}
				}
			}
		}

		if len(clusters) == 0 {
			fmt.Println("Cache is empty")
			return
		}

		fmt.Fprintf(os.Stderr, "%d cached clusters:\n\n", len(clusters))
		printutils.PrintClusters(false, clusters...)
	},
}

func init() {
	cacheRefreshCmd.Flags().StringP("profile", "p", "", "Only refresh clusters for this AWS profile")
	cacheRefreshCmd.Flags().StringP("profile-contains", "q", "", "Only refresh profiles containing this string")
	cacheRefreshCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	cacheRefreshCmd.Flags().StringP("region", "r", "", "Only refresh clusters in this AWS region")

	cacheCmd.AddCommand(cacheRefreshCmd)
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheShowCmd)
	rootCmd.AddCommand(cacheCmd)
}
