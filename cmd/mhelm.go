package cmd

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/kubernetes"
)

var mHelmCmd = &cobra.Command{
	Use:   "mhelm [release-name]",
	Short: "List Helm releases from multiple clusters",
	Long: `List Helm releases from all EKS clusters that match a filter.
By default, releases are listed from the default namespace. Use -n to query a
specific namespace or -A to query all namespaces.`,
	Example: `  # List releases in the default namespace
  kubectl eks mhelm

  # List releases in a specific namespace
  kubectl eks mhelm -n kube-system

  # List releases in all namespaces
  kubectl eks mhelm -A

  # Find a release by exact name across production clusters
  kubectl eks mhelm ingress-nginx -A --cluster-contains prod`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
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

		namespace = helmNamespace(namespace, allNamespaces)

		var releaseName string
		if len(args) == 1 {
			releaseName = args[0]
		}

		clusterList, err := LoadClusterList([]string{}, profile, profileContains, profileNotContains, nameContains, nameNotContains, region, version, refresh)
		if err != nil {
			log.Fatalf("Error loading cluster list: %v", err)
		}

		results := runHelmListing(context.Background(), clusterList, namespace, releaseName)
		printutils.PrintHelmReleases(noHeaders, results)
		saveCacheToDisk()
	},
}

func helmNamespace(namespace string, allNamespaces bool) string {
	if allNamespaces {
		return ""
	}
	if namespace == "" {
		return "default"
	}
	return namespace
}

func runHelmListing(ctx context.Context, clusterList []data.ClusterInfo, namespace, releaseName string) []data.HelmReleaseResult {
	results := []data.HelmReleaseResult{}

	for _, clusterInfo := range clusterList {
		restConfig, err := GetRestConfigForCluster(clusterInfo)
		if err != nil {
			if verbose {
				log.Printf("Warning: Failed to get kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
			}
			continue
		}

		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			continue
		}

		helmStorage := storage.Init(driver.NewSecrets(clientset.CoreV1().Secrets(namespace)))
		releases, err := helmStorage.ListReleases()
		if err != nil {
			results = append(results, data.HelmReleaseResult{
				Profile:     clusterInfo.AWSProfile,
				Region:      clusterInfo.Region,
				ClusterName: clusterInfo.ClusterName,
				Namespace:   namespace,
				Error:       err.Error(),
			})
			continue
		}

		select {
		case <-ctx.Done():
			return results
		default:
		}

		for _, helmRelease := range latestHelmReleases(releases) {
			if releaseName != "" && helmRelease.Name != releaseName {
				continue
			}
			results = append(results, helmReleaseResult(clusterInfo, helmRelease))
		}
	}

	return results
}

func latestHelmReleases(releases []*release.Release) []*release.Release {
	latest := make(map[string]*release.Release)
	for _, helmRelease := range releases {
		if helmRelease == nil {
			continue
		}
		key := helmRelease.Namespace + "\x00" + helmRelease.Name
		if current, ok := latest[key]; !ok || helmRelease.Version > current.Version {
			latest[key] = helmRelease
		}
	}

	result := make([]*release.Release, 0, len(latest))
	for _, helmRelease := range latest {
		result = append(result, helmRelease)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Namespace != result[j].Namespace {
			return result[i].Namespace < result[j].Namespace
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func helmReleaseResult(clusterInfo data.ClusterInfo, helmRelease *release.Release) data.HelmReleaseResult {
	result := data.HelmReleaseResult{
		Profile:     clusterInfo.AWSProfile,
		Region:      clusterInfo.Region,
		ClusterName: clusterInfo.ClusterName,
		Namespace:   helmRelease.Namespace,
		Name:        helmRelease.Name,
		Revision:    helmRelease.Version,
		Updated:     "-",
		Status:      "unknown",
		Chart:       "-",
		AppVersion:  "-",
	}

	if helmRelease.Info != nil {
		result.Status = helmRelease.Info.Status.String()
		if !helmRelease.Info.LastDeployed.IsZero() {
			result.Updated = helmRelease.Info.LastDeployed.Format("2006-01-02 15:04:05 MST")
		}
	}
	if helmRelease.Chart != nil && helmRelease.Chart.Metadata != nil {
		metadata := helmRelease.Chart.Metadata
		result.Chart = fmt.Sprintf("%s-%s", metadata.Name, metadata.Version)
		result.AppVersion = metadata.AppVersion
		if metadata.Name == "" && metadata.Version == "" {
			result.Chart = "-"
		}
		if result.AppVersion == "" {
			result.AppVersion = "-"
		}
	}

	return result
}

func init() {
	mHelmCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	mHelmCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	mHelmCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	mHelmCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	mHelmCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	mHelmCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	mHelmCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	mHelmCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	mHelmCmd.Flags().StringP("namespace", "n", "", "Kubernetes namespace")
	mHelmCmd.Flags().BoolP("all-namespaces", "A", false, "Query all Kubernetes namespaces")
	mHelmCmd.Flags().Bool("no-headers", false, "Don't print headers")

	rootCmd.AddCommand(mHelmCmd)
}
