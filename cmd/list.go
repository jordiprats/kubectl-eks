package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/awsconfig"
	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/jordiprats/kubectl-eks/pkg/sts"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/tools/clientcmd"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all EKS clusters in your AWS account",
	Long: `List all EKS clusters in your AWS account with optional filters.
You can filter by cluster name, region, version, or AWS profile.`,
	Example: `  # List all clusters
  kubectl eks list
  
  # Filter by name
  kubectl eks list --name-contains dev
  
  # Filter by region
  kubectl eks list --region us-east-1
  
  # Filter by version and profile
  kubectl eks list --version 1.29 --profile profile-1
  
  # List only cluster ARNs
  kubectl eks list -1`,
	Run: func(cmd *cobra.Command, args []string) {
		refresh, err := cmd.Flags().GetBool("refresh")
		if err != nil {
			refresh = false
		}

		profile, err := cmd.Flags().GetString("profile")
		if err != nil {
			profile = ""
		}

		profile_contains, err := cmd.Flags().GetString("profile-contains")
		if err != nil {
			profile_contains = ""
		}

		name_contains, err := cmd.Flags().GetString("name-contains")
		if err != nil {
			name_contains = ""
		}

		name_not_contains, err := cmd.Flags().GetString("name-not-contains")
		if err != nil {
			name_not_contains = ""
		}

		region, err := cmd.Flags().GetString("region")
		if err != nil {
			region = ""
		}

		version, err := cmd.Flags().GetString("version")
		if err != nil {
			version = ""
		}

		arnOnly, err := cmd.Flags().GetBool("arn-only")
		if err != nil {
			arnOnly = false
		}

		nameOnly, err := cmd.Flags().GetBool("name-only")
		if err != nil {
			nameOnly = false
		}

		output, err := cmd.Flags().GetString("output")
		if err != nil {
			output = ""
		}
		wide := output == "wide"

		loadCacheFromDisk()
		if CachedData == nil {
			CachedData = &data.KubeCtlEksCache{
				ClusterByARN: make(map[string]data.ClusterInfo),
				ClusterList:  make(map[string]map[string][]data.ClusterInfo),
			}
		}

		if refresh {
			CachedData.ClusterList = make(map[string]map[string][]data.ClusterInfo)
		}

		clusterList := []data.ClusterInfo{}

		awsProfiles := awsconfig.GetAWSProfilesWithEKSHints()
		for _, profileDetails := range awsProfiles {
			if profile != "" && profile != profileDetails.Name {
				continue
			}
			if profile_contains != "" && !strings.Contains(profileDetails.Name, profile_contains) {
				continue
			}
			for _, hintRegion := range profileDetails.HintEKSRegions {
				if region != "" && region != hintRegion {
					continue
				}

				if refresh {
					_, exists := CachedData.ClusterList[profileDetails.Name]
					if !exists {
						CachedData.ClusterList[profileDetails.Name] = make(map[string][]data.ClusterInfo)
					}
					_, exists = CachedData.ClusterList[profileDetails.Name][hintRegion]
					if !exists {
						CachedData.ClusterList[profileDetails.Name][hintRegion] = []data.ClusterInfo{}
					}
					loadClusters(profileDetails.Name, hintRegion)
				} else {
					cachedRegions, exists := CachedData.ClusterList[profileDetails.Name]
					if !exists {
						loadClusters(profileDetails.Name, hintRegion)
					} else {
						_, exists := cachedRegions[hintRegion]
						if !exists {
							loadClusters(profileDetails.Name, hintRegion)
						}
					}
				}

				currentClusterList, exists := CachedData.ClusterList[profileDetails.Name][hintRegion]
				if !exists {
					fmt.Fprintf(os.Stderr, "Unable to load clusters using profile: %s region: %s (listCmd)\n", profileDetails.Name, hintRegion)
				} else {
					if version == "" && name_contains == "" && name_not_contains == "" {
						clusterList = append(clusterList, currentClusterList...)
					} else {
						for _, cluster := range currentClusterList {
							// checking filter criteria
							shouldAdd := true

							// Check version filter
							if version != "" && cluster.Version != version {
								shouldAdd = false
							}

							// Check name_contains filter
							if name_contains != "" && !strings.Contains(cluster.ClusterName, name_contains) {
								shouldAdd = false
							}

							// Check name_not_contains filter
							if name_not_contains != "" && strings.Contains(cluster.ClusterName, name_not_contains) {
								shouldAdd = false
							}

							// only add the cluster if it meets the criteria
							if shouldAdd {
								clusterList = append(clusterList, cluster)
							}
						}
					}
				}

			}
		}

		if arnOnly {
			for _, cluster := range clusterList {
				fmt.Println(cluster.Arn)
			}
		} else if nameOnly {
			for _, cluster := range clusterList {
				fmt.Println(cluster.ClusterName)
			}
		} else {
			if wide {
				clusterList = enrichClusterNodeStats(clusterList)
			}

			noHeaders, err := cmd.Flags().GetBool("no-headers")
			if err != nil {
				noHeaders = false
			}

			printutils.PrintClustersWithOptions(noHeaders, wide, clusterList...)
		}

		saveCacheToDisk()
	},
}

func loadClusters(profile, region string) {
	// fmt.Printf("Loading clusters using profile: %s region: %s\n", profile, region)

	// Ensure the cache has an entry for this profile/region even when no clusters are found.
	if _, exists := CachedData.ClusterList[profile]; !exists {
		CachedData.ClusterList[profile] = make(map[string][]data.ClusterInfo)
	}
	if _, exists := CachedData.ClusterList[profile][region]; !exists {
		CachedData.ClusterList[profile][region] = []data.ClusterInfo{}
	}

	// Get the list of clusters
	clusters, err := eks.GetClusters(profile, region)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing clusters: %v\n", err.Error())
		os.Exit(1)
	}

	accountID, err := sts.GetAccountID(profile, region)
	if err != nil {
		accountID = "-"
	}

	for _, cluster := range clusters {
		if cluster == nil {
			continue
		}

		clusterData := data.ClusterInfo{
			ClusterName:  *cluster,
			Region:       region,
			AWSProfile:   profile,
			AWSAccountID: accountID,
		}

		clusterInfo, err := eks.DescribeCluster(profile, region, *cluster)
		if err != nil || clusterInfo == nil {
			fmt.Fprintf(os.Stderr, "Error describing cluster %s: %v\n", *cluster, err.Error())
		} else {
			clusterData.Status = string(clusterInfo.Status)
			clusterData.Version = *clusterInfo.Version
			clusterData.Arn = *clusterInfo.Arn
			clusterData.CreatedAt = clusterInfo.CreatedAt.Format("2006-01-02 15:04:05")
		}

		// CachedData.ClusterInfo[clusterName] = clusterInfo

		// fmt.Printf("Adding cluster %s to profile %s and region %s\n", clusterData.ClusterName, profile, region)
		CachedData.ClusterList[profile][region] = append(CachedData.ClusterList[profile][region], clusterData)
	}

}

func init() {
	listCmd.Flags().BoolP("refresh", "u", false, "Refresh data from AWS")
	listCmd.Flags().StringP("profile", "p", "", "AWS profile to use")
	listCmd.Flags().StringP("profile-contains", "q", "", "AWS profile contains string")
	listCmd.Flags().StringP("name-contains", "c", "", "Cluster name contains string")
	listCmd.Flags().StringP("name-not-contains", "x", "", "Cluster name does not contain string")
	listCmd.Flags().StringP("region", "r", "", "AWS region to use")
	listCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	listCmd.Flags().BoolP("arn-only", "1", false, "Output only cluster ARNs, one per line")
	listCmd.Flags().BoolP("name-only", "2", false, "Output only cluster names, one per line")
	listCmd.Flags().StringP("output", "o", "", "Output format: wide")

	rootCmd.AddCommand(listCmd)
}

func enrichClusterNodeStats(clusterList []data.ClusterInfo) []data.ClusterInfo {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := loadingRules.Load()
	if err != nil {
		return clusterList
	}

	previousContext := config.CurrentContext
	defer func() {
		config.CurrentContext = previousContext
		_ = clientcmd.ModifyConfig(loadingRules, *config, true)
	}()

	for i := range clusterList {
		cluster := &clusterList[i]

		err := eks.UpdateKubeConfig(cluster.AWSProfile, cluster.Region, cluster.ClusterName, "")
		if err != nil {
			continue
		}

		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		)

		restConfig, err := clientConfig.ClientConfig()
		if err != nil {
			continue
		}

		nodes, err := k8s.GetNodesWithConfig(restConfig)
		if err != nil {
			continue
		}

		cluster.NodeCount = len(nodes)
		totalCPUUsedMilli := int64(0)
		totalCPUCapacityMilli := int64(0)
		totalCPUAllocMilli := int64(0)
		totalMemUsedBytes := int64(0)
		totalMemCapacityBytes := int64(0)
		totalMemAllocBytes := int64(0)

		for _, node := range nodes {
			if strings.HasPrefix(node.Status, "Ready") {
				cluster.NodeReady++
			} else {
				cluster.NodeNotReady++
			}

			if strings.Contains(node.Status, "SchedulingDisabled") {
				cluster.NodeSchedDisabled++
			}

			if q, err := resource.ParseQuantity(node.CPUUsed); err == nil {
				totalCPUUsedMilli += q.MilliValue()
			}
			if q, err := resource.ParseQuantity(node.CPUCapacity); err == nil {
				totalCPUCapacityMilli += q.MilliValue()
			}
			if q, err := resource.ParseQuantity(node.CPUAllocatable); err == nil {
				totalCPUAllocMilli += q.MilliValue()
			}

			if q, err := resource.ParseQuantity(node.MemoryUsed); err == nil {
				totalMemUsedBytes += q.Value()
			}
			if q, err := resource.ParseQuantity(node.MemoryCapacity); err == nil {
				totalMemCapacityBytes += q.Value()
			}
			if q, err := resource.ParseQuantity(node.MemoryAllocatable); err == nil {
				totalMemAllocBytes += q.Value()
			}
		}

		cluster.CPUUsedTotal = resource.NewMilliQuantity(totalCPUUsedMilli, resource.DecimalSI).String()
		cluster.CPUCapacityTotal = resource.NewMilliQuantity(totalCPUCapacityMilli, resource.DecimalSI).String()
		cluster.CPUAllocatableTotal = resource.NewMilliQuantity(totalCPUAllocMilli, resource.DecimalSI).String()

		cluster.MemoryUsedTotal = resource.NewQuantity(totalMemUsedBytes, resource.BinarySI).String()
		cluster.MemoryCapacityTotal = resource.NewQuantity(totalMemCapacityBytes, resource.BinarySI).String()
		cluster.MemoryAllocatableTotal = resource.NewQuantity(totalMemAllocBytes, resource.BinarySI).String()
	}

	return clusterList
}
