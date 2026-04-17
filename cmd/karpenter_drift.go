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

var karpenterDriftCmd = &cobra.Command{
	Use:   "drift",
	Short: "List drifted Karpenter nodes and NodeClaims",
	Long: `List nodes and NodeClaims currently in drifted state across clusters.

Drift occurs when NodeClaims no longer match their NodePool requirements
due to configuration changes, AMI updates, or other factors.`,
	Example: `  # List drifted resources for current cluster
  kubectl eks karpenter drift

  # List drifted resources across clusters matching filter
  kubectl eks karpenter drift --cluster-contains prod`,
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

		allDriftedResources := []data.KarpenterDriftInfo{}

		for _, clusterInfo := range clusterList {
			err := eks.UpdateKubeConfig(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, "")
			if err != nil {
				log.Printf("Warning: Failed to update kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
				continue
			}

			driftedResources, err := karpenter.GetDriftedResources(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
			if err != nil {
				log.Printf("Warning: Failed to get drifted resources from cluster %s: %v", clusterInfo.ClusterName, err)
				continue
			}

			allDriftedResources = append(allDriftedResources, driftedResources...)
		}

		printutils.PrintKarpenterDrift(noHeaders, allDriftedResources...)

		saveCacheToDisk()
	},
}

func init() {
	karpenterDriftCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	karpenterDriftCmd.Flags().StringP("profile", "p", "", "AWS profile to use")
	karpenterDriftCmd.Flags().StringP("profile-contains", "q", "", "AWS profile contains string")
	karpenterDriftCmd.Flags().StringP("cluster-contains", "c", "", "Cluster name contains string")
	karpenterDriftCmd.Flags().StringP("cluster-not-contains", "x", "", "Cluster name does not contain string")
	karpenterDriftCmd.Flags().StringP("region", "r", "", "AWS region to use")
	karpenterDriftCmd.Flags().StringP("version", "v", "", "Filter by EKS version")

	karpenterCmd.AddCommand(karpenterDriftCmd)
}
