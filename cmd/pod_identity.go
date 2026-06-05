package cmd

import (
	"fmt"
	"log"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var podIdentityCmd = &cobra.Command{
	Use:     "pod-identity",
	Aliases: []string{"pi"},
	Short:   "List EKS Pod Identity associations from the AWS EKS API",
	Long: `List EKS Pod Identity associations configured via the AWS EKS API.

This command queries the AWS EKS API to show true EKS Pod Identity
associations. These are different from IRSA (IAM Roles for Service Accounts).

EKS Pod Identity is a newer AWS feature that eliminates the need for OIDC providers.

When cluster filters are provided, queries multiple clusters.
Without filters, queries the current cluster context.`,
	Example: `  # List all Pod Identity associations
  kubectl eks pod-identity

  # List Pod Identity in specific namespace
  kubectl eks pod-identity -n kube-system

  # List Pod Identity across all namespaces
  kubectl eks pod-identity -A

  # List across clusters matching filter
  kubectl eks pod-identity --cluster-contains prod`,
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

		podIdentityInfos := []data.PodIdentityInfo{}
		for _, clusterInfo := range clusterList {
			associations, err := eks.GetPodIdentityAssociations(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
			if err != nil {
				fmt.Fprintf(log.Writer(), "Error getting pod identity associations for cluster %s: %v\n", clusterInfo.ClusterName, err)
				continue
			}

			for _, assoc := range associations {
				if namespace != "" && assoc.Namespace != namespace {
					continue
				}
				podIdentityInfos = append(podIdentityInfos, assoc)
			}
		}

		if len(podIdentityInfos) == 0 {
			if namespace == "" {
				log.Println("No EKS Pod Identity associations found")
			} else {
				log.Printf("No EKS Pod Identity associations found in namespace: %s\n", namespace)
			}
			return
		}

		printutils.PrintPodIdentity(noHeaders, podIdentityInfos...)

		if hasFilters {
			saveCacheToDisk()
		}
	},
}

func init() {
	podIdentityCmd.Flags().StringP("namespace", "n", "", "Namespace to show Pod Identity for")
	podIdentityCmd.Flags().BoolP("all-namespaces", "A", false, "Show Pod Identity across all namespaces (default)")
	podIdentityCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	podIdentityCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	podIdentityCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	podIdentityCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	podIdentityCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	podIdentityCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	podIdentityCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	podIdentityCmd.Flags().StringP("version", "v", "", "Filter by EKS version")

	rootCmd.AddCommand(podIdentityCmd)
}
