package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

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
			runMultiClusterNodes(clusterList, noHeaders, output == "wide", false, managedByContains)
		} else {
			// No filters - use current context directly
			clusterInfo, err := GetCurrentClusterInfo()
			if err != nil {
				log.Fatalf("Error getting current cluster info: %v", err)
			}
			clusterList = []data.ClusterInfo{clusterInfo}
			runMultiClusterNodes(clusterList, noHeaders, output == "wide", true, managedByContains)
		}
	},
}

func runMultiClusterNodes(clusterList []data.ClusterInfo, noHeaders bool, wide bool, skipContextSwitch bool, managedByContains string) {
	if len(clusterList) == 0 {
		fmt.Println("No clusters found matching the specified filters")
		return
	}

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
			// Use current context directly
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

	rootCmd.AddCommand(nodesCmd)
}
