package cmd

import (
	"context"
	"log"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var kube2iamCmd = &cobra.Command{
	Use:     "kube2iam",
	Aliases: []string{"k2iam", "k2i"},
	Short:   "List pods with kube2iam annotations and their IAM roles (multi-cluster)",
	Long: `List pods with kube2iam annotations and their associated IAM role ARNs.

Shows the pod name, namespace, IAM role from the iam.amazonaws.com/role annotation,
and the node the pod is running on.

When cluster filters are provided, queries multiple clusters.
Without filters, queries the current cluster context.`,
	Example: `  # List all kube2iam pods in current cluster
  kubectl eks kube2iam

  # List kube2iam pods in specific namespace
  kubectl eks kube2iam -n production

  # List kube2iam pods across all namespaces
  kubectl eks kube2iam -A

  # List across clusters matching filter
  kubectl eks kube2iam --cluster-contains prod

  # Filter by AWS profile
  kubectl eks kube2iam -p my-aws-profile`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get flags
		refresh, _ := cmd.Flags().GetBool("refresh")
		profile, _ := cmd.Flags().GetString("profile")
		profileContains, _ := cmd.Flags().GetString("profile-contains")
		profileNotContains, _ := cmd.Flags().GetString("profile-not-contains")
		nameContains, _ := cmd.Flags().GetString("cluster-contains")
		nameNotContains, _ := cmd.Flags().GetString("cluster-not-contains")
		region, _ := cmd.Flags().GetString("region")
		version, _ := cmd.Flags().GetString("version")
		namespace, _ := cmd.Flags().GetString("namespace")
		allNamespaces, _ := cmd.Flags().GetBool("all-namespaces")
		noHeaders, _ := cmd.Flags().GetBool("no-headers")

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
			log.Println("No clusters found matching the specified filters")
			return
		}

		kube2iamInfos := make([]data.Kube2IAMInfo, 0)

		if hasFilters {
			for _, clusterInfo := range clusterList {
				restConfig, err := GetRestConfigForCluster(clusterInfo)
				if err != nil {
					if verbose {
						log.Printf("Warning: Failed to get kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
					}
					continue
				}

				pods, err := k8s.GetPodsWithKube2IAMWithConfig(context.Background(), restConfig, namespace)
				if err != nil {
					if verbose {
						log.Printf("Warning: Failed to get pods from cluster %s: %v", clusterInfo.ClusterName, err)
					}
					continue
				}

				for _, pod := range pods {
					roleArn := pod.Annotations["iam.amazonaws.com/role"]
					if roleArn != "" {
						kube2iamInfos = append(kube2iamInfos, data.Kube2IAMInfo{
							Profile:     clusterInfo.AWSProfile,
							Region:      clusterInfo.Region,
							ClusterName: clusterInfo.ClusterName,
							Namespace:   pod.Namespace,
							PodName:     pod.Name,
							IAMRole:     roleArn,
							NodeName:    pod.Spec.NodeName,
						})
					}
				}
			}
		} else {
			clusterInfo := clusterList[0]
			pods, err := k8s.GetPodsWithKube2IAM(context.Background(), namespace)
			if err != nil {
				log.Fatalf("Error getting pods: %v", err)
			}

			for _, pod := range pods {
				roleArn := pod.Annotations["iam.amazonaws.com/role"]
				if roleArn != "" {
					kube2iamInfos = append(kube2iamInfos, data.Kube2IAMInfo{
						Profile:     clusterInfo.AWSProfile,
						Region:      clusterInfo.Region,
						ClusterName: clusterInfo.ClusterName,
						Namespace:   pod.Namespace,
						PodName:     pod.Name,
						IAMRole:     roleArn,
						NodeName:    pod.Spec.NodeName,
					})
				}
			}
		}

		if len(kube2iamInfos) == 0 {
			log.Println("No pods with kube2iam annotations found")
			return
		}

		printutils.PrintKube2IAM(noHeaders, kube2iamInfos...)

		if hasFilters {
			saveCacheToDisk()
		}
	},
}

func init() {
	kube2iamCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	kube2iamCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	kube2iamCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	kube2iamCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	kube2iamCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	kube2iamCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	kube2iamCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	kube2iamCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	kube2iamCmd.Flags().StringP("namespace", "n", "", "Namespace to show kube2iam for")
	kube2iamCmd.Flags().BoolP("all-namespaces", "A", false, "Show kube2iam across all namespaces (default)")
	kube2iamCmd.Flags().Bool("no-headers", false, "Don't print headers")

	rootCmd.AddCommand(kube2iamCmd)
}
