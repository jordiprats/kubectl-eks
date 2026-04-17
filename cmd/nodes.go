package cmd

import (
	"fmt"
	"log"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
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
			clusterList, err = LoadClusterList([]string{}, profile, profileContains, nameContains, nameNotContains, region, version, refresh)
			if err != nil {
				log.Fatalf("Error loading cluster list: %v", err)
			}
			runMultiClusterNodes(clusterList, noHeaders, output == "wide", false)
		} else {
			// No filters - use current context directly
			clusterInfo, err := GetCurrentClusterInfo()
			if err != nil {
				log.Fatalf("Error getting current cluster info: %v", err)
			}
			clusterList = []data.ClusterInfo{clusterInfo}
			runMultiClusterNodes(clusterList, noHeaders, output == "wide", true)
		}
	},
}

func runMultiClusterNodes(clusterList []data.ClusterInfo, noHeaders bool, wide bool, skipContextSwitch bool) {
	if len(clusterList) == 0 {
		fmt.Println("No clusters found matching the specified filters")
		return
	}

	// Save and restore context only if we're switching clusters
	if !skipContextSwitch {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		config, err := loadingRules.Load()
		if err != nil {
			log.Fatalf("Error loading kubeconfig: %v", err)
		}
		previousContext := config.CurrentContext
		defer func() {
			config.CurrentContext = previousContext
			clientcmd.ModifyConfig(loadingRules, *config, true)
		}()
	}

	allNodes := []data.ClusterNodeInfo{}

	for _, clusterInfo := range clusterList {
		if !skipContextSwitch {
			err := eks.UpdateKubeConfig(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, "")
			if err != nil {
				log.Printf("Warning: Failed to update kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
				continue
			}
		}

		// Create fresh client config to pick up the switched context (same pattern as mget)
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		)

		restConfig, err := clientConfig.ClientConfig()
		if err != nil {
			log.Printf("Warning: Failed to get client config for cluster %s: %v", clusterInfo.ClusterName, err)
			continue
		}

		nodeList, err := k8s.GetNodesWithConfig(restConfig)
		if err != nil {
			log.Printf("Warning: Failed to get nodes from cluster %s: %v", clusterInfo.ClusterName, err)
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

	printutils.PrintMultiClusterNodes(noHeaders, wide, allNodes)

	saveCacheToDisk()
}

func init() {
	nodesCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	nodesCmd.Flags().StringP("profile", "p", "", "AWS profile to use")
	nodesCmd.Flags().StringP("profile-contains", "q", "", "AWS profile contains string")
	nodesCmd.Flags().StringP("cluster-contains", "c", "", "Cluster name contains string")
	nodesCmd.Flags().StringP("cluster-not-contains", "x", "", "Cluster name does not contain string")
	nodesCmd.Flags().StringP("region", "r", "", "AWS region to use")
	nodesCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	nodesCmd.Flags().StringP("output", "o", "", "Output format: wide")

	rootCmd.AddCommand(nodesCmd)
}
