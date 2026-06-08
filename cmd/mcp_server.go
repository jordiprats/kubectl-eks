package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/awsconfig"
	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/karpenter"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var mcpServerCmd = &cobra.Command{
	Use:   "mcp-server",
	Short: "Start an MCP (Model Context Protocol) server over stdio",
	Long: `Start a Model Context Protocol (MCP) server that exposes kubectl-eks
functionality as tools for AI assistants.

The server communicates over stdin/stdout using the MCP JSON-RPC protocol.
Configure your AI client (Claude Desktop, VS Code Copilot, Cursor, etc.)
to launch this command as an MCP server.`,
	Example: `  # Start MCP server (used by AI clients, not typically run manually)
  kubectl eks mcp-server

  # Claude Desktop configuration (~/.claude/claude_desktop_config.json):
  # {
  #   "mcpServers": {
  #     "kubectl-eks": {
  #       "command": "kubectl-eks",
  #       "args": ["mcp-server"]
  #     }
  #   }
  # }`,
	Run: func(cmd *cobra.Command, args []string) {
		runMCPServer()
	},
}

func init() {
	rootCmd.AddCommand(mcpServerCmd)
}

// captureOutput redirects stdout to capture printed output from printutils functions.
func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Also capture log output that goes to stderr (some commands use log.Printf)
	oldLog := log.Writer()
	log.SetOutput(w)

	fn()

	w.Close()
	os.Stdout = old
	log.SetOutput(oldLog)

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// suppressStderr discards stderr during fn to keep MCP stdio clean.
func suppressStderr(fn func()) {
	old := os.Stderr
	devnull, err := os.Open(os.DevNull)
	if err == nil {
		os.Stderr = devnull
		defer func() {
			os.Stderr = old
			devnull.Close()
		}()
	}
	fn()
}

// clusterFilterArgs holds the common filter parameters used across multi-cluster tools.
type clusterFilterArgs struct {
	Profile            string
	ProfileContains    string
	ProfileNotContains string
	ClusterContains    string
	ClusterNotContains string
	Region             string
	Version            string
}

func parseClusterFilters(request mcp.CallToolRequest) clusterFilterArgs {
	return clusterFilterArgs{
		Profile:            request.GetString("profile", ""),
		ProfileContains:    request.GetString("profile_contains", ""),
		ProfileNotContains: request.GetString("profile_not_contains", ""),
		ClusterContains:    request.GetString("cluster_contains", ""),
		ClusterNotContains: request.GetString("cluster_not_contains", ""),
		Region:             request.GetString("region", ""),
		Version:            request.GetString("version", ""),
	}
}

func (f clusterFilterArgs) hasFilters() bool {
	return f.Profile != "" || f.ProfileContains != "" || f.ProfileNotContains != "" || f.ClusterContains != "" ||
		f.ClusterNotContains != "" || f.Region != "" || f.Version != ""
}

func (f clusterFilterArgs) loadClusters() ([]data.ClusterInfo, error) {
	if !f.hasFilters() {
		clusterInfo, err := GetCurrentClusterInfo()
		if err != nil {
			return nil, err
		}
		return []data.ClusterInfo{clusterInfo}, nil
	}

	loadCacheFromDisk()
	if CachedData == nil {
		CachedData = &data.KubeCtlEksCache{
			ClusterByARN: make(map[string]data.ClusterInfo),
			ClusterList:  make(map[string]map[string][]data.ClusterInfo),
		}
	}
	return LoadClusterList([]string{}, f.Profile, f.ProfileContains, f.ProfileNotContains, f.ClusterContains, f.ClusterNotContains, f.Region, f.Version)
}

func addClusterFilterProps(tool mcp.Tool) mcp.Tool {
	return mcp.NewTool(
		tool.Name,
		mcp.WithDescription(tool.Description),
		mcp.WithString("profile", mcp.Description("Filter by exact AWS profile name")),
		mcp.WithString("profile_contains", mcp.Description("Filter by AWS profile name substring")),
		mcp.WithString("profile_not_contains", mcp.Description("Exclude profiles whose name contains this substring")),
		mcp.WithString("cluster_contains", mcp.Description("Filter by cluster name substring")),
		mcp.WithString("cluster_not_contains", mcp.Description("Exclude clusters whose name contains this substring")),
		mcp.WithString("region", mcp.Description("Filter by AWS region")),
		mcp.WithString("version", mcp.Description("Filter by EKS Kubernetes version")),
	)
}

func runMCPServer() {
	// Redirect log output to stderr to keep stdout clean for MCP protocol
	log.SetOutput(os.Stderr)

	s := server.NewMCPServer(
		"kubectl-eks",
		"1.0.0",
	)

	// --- list_clusters ---
	s.AddTool(
		addClusterFilterProps(mcp.NewTool("list_clusters",
			mcp.WithDescription("List EKS clusters across AWS accounts and regions. Returns cluster name, region, profile, version, status, and ARN. Without filters, lists all discoverable clusters."),
		)),
		handleListClusters,
	)

	// --- get_current_cluster ---
	s.AddTool(
		mcp.NewTool("get_current_cluster",
			mcp.WithDescription("Show the current EKS cluster context including name, region, AWS profile, version, status, and namespace."),
		),
		handleGetCurrentCluster,
	)

	// --- use_cluster ---
	s.AddTool(
		mcp.NewTool("use_cluster",
			mcp.WithDescription("Switch kubectl context to a different EKS cluster. Accepts a cluster name, substring, or full ARN."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name, name substring, or full ARN to switch to")),
			mcp.WithString("namespace", mcp.Description("Namespace to set after switching")),
			mcp.WithString("profile", mcp.Description("AWS profile to use for the cluster")),
			mcp.WithString("profile_contains", mcp.Description("Filter by AWS profile name substring")),
			mcp.WithString("region", mcp.Description("Filter by AWS region")),
		),
		handleUseCluster,
	)

	// --- get_nodes ---
	s.AddTool(
		addClusterFilterProps(mcp.NewTool("get_nodes",
			mcp.WithDescription("List Kubernetes nodes with instance type, status, CPU/memory capacity and usage, and pressure conditions. Without filters, queries the current cluster."),
			mcp.WithString("managed_by", mcp.Description("Filter nodes by managed-by substring (e.g. 'karpenter', 'nodegroup', 'fargate'). Case-insensitive.")),
		)),
		handleGetNodes,
	)

	// --- get_nodegroups ---
	s.AddTool(
		mcp.NewTool("get_nodegroups",
			mcp.WithDescription("List EKS managed node groups with instance types, scaling config (min/max/desired), AMI type, and capacity type for the current cluster."),
		),
		handleGetNodegroups,
	)

	// --- get_events ---
	s.AddTool(
		mcp.NewTool("get_events",
			mcp.WithDescription("Show Kubernetes events for the current cluster. Events include scheduling, image pulls, errors, and warnings."),
			mcp.WithString("namespace", mcp.Description("Namespace to show events for (default: all namespaces)")),
			mcp.WithBoolean("warnings_only", mcp.Description("Show only Warning events")),
		),
		handleGetEvents,
	)

	// --- get_insights ---
	s.AddTool(
		mcp.NewTool("get_insights",
			mcp.WithDescription("Show EKS cluster insights and recommendations including deprecated API usage, security findings, and upgrade blockers for the current cluster."),
			mcp.WithString("show_id", mcp.Description("Show detailed info for a specific insight ID")),
		),
		handleGetInsights,
	)

	// --- whoami ---
	s.AddTool(
		mcp.NewTool("whoami",
			mcp.WithDescription("Show current AWS IAM identity and Kubernetes user/group mapping for the current cluster."),
		),
		handleWhoAmI,
	)

	// --- get_stats ---
	s.AddTool(
		addClusterFilterProps(mcp.NewTool("get_stats",
			mcp.WithDescription("Show aggregated cluster statistics: node counts, pod counts, namespace counts, and health indicators. Without filters, queries the current cluster."),
		)),
		handleGetStats,
	)

	// --- get_karpenter_nodepools ---
	s.AddTool(
		addClusterFilterProps(mcp.NewTool("get_karpenter_nodepools",
			mcp.WithDescription("List Karpenter NodePools with instance type constraints, resource limits, disruption settings, and NodeClass info. Without filters, queries the current cluster."),
		)),
		handleGetKarpenterNodePools,
	)

	// --- get_karpenter_nodeclaims ---
	s.AddTool(
		addClusterFilterProps(mcp.NewTool("get_karpenter_nodeclaims",
			mcp.WithDescription("List active Karpenter NodeClaims with provisioning status, instance type, AMI, capacity type, zone, and associated NodePool. Without filters, queries the current cluster."),
		)),
		handleGetKarpenterNodeClaims,
	)

	// --- get_karpenter_drift ---
	s.AddTool(
		addClusterFilterProps(mcp.NewTool("get_karpenter_drift",
			mcp.WithDescription("List drifted Karpenter nodes and NodeClaims. Drift occurs when NodeClaims no longer match their NodePool requirements. Without filters, queries the current cluster."),
		)),
		handleGetKarpenterDrift,
	)

	// --- get_pod_identity ---
	s.AddTool(
		mcp.NewTool("get_pod_identity",
			mcp.WithDescription("List EKS Pod Identity associations (AWS EKS API) for the current cluster showing service account to IAM role mappings."),
			mcp.WithString("namespace", mcp.Description("Namespace to filter (default: all namespaces)")),
		),
		handleGetPodIdentity,
	)

	// --- get_irsa ---
	s.AddTool(
		mcp.NewTool("get_irsa",
			mcp.WithDescription("List service accounts with IRSA (IAM Roles for Service Accounts) annotations and their IAM role ARNs for the current cluster."),
			mcp.WithString("namespace", mcp.Description("Namespace to filter (default: all namespaces)")),
		),
		handleGetIRSA,
	)

	// --- get_updates ---
	s.AddTool(
		mcp.NewTool("get_updates",
			mcp.WithDescription("Check for available Kubernetes version and EKS add-on updates for the current cluster."),
		),
		handleGetUpdates,
	)

	// --- get_fargate_profiles ---
	s.AddTool(
		mcp.NewTool("get_fargate_profiles",
			mcp.WithDescription("List EKS Fargate profiles with namespace selectors and configuration for the current cluster."),
		),
		handleGetFargateProfiles,
	)

	// --- get_quotas ---
	s.AddTool(
		mcp.NewTool("get_quotas",
			mcp.WithDescription("Show ResourceQuota configurations and current usage (CPU, memory, pod counts, etc.) for the current cluster."),
			mcp.WithString("namespace", mcp.Description("Namespace to show quotas for (default: current namespace)")),
			mcp.WithBoolean("all_namespaces", mcp.Description("Show quotas across all namespaces")),
		),
		handleGetQuotas,
	)

	// --- get_resources ---
	s.AddTool(
		addClusterFilterProps(mcp.NewTool("get_resources",
			mcp.WithDescription("Get Kubernetes resources (pods, deployments, statefulsets, daemonsets, services, configmaps, CRDs, etc.) across clusters. Similar to 'kubectl get' but works across multiple EKS clusters. Supports any resource type including CRDs."),
			mcp.WithString("resource_type", mcp.Required(), mcp.Description("Resource type to query (e.g., pods, deployments, statefulsets, services, configmaps, nodes, ingresses, or any CRD)")),
			mcp.WithString("resource_name", mcp.Description("Specific resource name to get (optional, lists all if empty)")),
			mcp.WithString("namespace", mcp.Description("Namespace to query (default: 'default')")),
			mcp.WithBoolean("all_namespaces", mcp.Description("Query all namespaces")),
			mcp.WithString("resource_contains", mcp.Description("Filter resources whose name contains this substring")),
			mcp.WithString("filter", mcp.Description("Filter by field value using dot-notation (e.g., status.phase=Running)")),
			mcp.WithString("output", mcp.Description("Output format: wide, json, yaml")),
		)),
		handleGetResources,
	)

	// Start serving over stdio
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

// --- Tool Handlers ---

func handleListClusters(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filters := parseClusterFilters(request)

	var clusterList []data.ClusterInfo
	var errMsg string

	suppressStderr(func() {
		loadCacheFromDisk()
		if CachedData == nil {
			CachedData = &data.KubeCtlEksCache{
				ClusterByARN: make(map[string]data.ClusterInfo),
				ClusterList:  make(map[string]map[string][]data.ClusterInfo),
			}
		}

		// List all clusters across all profiles/regions (same logic as listCmd),
		// then apply filters. LoadClusterList with all-empty filters only returns
		// the current cluster, so we replicate the list command's iteration.
		awsProfiles := awsconfig.GetAWSProfilesWithEKSHints()
		for _, profileDetails := range awsProfiles {
			if filters.Profile != "" && filters.Profile != profileDetails.Name {
				continue
			}
			if filters.ProfileContains != "" && !strings.Contains(profileDetails.Name, filters.ProfileContains) {
				continue
			}
			for _, hintRegion := range profileDetails.HintEKSRegions {
				if filters.Region != "" && filters.Region != hintRegion {
					continue
				}

				cachedRegions, exists := CachedData.ClusterList[profileDetails.Name]
				if !exists {
					loadClusters(profileDetails.Name, hintRegion)
				} else {
					if _, exists := cachedRegions[hintRegion]; !exists {
						loadClusters(profileDetails.Name, hintRegion)
					}
				}

				currentClusterList, exists := CachedData.ClusterList[profileDetails.Name][hintRegion]
				if !exists {
					continue
				}

				for _, cluster := range currentClusterList {
					shouldAdd := true
					if filters.Version != "" && cluster.Version != filters.Version {
						shouldAdd = false
					}
					if filters.ClusterContains != "" && !strings.Contains(cluster.ClusterName, filters.ClusterContains) {
						shouldAdd = false
					}
					if filters.ClusterNotContains != "" && strings.Contains(cluster.ClusterName, filters.ClusterNotContains) {
						shouldAdd = false
					}
					if shouldAdd {
						clusterList = append(clusterList, cluster)
					}
				}
			}
		}
		saveCacheToDisk()
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	if len(clusterList) == 0 {
		return mcp.NewToolResultText("No clusters found matching the specified filters."), nil
	}

	result := clustersToJSON(clusterList)
	return mcp.NewToolResultText(result), nil
}

func handleGetCurrentCluster(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var clusterInfo data.ClusterInfo
	var err error

	suppressStderr(func() {
		clusterInfo, err = GetCurrentClusterInfo()
		if err == nil {
			ns, nsErr := k8s.GetCurrentNamespace()
			if nsErr == nil {
				clusterInfo.Namespace = ns
			} else {
				clusterInfo.Namespace = "default"
			}
		}
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error getting current cluster: %v", err)), nil
	}

	result, _ := json.MarshalIndent(clusterInfo, "", "  ")
	return mcp.NewToolResultText(string(result)), nil
}

func handleUseCluster(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cluster := request.GetString("cluster", "")
	if cluster == "" {
		return mcp.NewToolResultError("'cluster' parameter is required"), nil
	}

	namespace := request.GetString("namespace", "")
	profile := request.GetString("profile", "")
	profileContains := request.GetString("profile_contains", "")
	region := request.GetString("region", "")

	var resultMsg string
	var errMsg string

	suppressStderr(func() {
		// Initialize cache
		loadCacheFromDisk()
		if CachedData == nil {
			CachedData = &data.KubeCtlEksCache{
				ClusterByARN: make(map[string]data.ClusterInfo),
				ClusterList:  make(map[string]map[string][]data.ClusterInfo),
			}
		}

		resolved, ambiguous, err := resolveClusterForUse(cluster, profile, profileContains, "", "", "", region, "", false, false, false)
		if err != nil {
			if ambiguous != nil {
				names := make([]string, len(ambiguous))
				for i, c := range ambiguous {
					names[i] = fmt.Sprintf("%s (profile=%s, region=%s)", c.ClusterName, c.AWSProfile, c.Region)
				}
				errMsg = fmt.Sprintf("Multiple clusters matched %q:\n%s\nPlease be more specific.", cluster, strings.Join(names, "\n"))
			} else {
				errMsg = fmt.Sprintf("Error resolving cluster: %v", err)
			}
			return
		}

		if profile != "" {
			resolved.AWSProfile = profile
		}

		err = eks.UpdateKubeConfig(resolved.AWSProfile, resolved.Region, resolved.ClusterName, "")
		if err != nil {
			errMsg = fmt.Sprintf("Failed to update kubeconfig: %v", err)
			return
		}

		if namespace != "" {
			err = k8s.SetNamespace(namespace)
			if err != nil {
				errMsg = fmt.Sprintf("Switched cluster but failed to set namespace: %v", err)
				return
			}
			resultMsg = fmt.Sprintf("Switched to EKS cluster %q (namespace: %q) in region %q using profile %q",
				resolved.ClusterName, namespace, resolved.Region, resolved.AWSProfile)
		} else {
			resultMsg = fmt.Sprintf("Switched to EKS cluster %q in region %q using profile %q",
				resolved.ClusterName, resolved.Region, resolved.AWSProfile)
		}
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(resultMsg), nil
}

func handleGetNodes(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filters := parseClusterFilters(request)
	managedByContains := request.GetString("managed_by", "")

	var output string
	var errMsg string

	suppressStderr(func() {
		clusterList, err := filters.loadClusters()
		if err != nil {
			errMsg = fmt.Sprintf("Error loading clusters: %v", err)
			return
		}

		allNodes := []data.ClusterNodeInfo{}

		for _, clusterInfo := range clusterList {
			restConfig, err := GetRestConfigForCluster(clusterInfo)
			if err != nil {
				continue
			}

			nodeList, err := k8s.GetNodesWithConfig(restConfig)
			if err != nil {
				continue
			}

			for _, node := range nodeList {
				allNodes = append(allNodes, data.ClusterNodeInfo{
					Profile:     clusterInfo.AWSProfile,
					Region:      clusterInfo.Region,
					ClusterName: clusterInfo.ClusterName,
					Node:        node,
				})
			}
		}

		if managedByContains != "" {
			filtered := allNodes[:0]
			for _, n := range allNodes {
				if strings.Contains(strings.ToLower(n.Node.ManagedBy), strings.ToLower(managedByContains)) {
					filtered = append(filtered, n)
				}
			}
			allNodes = filtered
		}

		if len(allNodes) == 0 {
			output = "No nodes found."
			return
		}

		output = captureOutput(func() {
			printutils.PrintMultiClusterNodes(false, true, allNodes)
		})
		saveCacheToDisk()
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetNodegroups(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var output string
	var errMsg string

	suppressStderr(func() {
		clusterInfo, err := GetCurrentClusterInfo()
		if err != nil {
			errMsg = fmt.Sprintf("Error getting current cluster: %v", err)
			return
		}

		ngList, err := eks.GetEKSNodeGroups(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
		if err != nil {
			errMsg = fmt.Sprintf("Error listing nodegroups: %v", err)
			return
		}

		if len(ngList) == 0 {
			output = "No managed node groups found."
			return
		}

		output = captureOutput(func() {
			printutils.PrintNodeGroup(false, ngList...)
		})
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetEvents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	namespace := request.GetString("namespace", "")
	warningsOnly := request.GetBool("warnings_only", false)

	var output string
	var errMsg string

	suppressStderr(func() {
		clusterInfo, err := GetCurrentClusterInfo()
		if err != nil {
			errMsg = fmt.Sprintf("Error getting current cluster: %v", err)
			return
		}

		events, err := k8s.GetEvents(context.Background(), namespace)
		if err != nil {
			errMsg = fmt.Sprintf("Error getting events: %v", err)
			return
		}

		eventInfos := make([]data.EventInfo, 0)
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

		if len(eventInfos) == 0 {
			output = "No events found."
			return
		}

		output = captureOutput(func() {
			printutils.PrintEvents(false, eventInfos...)
		})
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetInsights(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	showID := request.GetString("show_id", "")

	var output string
	var errMsg string

	suppressStderr(func() {
		clusterInfo, err := GetCurrentClusterInfo()
		if err != nil {
			errMsg = fmt.Sprintf("Error getting current cluster: %v", err)
			return
		}

		if showID == "" {
			insightsList, err := eks.GetEKSInsights(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
			if err != nil {
				errMsg = fmt.Sprintf("Error getting insights: %v", err)
				return
			}

			if len(insightsList) == 0 {
				output = "No insights found."
				return
			}

			output = captureOutput(func() {
				printutils.PrintInsights(false, insightsList...)
			})
		} else {
			insightItem, err := eks.DescribeEKSInsight(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, showID)
			if err != nil {
				errMsg = fmt.Sprintf("Error getting insight detail: %v", err)
				return
			}

			result, _ := json.MarshalIndent(insightItem, "", "  ")
			output = string(result)
		}
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleWhoAmI(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var output string
	var errMsg string

	suppressStderr(func() {
		clusterInfo, err := GetCurrentClusterInfo()
		if err != nil {
			errMsg = fmt.Sprintf("Error getting current cluster: %v", err)
			return
		}

		output = captureOutput(func() {
			// Re-implement whoami logic to capture output
			printutils.PrintWhoAmI(
				false,
				clusterInfo.AWSProfile,
				clusterInfo.Region,
				clusterInfo.ClusterName,
				"", "", "", // AWS identity fields - skip STS call in table, provide JSON instead
				"", "", nil,
			)
		})

		// Since the full whoami needs STS + K8s calls, provide a JSON summary
		ns, _ := k8s.GetCurrentNamespace()
		info := map[string]interface{}{
			"cluster":    clusterInfo.ClusterName,
			"region":     clusterInfo.Region,
			"profile":    clusterInfo.AWSProfile,
			"account_id": clusterInfo.AWSAccountID,
			"arn":        clusterInfo.Arn,
			"version":    clusterInfo.Version,
			"namespace":  ns,
		}
		result, _ := json.MarshalIndent(info, "", "  ")
		output = string(result)
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetStats(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filters := parseClusterFilters(request)

	var output string
	var errMsg string

	suppressStderr(func() {
		clusterList, err := filters.loadClusters()
		if err != nil {
			errMsg = fmt.Sprintf("Error loading clusters: %v", err)
			return
		}

		k8sStatsList := []k8s.K8Sstats{}

		for _, clusterInfo := range clusterList {
			restConfig, err := GetRestConfigForCluster(clusterInfo)
			if err != nil {
				continue
			}

			stats, err := k8s.GetK8sStatsWithConfig(restConfig, clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, clusterInfo.Arn, clusterInfo.Version)
			if err != nil {
				continue
			}
			k8sStatsList = append(k8sStatsList, *stats)
		}

		if len(k8sStatsList) == 0 {
			output = "No stats available."
			return
		}

		output = captureOutput(func() {
			printutils.PrintK8SStats(false, k8sStatsList...)
		})
		saveCacheToDisk()
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetKarpenterNodePools(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filters := parseClusterFilters(request)

	var output string
	var errMsg string

	suppressStderr(func() {
		clusterList, err := filters.loadClusters()
		if err != nil {
			errMsg = fmt.Sprintf("Error loading clusters: %v", err)
			return
		}

		allNodePools := []data.KarpenterNodePoolInfo{}

		for _, clusterInfo := range clusterList {
			restConfig, err := GetRestConfigForCluster(clusterInfo)
			if err != nil {
				continue
			}

			nodePools, err := karpenter.GetNodePoolsWithConfig(restConfig, clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
			if err != nil {
				continue
			}
			allNodePools = append(allNodePools, nodePools...)
		}

		if len(allNodePools) == 0 {
			output = "No Karpenter NodePools found."
			return
		}

		output = captureOutput(func() {
			printutils.PrintKarpenterNodePools(false, true, allNodePools...)
		})
		saveCacheToDisk()
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetKarpenterNodeClaims(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filters := parseClusterFilters(request)

	var output string
	var errMsg string

	suppressStderr(func() {
		clusterList, err := filters.loadClusters()
		if err != nil {
			errMsg = fmt.Sprintf("Error loading clusters: %v", err)
			return
		}

		allNodeClaims := []data.KarpenterNodeClaimInfo{}

		for _, clusterInfo := range clusterList {
			restConfig, err := GetRestConfigForCluster(clusterInfo)
			if err != nil {
				continue
			}

			nodeClaims, err := karpenter.GetNodeClaimsWithConfig(restConfig, clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
			if err != nil {
				continue
			}
			allNodeClaims = append(allNodeClaims, nodeClaims...)
		}

		if len(allNodeClaims) == 0 {
			output = "No Karpenter NodeClaims found."
			return
		}

		output = captureOutput(func() {
			printutils.PrintKarpenterNodeClaims(false, true, allNodeClaims...)
		})
		saveCacheToDisk()
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetKarpenterDrift(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filters := parseClusterFilters(request)

	var output string
	var errMsg string

	suppressStderr(func() {
		clusterList, err := filters.loadClusters()
		if err != nil {
			errMsg = fmt.Sprintf("Error loading clusters: %v", err)
			return
		}

		allDrift := []data.KarpenterDriftInfo{}

		for _, clusterInfo := range clusterList {
			restConfig, err := GetRestConfigForCluster(clusterInfo)
			if err != nil {
				continue
			}

			drifted, err := karpenter.GetDriftedResourcesWithConfig(restConfig, clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
			if err != nil {
				continue
			}
			allDrift = append(allDrift, drifted...)
		}

		if len(allDrift) == 0 {
			output = "No drifted Karpenter resources found."
			return
		}

		output = captureOutput(func() {
			printutils.PrintKarpenterDrift(false, allDrift...)
		})
		saveCacheToDisk()
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetPodIdentity(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	namespace := request.GetString("namespace", "")

	var output string
	var errMsg string

	suppressStderr(func() {
		clusterInfo, err := GetCurrentClusterInfo()
		if err != nil {
			errMsg = fmt.Sprintf("Error getting current cluster: %v", err)
			return
		}

		associations, err := eks.GetPodIdentityAssociations(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
		if err != nil {
			errMsg = fmt.Sprintf("Error getting pod identity associations: %v", err)
			return
		}

		filtered := []data.PodIdentityInfo{}
		for _, assoc := range associations {
			if namespace != "" && assoc.Namespace != namespace {
				continue
			}
			filtered = append(filtered, assoc)
		}

		if len(filtered) == 0 {
			output = "No EKS Pod Identity associations found."
			return
		}

		output = captureOutput(func() {
			printutils.PrintPodIdentity(false, filtered...)
		})
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetIRSA(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	namespace := request.GetString("namespace", "")

	var output string
	var errMsg string

	suppressStderr(func() {
		clusterInfo, err := GetCurrentClusterInfo()
		if err != nil {
			errMsg = fmt.Sprintf("Error getting current cluster: %v", err)
			return
		}

		serviceAccounts, err := k8s.GetServiceAccountsWithIRSA(context.Background(), namespace)
		if err != nil {
			errMsg = fmt.Sprintf("Error getting IRSA service accounts: %v", err)
			return
		}

		irsaInfos := make([]data.IRSAInfo, 0)
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

		if len(irsaInfos) == 0 {
			output = "No service accounts with IRSA annotations found."
			return
		}

		output = captureOutput(func() {
			printutils.PrintIRSA(false, irsaInfos...)
		})
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetUpdates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var output string
	var errMsg string

	suppressStderr(func() {
		clusterInfo, err := GetCurrentClusterInfo()
		if err != nil {
			errMsg = fmt.Sprintf("Error getting current cluster: %v", err)
			return
		}

		updateList, err := eks.GetEKSUpdates(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
		if err != nil {
			errMsg = fmt.Sprintf("Error getting updates: %v", err)
			return
		}

		if len(updateList) == 0 {
			output = "No updates available."
			return
		}

		output = captureOutput(func() {
			printutils.PrintUpdates(false, updateList...)
		})
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetFargateProfiles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var output string
	var errMsg string

	suppressStderr(func() {
		clusterInfo, err := GetCurrentClusterInfo()
		if err != nil {
			errMsg = fmt.Sprintf("Error getting current cluster: %v", err)
			return
		}

		profiles, err := eks.GetEKSFargateProfiles(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
		if err != nil {
			errMsg = fmt.Sprintf("Error getting Fargate profiles: %v", err)
			return
		}

		if len(profiles) == 0 {
			output = "No Fargate profiles found."
			return
		}

		output = captureOutput(func() {
			printutils.PrintFargateProfiles(false, profiles...)
		})
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetQuotas(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	namespace := request.GetString("namespace", "")
	allNamespaces := request.GetBool("all_namespaces", false)

	var output string
	var errMsg string

	suppressStderr(func() {
		clusterInfo, err := GetCurrentClusterInfo()
		if err != nil {
			errMsg = fmt.Sprintf("Error getting current cluster: %v", err)
			return
		}

		if !allNamespaces && namespace == "" {
			ns, err := k8s.GetCurrentNamespace()
			if err != nil {
				namespace = "default"
			} else {
				namespace = ns
			}
		}

		if allNamespaces {
			namespace = ""
		}

		quotas, err := k8s.GetResourceQuotas(context.Background(), namespace)
		if err != nil {
			errMsg = fmt.Sprintf("Error getting quotas: %v", err)
			return
		}

		quotaInfos := make([]data.ResourceQuotaInfo, 0)
		for _, quota := range quotas {
			for resourceName, hardLimit := range quota.Status.Hard {
				used := quota.Status.Used[resourceName]
				info := data.ResourceQuotaInfo{
					Profile:      clusterInfo.AWSProfile,
					Region:       clusterInfo.Region,
					ClusterName:  clusterInfo.ClusterName,
					Namespace:    quota.Namespace,
					QuotaName:    quota.Name,
					ResourceName: string(resourceName),
					Hard:         hardLimit.String(),
					Used:         used.String(),
				}
				quotaInfos = append(quotaInfos, info)
			}
		}

		if len(quotaInfos) == 0 {
			output = "No resource quotas found."
			return
		}

		output = captureOutput(func() {
			printutils.PrintResourceQuotas(false, quotaInfos...)
		})
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(output), nil
}

func handleGetResources(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filters := parseClusterFilters(request)

	resourceType := request.GetString("resource_type", "")
	resourceName := request.GetString("resource_name", "")
	namespace := request.GetString("namespace", "")
	allNamespaces := request.GetBool("all_namespaces", false)
	contains := request.GetString("resource_contains", "")
	filter := request.GetString("filter", "")
	output := request.GetString("output", "")

	if resourceType == "" {
		return mcp.NewToolResultError("resource_type is required"), nil
	}

	var result string
	var errMsg string

	suppressStderr(func() {
		clusterList, err := filters.loadClusters()
		if err != nil {
			errMsg = fmt.Sprintf("Error loading clusters: %v", err)
			return
		}

		result = captureOutput(func() {
			runGenericListing(clusterList, resourceType, resourceName, namespace, allNamespaces, "", contains, filter, output, false)
		})

		if result == "" {
			result = fmt.Sprintf("No %s found.", resourceType)
		}

		saveCacheToDisk()
	})

	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	return mcp.NewToolResultText(result), nil
}

// --- JSON Helpers ---

func clustersToJSON(clusters []data.ClusterInfo) string {
	type clusterJSON struct {
		Name      string `json:"name"`
		Region    string `json:"region"`
		Profile   string `json:"profile"`
		AccountID string `json:"account_id"`
		Version   string `json:"version"`
		Status    string `json:"status"`
		ARN       string `json:"arn"`
		CreatedAt string `json:"created_at,omitempty"`
	}

	items := make([]clusterJSON, len(clusters))
	for i, c := range clusters {
		items[i] = clusterJSON{
			Name:      c.ClusterName,
			Region:    c.Region,
			Profile:   c.AWSProfile,
			AccountID: c.AWSAccountID,
			Version:   c.Version,
			Status:    c.Status,
			ARN:       c.Arn,
			CreatedAt: c.CreatedAt,
		}
	}

	result, _ := json.MarshalIndent(items, "", "  ")
	return string(result)
}
