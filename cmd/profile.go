package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:     "aws-profile [cluster-name-or-arn]",
	Aliases: []string{"profile"},
	Short:   "Get AWS profile",
	Long: `Get the AWS profile name for the current cluster (or specified cluster ARN).

When no arguments or filters are provided, returns the AWS profile for the
current kubeconfig context.

You can specify a cluster by partial name, ARN, or use filters to narrow down
the cluster list. When multiple clusters match, use --oldest or --newest to
pick one.`,
	Example: `  # Get profile for the current cluster
  kubectl eks aws-profile

  # Get profile for a cluster by partial name
  kubectl eks aws-profile my-cluster

  # Get profile for a cluster by ARN
  kubectl eks aws-profile arn:aws:eks:us-east-1:123456789012:cluster/my-cluster

  # Get profile filtering by cluster name substring
  kubectl eks aws-profile --cluster-contains dev

  # Get profile filtering by region
  kubectl eks aws-profile --region us-east-1

  # Get profile filtering by version and region
  kubectl eks aws-profile --version 1.29 --region eu-west-1

  # Get profile filtering by AWS profile name substring
  kubectl eks aws-profile --profile-contains prod

  # When multiple clusters match, pick the oldest or newest
  kubectl eks aws-profile --cluster-contains dev --oldest
  kubectl eks aws-profile --cluster-contains dev --newest

  # Exclude clusters by name
  kubectl eks aws-profile --cluster-contains dev --cluster-not-contains staging`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := ""
		if len(args) == 1 {
			target = strings.TrimSpace(args[0])
		}

		profileContains, _ := cmd.Flags().GetString("profile-contains")
		profileNotContains, _ := cmd.Flags().GetString("profile-not-contains")
		nameContains, _ := cmd.Flags().GetString("cluster-contains")
		nameNotContains, _ := cmd.Flags().GetString("cluster-not-contains")
		region, _ := cmd.Flags().GetString("region")
		version, _ := cmd.Flags().GetString("version")
		refresh, _ := cmd.Flags().GetBool("refresh")
		oldest, _ := cmd.Flags().GetBool("oldest")
		newest, _ := cmd.Flags().GetBool("newest")

		hasFilters := profileContains != "" || profileNotContains != "" || nameContains != "" || nameNotContains != "" || region != "" || version != "" || refresh || oldest || newest

		// When filters are provided (or no args at all and filters narrow it down),
		// use the same resolution logic as 'use'.
		if hasFilters || target != "" {
			clusterInfo, ambiguousMatches, err := resolveClusterForUse(target, "", profileContains, profileNotContains, nameContains, nameNotContains, region, version, refresh, oldest, newest)
			if err != nil {
				if len(ambiguousMatches) > 1 {
					printAmbiguousSelectionHelp(target, ambiguousMatches)
				} else {
					fmt.Println(err.Error())
				}
				os.Exit(1)
			}
			fmt.Println(clusterInfo.AWSProfile)
			return
		}

		// Default: resolve from current kubeconfig context
		config, err := KubernetesConfigFlags.ToRawKubeConfigLoader().RawConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading kubeconfig: %v\n", err.Error())
			os.Exit(1)
		}

		currentContext := config.CurrentContext
		contextDetails, exists := config.Contexts[currentContext]
		if !exists {
			fmt.Fprintf(os.Stderr, "Context '%s' not found in kubeconfig\n", currentContext)
			os.Exit(1)
		}

		clusterArn := contextDetails.Cluster

		loadCacheFromDisk()
		if CachedData == nil {
			CachedData = &data.KubeCtlEksCache{
				ClusterByARN: make(map[string]data.ClusterInfo),
				ClusterList:  make(map[string]map[string][]data.ClusterInfo),
			}
		}

		clusterInfo, found := CachedData.ClusterByARN[clusterArn]
		if !found || clusterInfo.Arn != clusterArn {
			CachedData = &data.KubeCtlEksCache{
				ClusterByARN: make(map[string]data.ClusterInfo),
				ClusterList:  make(map[string]map[string][]data.ClusterInfo),
			}
			foundClusterInfo := loadClusterByArn(clusterArn)
			if foundClusterInfo == nil {
				fmt.Println("Current cluster is not an EKS cluster")
				os.Exit(1)
			}
			clusterInfo = *foundClusterInfo
		}

		fmt.Println(clusterInfo.AWSProfile)
	},
}

func init() {
	profileCmd.Flags().BoolP("refresh", "u", false, "Refresh data from AWS")
	profileCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	profileCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	profileCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	profileCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	profileCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	profileCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	profileCmd.Flags().Bool("oldest", false, "When multiple clusters match, use the oldest cluster")
	profileCmd.Flags().Bool("newest", false, "When multiple clusters match, use the newest cluster")

	rootCmd.AddCommand(profileCmd)
}
