package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Show Kubernetes events across namespaces",
	Long: `Display Kubernetes events across all or specific namespaces, sorted by timestamp.

Events provide insights into cluster activities such as pod scheduling,
image pulls, volume mounts, configuration changes, and errors.

By default shows all event types (Normal and Warning). Use --warnings-only
to filter for just Warning events. Events are sorted with most recent first.

When cluster filters are provided, queries multiple clusters.
Without filters, queries the current cluster context.`,
	Example: `  # Show all events across all namespaces
  kubectl eks events

  # Show only warning events
  kubectl eks events --warnings-only

  # Show events for specific namespace
  kubectl eks events -n kube-system

  # Show events across clusters matching filter
  kubectl eks events --cluster-contains prod`,
	Run: func(cmd *cobra.Command, args []string) {
		noHeaders, _ := cmd.Flags().GetBool("no-headers")
		refresh, _ := cmd.Flags().GetBool("refresh")
		namespace, _ := cmd.Flags().GetString("namespace")
		allNamespaces, _ := cmd.Flags().GetBool("all-namespaces")
		warningsOnly, _ := cmd.Flags().GetBool("warnings-only")
		allEvents, _ := cmd.Flags().GetBool("all")

		// Get filter flags
		profile, _ := cmd.Flags().GetString("profile")
		profileContains, _ := cmd.Flags().GetString("profile-contains")
		nameContains, _ := cmd.Flags().GetString("cluster-contains")
		nameNotContains, _ := cmd.Flags().GetString("cluster-not-contains")
		region, _ := cmd.Flags().GetString("region")
		version, _ := cmd.Flags().GetString("version")

		// Check if any filter is specified
		hasFilters := profile != "" || profileContains != "" || nameContains != "" ||
			nameNotContains != "" || region != "" || version != ""

		// Default to all namespaces unless specific namespace is provided
		if !allNamespaces && namespace == "" {
			allNamespaces = true
		}

		if allNamespaces {
			namespace = ""
		}

		// Show all events by default (both Normal and Warning)
		if !warningsOnly && !allEvents {
			allEvents = true
		}

		// If --warnings-only is explicitly set, only show warnings
		if warningsOnly {
			allEvents = false
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

		if len(clusterList) == 0 {
			fmt.Println("No clusters found matching the specified filters")
			return
		}

		eventInfos := make([]data.EventInfo, 0)

		if hasFilters {
			// Multi-cluster: use temp kubeconfig for each cluster
			for _, clusterInfo := range clusterList {
				restConfig, err := GetRestConfigForCluster(clusterInfo)
				if err != nil {
					log.Printf("Warning: Failed to get config for cluster %s: %v", clusterInfo.ClusterName, err)
					continue
				}

				events, err := k8s.GetEventsWithConfig(context.Background(), restConfig, namespace)
				if err != nil {
					log.Printf("Warning: Failed to get events for cluster %s: %v", clusterInfo.ClusterName, err)
					continue
				}

				for _, event := range events {
					if warningsOnly && !strings.EqualFold(event.Type, "Warning") {
						continue
					}

					lastSeen := event.LastTimestamp.Time
					if lastSeen.IsZero() && !event.EventTime.Time.IsZero() {
						lastSeen = event.EventTime.Time
					}

					info := data.EventInfo{
						Profile:     clusterInfo.AWSProfile,
						Region:      clusterInfo.Region,
						ClusterName: clusterInfo.ClusterName,
						Namespace:   event.Namespace,
						LastSeen:    lastSeen,
						Type:        event.Type,
						Reason:      event.Reason,
						Object:      event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name,
						Message:     event.Message,
						Count:       event.Count,
					}
					eventInfos = append(eventInfos, info)
				}
			}
		} else {
			clusterInfo := clusterList[0]
			events, err := k8s.GetEvents(context.Background(), namespace)
			if err != nil {
				log.Fatalf("Error getting events: %v", err)
			}

			for _, event := range events {
				if warningsOnly && !strings.EqualFold(event.Type, "Warning") {
					continue
				}

				lastSeen := event.LastTimestamp.Time
				if lastSeen.IsZero() && !event.EventTime.Time.IsZero() {
					lastSeen = event.EventTime.Time
				}

				info := data.EventInfo{
					Profile:     clusterInfo.AWSProfile,
					Region:      clusterInfo.Region,
					ClusterName: clusterInfo.ClusterName,
					Namespace:   event.Namespace,
					LastSeen:    lastSeen,
					Type:        event.Type,
					Reason:      event.Reason,
					Object:      event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name,
					Message:     event.Message,
					Count:       event.Count,
				}
				eventInfos = append(eventInfos, info)
			}
		}

		if len(eventInfos) == 0 {
			if warningsOnly {
				log.Println("No warning events found. Try --all to see all event types.")
			} else {
				log.Println("No events match the specified criteria")
			}
			return
		}

		printutils.PrintEvents(noHeaders, eventInfos...)

		if hasFilters {
			saveCacheToDisk()
		}
	},
}

func init() {
	eventsCmd.Flags().StringP("namespace", "n", "", "Namespace to show events for")
	eventsCmd.Flags().BoolP("all-namespaces", "A", false, "Show events across all namespaces (default)")
	eventsCmd.Flags().Bool("warnings-only", false, "Show only warning events")
	eventsCmd.Flags().Bool("all", false, "Show all events (default behavior)")
	eventsCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	eventsCmd.Flags().StringP("profile", "p", "", "Filter by exact AWS profile name (account)")
	eventsCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	eventsCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	eventsCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	eventsCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	eventsCmd.Flags().StringP("version", "v", "", "Filter by EKS version")

	rootCmd.AddCommand(eventsCmd)
}
