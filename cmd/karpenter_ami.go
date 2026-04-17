package cmd

import (
	"log"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/karpenter"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var karpenterAMICmd = &cobra.Command{
	Use:   "ami",
	Short: "Show AMI usage across Karpenter NodePools",
	Long: `Show current AMIs in use per NodePool across clusters.

This helps identify which AMIs are being used by each NodePool for
inventory and tracking purposes.`,
	Example: `  # Show AMI usage for current cluster
  kubectl eks karpenter ami

  # Show AMI usage across clusters matching filter
  kubectl eks karpenter ami --cluster-contains prod`,
	Run: func(cmd *cobra.Command, args []string) {
		refresh, _ := cmd.Flags().GetBool("refresh")
		profile, _ := cmd.Flags().GetString("profile")
		profileContains, _ := cmd.Flags().GetString("profile-contains")
		nameContains, _ := cmd.Flags().GetString("cluster-contains")
		nameNotContains, _ := cmd.Flags().GetString("cluster-not-contains")
		region, _ := cmd.Flags().GetString("region")
		version, _ := cmd.Flags().GetString("version")
		noHeaders, _ := cmd.Flags().GetBool("no-headers")

		hasFilters := profile != "" || profileContains != "" || nameContains != "" ||
			nameNotContains != "" || region != "" || version != ""

		var clusterList []data.ClusterInfo
		var err error

		if hasFilters {
			loadCacheFromDisk()
			if CachedData == nil {
				CachedData = &data.KubeCtlEksCache{
					ClusterByARN: make(map[string]data.ClusterInfo),
					ClusterList:  make(map[string]map[string][]data.ClusterInfo),
				}
			}
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

		// Save and restore context
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

		allAMIUsage := []data.KarpenterAMIUsageInfo{}

		for _, clusterInfo := range clusterList {
			err := eks.UpdateKubeConfig(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, "")
			if err != nil {
				log.Printf("Warning: Failed to update kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
				continue
			}

			amiUsage, err := karpenter.GetAMIUsage(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, clusterInfo.Version)
			if err != nil {
				log.Printf("Warning: Failed to get AMI usage from cluster %s: %v", clusterInfo.ClusterName, err)
				continue
			}

			allAMIUsage = append(allAMIUsage, amiUsage...)
		}

		printutils.PrintKarpenterAMIUsage(noHeaders, allAMIUsage...)

		saveCacheToDisk()
	},
}

func init() {
	karpenterAMICmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	karpenterAMICmd.Flags().StringP("profile", "p", "", "AWS profile to use")
	karpenterAMICmd.Flags().StringP("profile-contains", "q", "", "AWS profile contains string")
	karpenterAMICmd.Flags().StringP("cluster-contains", "c", "", "Cluster name contains string")
	karpenterAMICmd.Flags().StringP("cluster-not-contains", "x", "", "Cluster name does not contain string")
	karpenterAMICmd.Flags().StringP("region", "r", "", "AWS region to use")
	karpenterAMICmd.Flags().StringP("version", "v", "", "Filter by EKS version")

	karpenterCmd.AddCommand(karpenterAMICmd)
}
