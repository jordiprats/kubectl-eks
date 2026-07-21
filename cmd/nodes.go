package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var daysPattern = regexp.MustCompile(`^(\d+)d(.*)$`)

func parseDurationWithDays(s string) (time.Duration, error) {
	total := time.Duration(0)
	rest := strings.TrimSpace(s)
	if m := daysPattern.FindStringSubmatch(rest); m != nil {
		days, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		total += time.Duration(days) * 24 * time.Hour
		rest = m[2]
	}
	if rest != "" {
		d, err := time.ParseDuration(rest)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		total += d
	}
	if total == 0 && s != "0" {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	return total, nil
}

var nodesCmd = &cobra.Command{
	Use:     "nodes",
	Aliases: []string{"node"},
	Short:   "List Kubernetes nodes with EC2 instance details",
	Long: `List Kubernetes nodes across EKS clusters with EC2 instance metadata.

Shows node status, instance type, capacity (CPU/memory), provider ID,
and other node attributes. Queries both Kubernetes and AWS APIs for
complete node information.

When cluster filters are provided, queries multiple clusters.
Without filters, queries the current cluster context.`,
	Example: `  # List nodes for current cluster
  kubectl eks nodes

	# List nodes for current cluster with pressure indicators
	kubectl eks nodes -o wide

  # List nodes across clusters matching filter
  kubectl eks nodes --cluster-contains prod

  # List nodes for specific profile
  kubectl eks nodes --profile my-aws-profile

  # List nodes across all clusters in a region
  kubectl eks nodes --region us-west-2`,
	Run: func(cmd *cobra.Command, args []string) {
		noHeaders, _ := cmd.Flags().GetBool("no-headers")
		refresh, _ := cmd.Flags().GetBool("refresh")
		output, _ := cmd.Flags().GetString("output")
		managedByContains, _ := cmd.Flags().GetString("managed-by")
		watchInterval, _ := cmd.Flags().GetDuration("watch")
		olderStr, _ := cmd.Flags().GetString("older")

		var olderThan time.Duration
		if olderStr != "" {
			var err error
			olderThan, err = parseDurationWithDays(olderStr)
			if err != nil {
				log.Fatalf("Invalid --older value: %v", err)
			}
		}

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
		skipContextSwitch := false

		if hasFilters {
			// Ensure cache is initialized before LoadClusterList
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
			skipContextSwitch = true
		}

		if watchInterval > 0 {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			effectiveInterval := watchInterval
			for {
				start := time.Now()
				allNodes := collectNodes(clusterList, skipContextSwitch, managedByContains, olderThan)
				elapsed := time.Since(start)

				printutils.ClearScreen()
				fmt.Printf("Every %s: kubectl eks nodes (last: %s)\n\n", effectiveInterval, time.Now().Format("15:04:05"))
				printutils.PrintMultiClusterNodesColored(output == "wide", allNodes)

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
			runMultiClusterNodes(clusterList, noHeaders, output == "wide", skipContextSwitch, managedByContains, olderThan)
		}
	},
}

func collectNodes(clusterList []data.ClusterInfo, skipContextSwitch bool, managedByContains string, olderThan time.Duration) []data.ClusterNodeInfo {
	allNodes := []data.ClusterNodeInfo{}

	for _, clusterInfo := range clusterList {
		var restConfig *rest.Config
		var err error

		if !skipContextSwitch {
			restConfig, err = GetRestConfigForCluster(clusterInfo)
			if err != nil {
				if verbose {
					log.Printf("Warning: Failed to get config for cluster %s: %v", clusterInfo.ClusterName, err)
				}
				continue
			}
		} else {
			clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				clientcmd.NewDefaultClientConfigLoadingRules(),
				&clientcmd.ConfigOverrides{},
			)
			restConfig, err = clientConfig.ClientConfig()
			if err != nil {
				if verbose {
					log.Printf("Warning: Failed to get client config for cluster %s: %v", clusterInfo.ClusterName, err)
				}
				continue
			}
		}

		nodeList, err := k8s.GetNodesWithConfig(restConfig)
		if err != nil {
			if verbose {
				log.Printf("Warning: Failed to get nodes from cluster %s: %v", clusterInfo.ClusterName, err)
			}
			continue
		}

		for _, node := range nodeList {
			allNodes = append(allNodes, data.ClusterNodeInfo{
				Profile:     clusterInfo.AWSProfile,
				Region:      clusterInfo.Region,
				ClusterName: clusterInfo.ClusterName,
				Node:        node,
			})
		}
	}

	if managedByContains != "" {
		filtered := allNodes[:0]
		for _, n := range allNodes {
			if strings.Contains(strings.ToLower(n.Node.ManagedBy), strings.ToLower(managedByContains)) {
				filtered = append(filtered, n)
			}
		}
		allNodes = filtered
	}

	if olderThan > 0 {
		cutoff := time.Now().Add(-olderThan)
		filtered := allNodes[:0]
		for _, n := range allNodes {
			if !n.Node.Created.IsZero() && n.Node.Created.Before(cutoff) {
				filtered = append(filtered, n)
			}
		}
		allNodes = filtered
	}

	return allNodes
}

func runMultiClusterNodes(clusterList []data.ClusterInfo, noHeaders bool, wide bool, skipContextSwitch bool, managedByContains string, olderThan time.Duration) {
	if len(clusterList) == 0 {
		fmt.Println("No clusters found matching the specified filters")
		return
	}

	allNodes := collectNodes(clusterList, skipContextSwitch, managedByContains, olderThan)
	printutils.PrintMultiClusterNodes(noHeaders, wide, allNodes)
	saveCacheToDisk()
}

func init() {
	nodesCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	nodesCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	nodesCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	nodesCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	nodesCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	nodesCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	nodesCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	nodesCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	nodesCmd.Flags().StringP("output", "o", "", "Output format: wide")
	nodesCmd.Flags().StringP("managed-by", "m", "", "Filter nodes by managed-by substring (e.g. karpenter, nodegroup, fargate)")
	nodesCmd.Flags().String("older", "", "Only show nodes older than this duration (e.g. 1d, 12h, 1d12h)")
	nodesCmd.Flags().DurationP("watch", "w", 0, "Watch mode: refresh every interval (default 30s, e.g. -w 5s)")
	nodesCmd.Flags().Lookup("watch").NoOptDefVal = "30s"

	rootCmd.AddCommand(nodesCmd)
}
