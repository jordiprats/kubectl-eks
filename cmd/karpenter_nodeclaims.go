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

var karpenterNodeClaimsCmd = &cobra.Command{
	Use:     "nodeclaims",
	Aliases: []string{"nc", "nodeclaim"},
	Short:   "List Karpenter NodeClaims across clusters",
	Long: `List active Karpenter NodeClaims across all clusters that match a filter.

Shows provisioning status, instance type, AMI, capacity type, zone,
and associated NodePool for each NodeClaim.`,
	Example: `  # List NodeClaims for current cluster
  kubectl eks karpenter nodeclaims

  # List NodeClaims across clusters matching filter
  kubectl eks karpenter nodeclaims --cluster-contains prod

  # List NodeClaims with wide output
  kubectl eks karpenter nodeclaims -o wide`,
	Run: func(cmd *cobra.Command, args []string) {
		refresh, _ := cmd.Flags().GetBool("refresh")
		profile, _ := cmd.Flags().GetString("profile")
		profileContains, _ := cmd.Flags().GetString("profile-contains")
		nameContains, _ := cmd.Flags().GetString("cluster-contains")
		nameNotContains, _ := cmd.Flags().GetString("cluster-not-contains")
		region, _ := cmd.Flags().GetString("region")
		version, _ := cmd.Flags().GetString("version")
		noHeaders, _ := cmd.Flags().GetBool("no-headers")
		output, _ := cmd.Flags().GetString("output")

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

		allNodeClaims := []data.KarpenterNodeClaimInfo{}

		for _, clusterInfo := range clusterList {
			err := eks.UpdateKubeConfig(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, "")
			if err != nil {
				log.Printf("Warning: Failed to update kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
				continue
			}

			nodeClaims, err := karpenter.GetNodeClaims(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
			if err != nil {
				log.Printf("Warning: Failed to get NodeClaims from cluster %s: %v", clusterInfo.ClusterName, err)
				continue
			}

			allNodeClaims = append(allNodeClaims, nodeClaims...)
		}

		printutils.PrintKarpenterNodeClaims(noHeaders, output == "wide", allNodeClaims...)

		saveCacheToDisk()
	},
}

func init() {
	karpenterNodeClaimsCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	karpenterNodeClaimsCmd.Flags().StringP("profile", "p", "", "AWS profile to use")
	karpenterNodeClaimsCmd.Flags().StringP("profile-contains", "q", "", "AWS profile contains string")
	karpenterNodeClaimsCmd.Flags().StringP("cluster-contains", "c", "", "Cluster name contains string")
	karpenterNodeClaimsCmd.Flags().StringP("cluster-not-contains", "x", "", "Cluster name does not contain string")
	karpenterNodeClaimsCmd.Flags().StringP("region", "r", "", "AWS region to use")
	karpenterNodeClaimsCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	karpenterNodeClaimsCmd.Flags().StringP("output", "o", "", "Output format: wide")

	karpenterCmd.AddCommand(karpenterNodeClaimsCmd)
}
