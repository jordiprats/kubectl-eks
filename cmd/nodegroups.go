package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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
  kubectl eks nodegroups -q genprod -c v2-b -x orch -r us-west-2

  # Watch nodegroups (refresh every 30s by default)
  kubectl eks nodegroups -w

  # Watch with custom interval
  kubectl eks nodegroups -w 5s`,
	Run: func(cmd *cobra.Command, args []string) {
		noHeaders, _ := cmd.Flags().GetBool("no-headers")
		refresh, _ := cmd.Flags().GetBool("refresh")
		ami, _ := cmd.Flags().GetString("ami")
		watchInterval, _ := cmd.Flags().GetDuration("watch")
		bootstrapRegex, _ := cmd.Flags().GetString("bootstrap-regex")
		output, _ := cmd.Flags().GetString("output")

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

		if ami != "" {
			for _, clusterInfo := range clusterList {
				amiInfo, err := ec2.GetAMIInfo(clusterInfo.AWSProfile, clusterInfo.Region, ami)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error getting AMI info for cluster %s: %s\n", clusterInfo.ClusterName, err.Error())
					continue
				}
				printutils.PrintAMIs(noHeaders, *amiInfo)
			}
		} else if watchInterval > 0 {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			effectiveInterval := watchInterval
			for {
				start := time.Now()
				allNodeGroups := []eks.EKSNodeGroupInfo{}
				for _, clusterInfo := range clusterList {
					clusterNGList, err := eks.GetEKSNodeGroups(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
					if err != nil {
						continue
					}
					allNodeGroups = append(allNodeGroups, clusterNGList...)
				}
				if bootstrapRegex != "" {
					filtered, err := eks.MatchBootstrapRegex(allNodeGroups, bootstrapRegex)
					if err != nil {
						log.Fatalf("Error filtering by bootstrap regex: %v", err)
					}
					allNodeGroups = filtered
				}
				elapsed := time.Since(start)

				multiCluster := false
				if len(allNodeGroups) > 0 {
					firstKey := allNodeGroups[0].Profile + "|" + allNodeGroups[0].Region + "|" + allNodeGroups[0].ClusterName
					for _, ng := range allNodeGroups[1:] {
						if ng.Profile+"|"+ng.Region+"|"+ng.ClusterName != firstKey {
							multiCluster = true
							break
						}
					}
				}

				printutils.ClearScreen()
				fmt.Printf("Every %s: kubectl eks nodegroups (last: %s)\n\n", effectiveInterval, time.Now().Format("15:04:05"))
				printutils.PrintNodeGroupColored(multiCluster, output == "wide", allNodeGroups...)

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
			allNodeGroups := []eks.EKSNodeGroupInfo{}
			for _, clusterInfo := range clusterList {
				clusterNGList, err := eks.GetEKSNodeGroups(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error listing nodegroups for cluster %s: %s\n", clusterInfo.ClusterName, err.Error())
					continue
				}
				allNodeGroups = append(allNodeGroups, clusterNGList...)
			}
			if bootstrapRegex != "" {
				filtered, err := eks.MatchBootstrapRegex(allNodeGroups, bootstrapRegex)
				if err != nil {
					log.Fatalf("Error filtering by bootstrap regex: %v", err)
				}
				allNodeGroups = filtered
			}
			printutils.PrintNodeGroup(noHeaders, output == "wide", allNodeGroups...)
		}

		if hasFilters {
			saveCacheToDisk()
		}
	},
}

func init() {
	nodegroupsCmd.Flags().StringP("ami", "a", "", "Describe AMI used by the nodegroup")
	nodegroupsCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	nodegroupsCmd.Flags().DurationP("watch", "w", 0, "Watch mode: refresh every interval (default 30s, e.g. -w 5s)")
	nodegroupsCmd.Flags().Lookup("watch").NoOptDefVal = "30s"
	nodegroupsCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	nodegroupsCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	nodegroupsCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	nodegroupsCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	nodegroupsCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	nodegroupsCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	nodegroupsCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	nodegroupsCmd.Flags().StringP("bootstrap-regex", "b", "", "Filter by regex match in bootstrap arguments (shows matched string)")
	nodegroupsCmd.Flags().StringP("output", "o", "", "Output format: wide")

	rootCmd.AddCommand(nodegroupsCmd)
}
