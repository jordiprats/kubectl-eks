package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/ec2"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var nodegroupsCmd = &cobra.Command{
	Use:     "nodegroups",
	Aliases: []string{"ng"},
	Short:   "List EKS managed node groups",
	Long: `List EKS managed node groups with configuration and status details.

Displays node group name, status, instance types, scaling configuration
(min/max/desired size), AMI type, capacity type (On-Demand/Spot), and
current Kubernetes version.

Use this to audit node group configurations and identify scaling settings.
When cluster filters are provided, queries multiple clusters.
Without filters, queries the current cluster context.`,
	Example: `  # List nodegroups for current cluster
  kubectl eks nodegroups

  # Filter by profile substring
  kubectl eks nodegroups --profile-contains genprod

  # Filter by cluster name substring
  kubectl eks nodegroups --cluster-contains v2-b

  # Exclude clusters by name substring
  kubectl eks nodegroups --cluster-not-contains staging

  # Filter by region
  kubectl eks nodegroups --region us-west-2

  # Combine filters
  kubectl eks nodegroups -q genprod -c v2-b -x orch -r us-west-2`,
	Run: func(cmd *cobra.Command, args []string) {
		noHeaders, _ := cmd.Flags().GetBool("no-headers")
		refresh, _ := cmd.Flags().GetBool("refresh")
		ami, _ := cmd.Flags().GetString("ami")

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

		if ami != "" {
			for _, clusterInfo := range clusterList {
				amiInfo, err := ec2.GetAMIInfo(clusterInfo.AWSProfile, clusterInfo.Region, ami)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error getting AMI info for cluster %s: %s\n", clusterInfo.ClusterName, err.Error())
					continue
				}
				printutils.PrintAMIs(noHeaders, *amiInfo)
			}
		} else {
			allNodeGroups := []eks.EKSNodeGroupInfo{}
			for _, clusterInfo := range clusterList {
				clusterNGList, err := eks.GetEKSNodeGroups(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error listing nodegroups for cluster %s: %s\n", clusterInfo.ClusterName, err.Error())
					continue
				}
				allNodeGroups = append(allNodeGroups, clusterNGList...)
			}
			printutils.PrintNodeGroup(noHeaders, allNodeGroups...)
		}

		if hasFilters {
			saveCacheToDisk()
		}
	},
}

func init() {
	nodegroupsCmd.Flags().StringP("ami", "a", "", "Describe AMI used by the nodegroup")
	nodegroupsCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	nodegroupsCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	nodegroupsCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	nodegroupsCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	nodegroupsCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	nodegroupsCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	nodegroupsCmd.Flags().StringP("version", "v", "", "Filter by EKS version")

	rootCmd.AddCommand(nodegroupsCmd)
}
