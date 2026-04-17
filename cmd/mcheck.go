package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var mCheckCmd = &cobra.Command{
	Use:     "mcheck",
	Aliases: []string{"mready", "mrediness", "health", "mhealth"},
	Short:   "Check health status of resources across multiple clusters",
	Long: `Check the readiness/health status of Kubernetes resources across all clusters that match a filter.

Checks the following resources:
  - Pods: Running or Completed status (excludes Completed from unhealthy)
  - Deployments: Ready replicas match desired replicas
  - StatefulSets: Ready replicas match desired replicas
  - DaemonSets: Ready nodes match desired nodes
  - ReplicaSets: Ready replicas match desired replicas

By default, checks all namespaces and only shows unhealthy resources.
Use -n to check a specific namespace, use --all to show healthy resources too.`,
	Example: `  # Check all resources across clusters (all namespaces, only unhealthy)
  kubectl eks mcheck

  # Check resources in specific namespace only
  kubectl eks mcheck -n kube-system

  # Show all resources including healthy ones
  kubectl eks mcheck --all

  # Filter clusters
  kubectl eks mcheck --cluster-contains prod

  # Check specific resource types
  kubectl eks mcheck --pods --deployments

  # Summary only (no individual resources)
  kubectl eks mcheck --summary`,
	Run: func(cmd *cobra.Command, args []string) {
		refresh, _ := cmd.Flags().GetBool("refresh")
		profile, _ := cmd.Flags().GetString("profile")
		profileContains, _ := cmd.Flags().GetString("profile-contains")
		nameContains, _ := cmd.Flags().GetString("cluster-contains")
		nameNotContains, _ := cmd.Flags().GetString("cluster-not-contains")
		region, _ := cmd.Flags().GetString("region")
		version, _ := cmd.Flags().GetString("version")
		namespace, _ := cmd.Flags().GetString("namespace")
		showAll, _ := cmd.Flags().GetBool("all")
		summaryOnly, _ := cmd.Flags().GetBool("summary")
		noHeaders, _ := cmd.Flags().GetBool("no-headers")

		checkPods, _ := cmd.Flags().GetBool("pods")
		checkDeploys, _ := cmd.Flags().GetBool("deployments")
		checkSts, _ := cmd.Flags().GetBool("statefulsets")
		checkDs, _ := cmd.Flags().GetBool("daemonsets")
		checkRs, _ := cmd.Flags().GetBool("replicasets")

		// If no specific types requested, check all
		checkAllTypes := !checkPods && !checkDeploys && !checkSts && !checkDs && !checkRs
		if checkAllTypes {
			checkPods, checkDeploys, checkSts, checkDs, checkRs = true, true, true, true, true
		}

		clusterList, err := LoadClusterList([]string{}, profile, profileContains, nameContains, nameNotContains, region, version, refresh)
		if err != nil {
			log.Fatalf("Error loading cluster list: %v", err)
		}

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

		allResults := []data.HealthCheckResult{}
		clusterSummaries := []data.ClusterHealthSummary{}

		for _, clusterInfo := range clusterList {
			err := eks.UpdateKubeConfig(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, "")
			if err != nil {
				log.Printf("Warning: Failed to update kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
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

			clientset, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				continue
			}

			namespaces := []string{}
			if namespace != "" {
				// If specific namespace provided, use only that
				namespaces = append(namespaces, namespace)
			} else {
				// Default: check all namespaces
				nsList, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
				if err == nil {
					for _, ns := range nsList.Items {
						namespaces = append(namespaces, ns.Name)
					}
				}
			}

			clusterResults := []data.HealthCheckResult{}

			for _, ns := range namespaces {
				if checkPods {
					results := checkPodsHealth(clientset, clusterInfo, ns)
					clusterResults = append(clusterResults, results...)
				}
				if checkDeploys {
					results := checkDeploymentsHealth(clientset, clusterInfo, ns)
					clusterResults = append(clusterResults, results...)
				}
				if checkSts {
					results := checkStatefulSetsHealth(clientset, clusterInfo, ns)
					clusterResults = append(clusterResults, results...)
				}
				if checkDs {
					results := checkDaemonSetsHealth(clientset, clusterInfo, ns)
					clusterResults = append(clusterResults, results...)
				}
				if checkRs {
					results := checkReplicaSetsHealth(clientset, clusterInfo, ns)
					clusterResults = append(clusterResults, results...)
				}
			}

			summary := summarizeResults(clusterInfo, clusterResults)
			clusterSummaries = append(clusterSummaries, summary)
			allResults = append(allResults, clusterResults...)
		}

		if summaryOnly {
			printutils.PrintHealthSummary(noHeaders, clusterSummaries)
		} else {
			filteredResults := allResults
			if !showAll {
				filteredResults = []data.HealthCheckResult{}
				for _, r := range allResults {
					if !r.IsHealthy {
						filteredResults = append(filteredResults, r)
					}
				}
			}
			printutils.PrintHealthDetails(noHeaders, filteredResults, clusterSummaries)
		}

		saveCacheToDisk()
	},
}

func checkPodsHealth(clientset *kubernetes.Clientset, cluster data.ClusterInfo, namespace string) []data.HealthCheckResult {
	results := []data.HealthCheckResult{}

	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return results
	}

	for _, pod := range pods.Items {
		result := data.HealthCheckResult{
			Profile:     cluster.AWSProfile,
			Region:      cluster.Region,
			ClusterName: cluster.ClusterName,
			Namespace:   pod.Namespace,
			Kind:        "Pod",
			Name:        pod.Name,
		}

		phase := string(pod.Status.Phase)
		ready, total := countReadyContainers(pod)
		result.Ready = fmt.Sprintf("%d/%d", ready, total)
		result.Status = phase

		// Completed/Succeeded pods are healthy
		if phase == string(corev1.PodSucceeded) {
			result.IsHealthy = true
			result.Message = "Completed"
		} else if phase == string(corev1.PodRunning) {
			if ready == total && total > 0 {
				result.IsHealthy = true
				result.Message = "All containers ready"
			} else {
				result.IsHealthy = false
				result.Message = fmt.Sprintf("Containers not ready: %d/%d", ready, total)
			}
		} else if phase == string(corev1.PodPending) {
			result.IsHealthy = false
			result.Message = getPodPendingReason(pod)
		} else if phase == string(corev1.PodFailed) {
			result.IsHealthy = false
			result.Message = getPodFailedReason(pod)
		} else {
			result.IsHealthy = false
			result.Message = fmt.Sprintf("Unknown phase: %s", phase)
		}

		results = append(results, result)
	}

	return results
}

func countReadyContainers(pod corev1.Pod) (int, int) {
	ready := 0
	total := len(pod.Spec.Containers)
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
	}
	return ready, total
}

func getPodPendingReason(pod corev1.Pod) string {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
			return fmt.Sprintf("Unschedulable: %s", cond.Message)
		}
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			return fmt.Sprintf("Waiting: %s", cs.State.Waiting.Reason)
		}
	}
	return "Pending"
}

func getPodFailedReason(pod corev1.Pod) string {
	if pod.Status.Reason != "" {
		return pod.Status.Reason
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
			return cs.State.Terminated.Reason
		}
	}
	return "Failed"
}

func checkDeploymentsHealth(clientset *kubernetes.Clientset, cluster data.ClusterInfo, namespace string) []data.HealthCheckResult {
	results := []data.HealthCheckResult{}

	deploys, err := clientset.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return results
	}

	for _, deploy := range deploys.Items {
		result := data.HealthCheckResult{
			Profile:     cluster.AWSProfile,
			Region:      cluster.Region,
			ClusterName: cluster.ClusterName,
			Namespace:   deploy.Namespace,
			Kind:        "Deployment",
			Name:        deploy.Name,
		}

		desired := int32(1)
		if deploy.Spec.Replicas != nil {
			desired = *deploy.Spec.Replicas
		}
		ready := deploy.Status.ReadyReplicas
		available := deploy.Status.AvailableReplicas
		upToDate := deploy.Status.UpdatedReplicas

		result.Ready = fmt.Sprintf("%d/%d", ready, desired)
		result.Status = fmt.Sprintf("Available:%d UpToDate:%d", available, upToDate)

		if ready == desired && available == desired && upToDate == desired {
			result.IsHealthy = true
			result.Message = "All replicas ready"
		} else {
			result.IsHealthy = false
			result.Message = getDeploymentConditionMessage(deploy)
		}

		results = append(results, result)
	}

	return results
}

func getDeploymentConditionMessage(deploy appsv1.Deployment) string {
	for _, cond := range deploy.Status.Conditions {
		if cond.Type == appsv1.DeploymentAvailable && cond.Status == corev1.ConditionFalse {
			return cond.Message
		}
		if cond.Type == appsv1.DeploymentProgressing && cond.Status == corev1.ConditionFalse {
			return cond.Message
		}
	}
	desired := int32(1)
	if deploy.Spec.Replicas != nil {
		desired = *deploy.Spec.Replicas
	}
	return fmt.Sprintf("Ready %d/%d", deploy.Status.ReadyReplicas, desired)
}

func checkStatefulSetsHealth(clientset *kubernetes.Clientset, cluster data.ClusterInfo, namespace string) []data.HealthCheckResult {
	results := []data.HealthCheckResult{}

	stsList, err := clientset.AppsV1().StatefulSets(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return results
	}

	for _, sts := range stsList.Items {
		result := data.HealthCheckResult{
			Profile:     cluster.AWSProfile,
			Region:      cluster.Region,
			ClusterName: cluster.ClusterName,
			Namespace:   sts.Namespace,
			Kind:        "StatefulSet",
			Name:        sts.Name,
		}

		desired := int32(1)
		if sts.Spec.Replicas != nil {
			desired = *sts.Spec.Replicas
		}
		ready := sts.Status.ReadyReplicas

		result.Ready = fmt.Sprintf("%d/%d", ready, desired)
		result.Status = fmt.Sprintf("CurrentRevision:%s", sts.Status.CurrentRevision)

		if ready == desired {
			result.IsHealthy = true
			result.Message = "All replicas ready"
		} else {
			result.IsHealthy = false
			result.Message = fmt.Sprintf("Ready %d/%d", ready, desired)
		}

		results = append(results, result)
	}

	return results
}

func checkDaemonSetsHealth(clientset *kubernetes.Clientset, cluster data.ClusterInfo, namespace string) []data.HealthCheckResult {
	results := []data.HealthCheckResult{}

	dsList, err := clientset.AppsV1().DaemonSets(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return results
	}

	for _, ds := range dsList.Items {
		result := data.HealthCheckResult{
			Profile:     cluster.AWSProfile,
			Region:      cluster.Region,
			ClusterName: cluster.ClusterName,
			Namespace:   ds.Namespace,
			Kind:        "DaemonSet",
			Name:        ds.Name,
		}

		desired := ds.Status.DesiredNumberScheduled
		ready := ds.Status.NumberReady
		available := ds.Status.NumberAvailable

		result.Ready = fmt.Sprintf("%d/%d", ready, desired)
		result.Status = fmt.Sprintf("Available:%d Unavailable:%d", available, ds.Status.NumberUnavailable)

		if ready == desired && available == desired {
			result.IsHealthy = true
			result.Message = "All nodes ready"
		} else {
			result.IsHealthy = false
			result.Message = fmt.Sprintf("Ready %d/%d, Unavailable %d", ready, desired, ds.Status.NumberUnavailable)
		}

		results = append(results, result)
	}

	return results
}

func checkReplicaSetsHealth(clientset *kubernetes.Clientset, cluster data.ClusterInfo, namespace string) []data.HealthCheckResult {
	results := []data.HealthCheckResult{}

	rsList, err := clientset.AppsV1().ReplicaSets(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return results
	}

	for _, rs := range rsList.Items {
		// Skip ReplicaSets with 0 desired (old revisions from deployments)
		desired := int32(0)
		if rs.Spec.Replicas != nil {
			desired = *rs.Spec.Replicas
		}
		if desired == 0 {
			continue
		}

		result := data.HealthCheckResult{
			Profile:     cluster.AWSProfile,
			Region:      cluster.Region,
			ClusterName: cluster.ClusterName,
			Namespace:   rs.Namespace,
			Kind:        "ReplicaSet",
			Name:        rs.Name,
		}

		ready := rs.Status.ReadyReplicas

		result.Ready = fmt.Sprintf("%d/%d", ready, desired)
		result.Status = fmt.Sprintf("Replicas:%d", rs.Status.Replicas)

		if ready == desired {
			result.IsHealthy = true
			result.Message = "All replicas ready"
		} else {
			result.IsHealthy = false
			result.Message = fmt.Sprintf("Ready %d/%d", ready, desired)
		}

		results = append(results, result)
	}

	return results
}

func summarizeResults(cluster data.ClusterInfo, results []data.HealthCheckResult) data.ClusterHealthSummary {
	summary := data.ClusterHealthSummary{
		Profile:     cluster.AWSProfile,
		Region:      cluster.Region,
		ClusterName: cluster.ClusterName,
	}

	for _, r := range results {
		switch r.Kind {
		case "Pod":
			summary.TotalPods++
			if r.IsHealthy {
				summary.HealthyPods++
			}
		case "Deployment":
			summary.TotalDeployments++
			if r.IsHealthy {
				summary.HealthyDeployments++
			}
		case "StatefulSet":
			summary.TotalStatefulSets++
			if r.IsHealthy {
				summary.HealthyStatefulSets++
			}
		case "DaemonSet":
			summary.TotalDaemonSets++
			if r.IsHealthy {
				summary.HealthyDaemonSets++
			}
		case "ReplicaSet":
			summary.TotalReplicaSets++
			if r.IsHealthy {
				summary.HealthyReplicaSets++
			}
		}
	}

	unhealthy := (summary.TotalPods - summary.HealthyPods) +
		(summary.TotalDeployments - summary.HealthyDeployments) +
		(summary.TotalStatefulSets - summary.HealthyStatefulSets) +
		(summary.TotalDaemonSets - summary.HealthyDaemonSets) +
		(summary.TotalReplicaSets - summary.HealthyReplicaSets)

	if unhealthy == 0 {
		summary.OverallStatus = "Healthy"
	} else {
		summary.OverallStatus = fmt.Sprintf("%d Unhealthy", unhealthy)
	}

	return summary
}

func init() {
	mCheckCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	mCheckCmd.Flags().StringP("profile", "p", "", "AWS profile to use")
	mCheckCmd.Flags().StringP("profile-contains", "q", "", "AWS profile contains string")
	mCheckCmd.Flags().StringP("cluster-contains", "c", "", "Cluster name contains string")
	mCheckCmd.Flags().StringP("cluster-not-contains", "x", "", "Cluster name does not contain string")
	mCheckCmd.Flags().StringP("region", "r", "", "AWS region to use")
	mCheckCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	mCheckCmd.Flags().StringP("namespace", "n", "", "Kubernetes namespace (default: all namespaces)")
	mCheckCmd.Flags().Bool("all", false, "Show all resources including healthy ones")
	mCheckCmd.Flags().Bool("summary", false, "Show health summary")
	mCheckCmd.Flags().Bool("no-headers", false, "Don't print headers")

	// Resource type filters
	mCheckCmd.Flags().Bool("pods", false, "Check only pods")
	mCheckCmd.Flags().Bool("deployments", false, "Check only deployments")
	mCheckCmd.Flags().Bool("statefulsets", false, "Check only statefulsets")
	mCheckCmd.Flags().Bool("daemonsets", false, "Check only daemonsets")
	mCheckCmd.Flags().Bool("replicasets", false, "Check only replicasets")

	rootCmd.AddCommand(mCheckCmd)
}
