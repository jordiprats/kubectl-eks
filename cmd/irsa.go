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

var irsaCmd = &cobra.Command{
	Use:   "irsa",
	Short: "List service accounts with IRSA annotations and their IAM roles",
	Long: `List service accounts with IRSA (IAM Roles for Service Accounts) annotations.

Shows the service account name, namespace, and associated IAM role ARN
from the eks.amazonaws.com/role-arn annotation.

When cluster filters are provided, queries multiple clusters.
Without filters, queries the current cluster context.`,
	Example: `  # List all service accounts with IRSA
  kubectl eks irsa

  # List IRSA in specific namespace
  kubectl eks irsa -n kube-system

  # List IRSA across all namespaces
  kubectl eks irsa -A

  # List across clusters matching filter
  kubectl eks irsa --cluster-contains prod`,
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

		// Default to all namespaces
		if !allNamespaces && namespace == "" {
			allNamespaces = true
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

		irsaInfos := make([]data.IRSAInfo, 0)

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

				serviceAccounts, err := k8s.GetServiceAccountsWithIRSAWithConfig(context.Background(), restConfig, namespace)
				if err != nil {
					if verbose {
						log.Printf("Warning: Failed to get service accounts for cluster %s: %v", clusterInfo.ClusterName, err)
					}
					continue
				}

				for _, sa := range serviceAccounts {
					roleArn := sa.Annotations["eks.amazonaws.com/role-arn"]
					if roleArn != "" {
						info := data.IRSAInfo{
							Profile:            clusterInfo.AWSProfile,
							Region:             clusterInfo.Region,
							ClusterName:        clusterInfo.ClusterName,
							Namespace:          sa.Namespace,
							ServiceAccountName: sa.Name,
							IAMRoleARN:         roleArn,
						}
						irsaInfos = append(irsaInfos, info)
					}
				}
			}
		} else {
			clusterInfo := clusterList[0]
			serviceAccounts, err := k8s.GetServiceAccountsWithIRSA(context.Background(), namespace)
			if err != nil {
				log.Fatalf("Error getting service accounts: %v", err)
			}

			for _, sa := range serviceAccounts {
				roleArn := sa.Annotations["eks.amazonaws.com/role-arn"]
				if roleArn != "" {
					info := data.IRSAInfo{
						Profile:            clusterInfo.AWSProfile,
						Region:             clusterInfo.Region,
						ClusterName:        clusterInfo.ClusterName,
						Namespace:          sa.Namespace,
						ServiceAccountName: sa.Name,
						IAMRoleARN:         roleArn,
					}
					irsaInfos = append(irsaInfos, info)
				}
			}
		}

		if len(irsaInfos) == 0 {
			if namespace == "" {
				log.Println("No service accounts with IRSA annotations found")
			} else {
				log.Printf("No service accounts with IRSA annotations found in namespace: %s\n", namespace)
			}
			return
		}

		printutils.PrintIRSA(noHeaders, irsaInfos...)

		if hasFilters {
			saveCacheToDisk()
		}
	},
}

func init() {
	irsaCmd.Flags().StringP("namespace", "n", "", "Namespace to show IRSA for")
	irsaCmd.Flags().BoolP("all-namespaces", "A", false, "Show IRSA across all namespaces (default)")
	irsaCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	irsaCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	irsaCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	irsaCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	irsaCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	irsaCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	irsaCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	irsaCmd.Flags().StringP("version", "v", "", "Filter by EKS version")

	rootCmd.AddCommand(irsaCmd)
}
