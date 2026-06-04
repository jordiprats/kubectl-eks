package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var insightsCmd = &cobra.Command{
	Use:   "insights",
	Short: "Show EKS cluster insights and recommendations",
	Long: `Display EKS Insights that provide recommendations and deprecation warnings.

EKS Insights help identify:
  - Deprecated API usage
  - Security findings
  - Best practice violations
  - Upgrade blockers
  - Configuration issues

Insights are categorized by severity and include remediation guidance to
help maintain cluster health and security.

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

		showID, _ := cmd.Flags().GetString("show")
		showID = strings.TrimSpace(showID)

		if showID == "" {
			allInsights := []data.EKSInsightInfo{}
			for _, clusterInfo := range clusterList {
				insightsList, err := eks.GetEKSInsights(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
				if err != nil {
					fmt.Fprintf(log.Writer(), "Error getting insights for cluster %s: %s\n", clusterInfo.ClusterName, err.Error())
					continue
				}
				allInsights = append(allInsights, insightsList...)
			}
			printutils.PrintInsights(noHeaders, allInsights...)
		} else {
			// --show only makes sense for a single cluster
			clusterInfo := clusterList[0]
			insightItem, err := eks.DescribeEKSInsight(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, showID)
			if err != nil {
				fmt.Printf("Error getting insight: %s\n", err.Error())
				return
			}

			fmt.Printf("Category: %s\n", insightItem.Category)
			fmt.Printf("Status: %s\n", insightItem.Status)
			fmt.Printf("Description: %s\n", insightItem.Description)
			fmt.Printf("Recommendation: %s\n", insightItem.Recommendation)
			if insightItem.AdditionalInfo != nil {
				if len(*insightItem.AdditionalInfo) > 0 {
					fmt.Printf("Additional Info:\n")
					for key, value := range *insightItem.AdditionalInfo {
						if value != nil {
							fmt.Printf("  * %s:\n      %s\n", key, *value)
						}
					}
				}
			}
			if len(insightItem.Summary.DeprecationDetails) > 0 {
				fmt.Printf("Deprecation Details:\n")
				for _, deprecation := range insightItem.Summary.DeprecationDetails {
					fmt.Printf("  * %q replaced with %q\n", deprecation.Usage, deprecation.ReplacedWith)
					fmt.Printf("    - Replacement from %s to %s\n", deprecation.StartServingReplacementVersion, deprecation.StopServingVersion)
					if len(deprecation.ClientStats) > 0 {
						fmt.Printf("    - Client Stats:\n")
						for _, clientStat := range deprecation.ClientStats {
							fmt.Printf("      * %s has requested %d in the last 30 days - last requested: %s\n", clientStat.UserAgent, clientStat.NumberOfRequestsLast30Days, clientStat.LastRequestTime.String())
						}
					}
				}
			} else {
				fmt.Printf("No deprecation details found\n")
			}
		}

		if hasFilters {
			saveCacheToDisk()
		}
	},
}

func init() {
	insightsCmd.Flags().String("show", "", "Show details for a specific ID")
	insightsCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	insightsCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	insightsCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	insightsCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	insightsCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	insightsCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	insightsCmd.Flags().StringP("version", "v", "", "Filter by EKS version")

	rootCmd.AddCommand(insightsCmd)
}
