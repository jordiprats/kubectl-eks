package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var quotasCmd = &cobra.Command{
	Use:   "quotas",
	Short: "Show ResourceQuota usage per namespace",
	Long: `Display ResourceQuota configurations and current usage across namespaces.

Shows hard limits and current usage for resources such as:
  - CPU requests/limits
  - Memory requests/limits
  - Pod counts
  - PersistentVolumeClaim counts
  - ConfigMap/Secret counts

When cluster filters are provided, queries multiple clusters.
Without filters, queries the current cluster context.`,
	Example: `  # Show quotas for current cluster
  kubectl eks quotas

  # Show quotas for specific namespace
  kubectl eks quotas -n production

  # Show quotas across all namespaces
  kubectl eks quotas -A

  # Show quotas across clusters matching filter
  kubectl eks quotas --cluster-contains prod`,
	Run: func(cmd *cobra.Command, args []string) {
		noHeaders, _ := cmd.Flags().GetBool("no-headers")
		refresh, _ := cmd.Flags().GetBool("refresh")
		namespace, _ := cmd.Flags().GetString("namespace")
		allNamespaces, _ := cmd.Flags().GetBool("all-namespaces")

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

		if !allNamespaces && namespace == "" {
			if !hasFilters {
				currentNs, err := k8s.GetCurrentNamespace()
				if err != nil {
					namespace = "default"
				} else {
					namespace = currentNs
				}
			} else {
				allNamespaces = true
			}
		}

		if allNamespaces {
			namespace = ""
		}

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

		quotaInfos := make([]data.ResourceQuotaInfo, 0)

		if hasFilters {
			// Multi-cluster: use temp kubeconfig for each cluster
			for _, clusterInfo := range clusterList {
				restConfig, err := GetRestConfigForCluster(clusterInfo)
				if err != nil {
					if verbose {
						log.Printf("Warning: Failed to get config for cluster %s: %v", clusterInfo.ClusterName, err)
					}
					continue
				}

				quotas, err := k8s.GetResourceQuotasWithConfig(context.Background(), restConfig, namespace)
				if err != nil {
					if verbose {
						log.Printf("Warning: Failed to get quotas for cluster %s: %v", clusterInfo.ClusterName, err)
					}
					continue
				}

				for _, quota := range quotas {
					for resourceName, hardLimit := range quota.Status.Hard {
						used := quota.Status.Used[resourceName]
						info := data.ResourceQuotaInfo{
							Profile:      clusterInfo.AWSProfile,
							Region:       clusterInfo.Region,
							ClusterName:  clusterInfo.ClusterName,
							Namespace:    quota.Namespace,
							QuotaName:    quota.Name,
							ResourceName: string(resourceName),
							Hard:         hardLimit.String(),
							Used:         used.String(),
						}
						quotaInfos = append(quotaInfos, info)
					}
				}
			}
		} else {
			clusterInfo := clusterList[0]
			quotas, err := k8s.GetResourceQuotas(context.Background(), namespace)
			if err != nil {
				log.Fatalf("Error getting resource quotas: %v", err)
			}

			for _, quota := range quotas {
				for resourceName, hardLimit := range quota.Status.Hard {
					used := quota.Status.Used[resourceName]
					info := data.ResourceQuotaInfo{
						Profile:      clusterInfo.AWSProfile,
						Region:       clusterInfo.Region,
						ClusterName:  clusterInfo.ClusterName,
						Namespace:    quota.Namespace,
						QuotaName:    quota.Name,
						ResourceName: string(resourceName),
						Hard:         hardLimit.String(),
						Used:         used.String(),
					}
					quotaInfos = append(quotaInfos, info)
				}
			}
		}

		if len(quotaInfos) == 0 {
			if namespace == "" {
				log.Println("No resource quotas found in any namespace")
			} else {
				log.Printf("No resource quotas found in namespace: %s\n", namespace)
			}
			return
		}

		printutils.PrintResourceQuotas(noHeaders, quotaInfos...)

		if hasFilters {
			saveCacheToDisk()
		}
	},
}

func init() {
	quotasCmd.Flags().StringP("namespace", "n", "", "Namespace to show quotas for")
	quotasCmd.Flags().BoolP("all-namespaces", "A", false, "Show quotas across all namespaces")
	quotasCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	quotasCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	quotasCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	quotasCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	quotasCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	quotasCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	quotasCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	quotasCmd.Flags().StringP("version", "v", "", "Filter by EKS version")

	rootCmd.AddCommand(quotasCmd)
}
