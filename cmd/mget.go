package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/jordiprats/kubectl-eks/pkg/status"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/jsonpath"
)

var mGetCmd = &cobra.Command{
	Use:   "mget [resource-type] [resource-name]",
	Short: "Get resources from multiple clusters",
	Long: `Get Kubernetes resources from all clusters that match a filter.
Similar to 'kubectl get' but works across multiple EKS clusters.

Supports output formats:
  -o wide          Additional details
  -o json          JSON output
  -o yaml          YAML output
  -o jsonpath=...  Extract specific fields using JSONPath`,
	Example: `  # List all pods across clusters
  kubectl eks mget pods

  # List pods in specific namespace
  kubectl eks mget pods -n kube-system

  # Get a specific deployment
  kubectl eks mget deployment my-app

  # Extract specific fields with JSONPath
  kubectl eks mget pods -o jsonpath='{.spec.dnsPolicy}'
  
  # List deployments with additional details
  kubectl eks mget deployments -o wide
  
  # Filter clusters and resources
  kubectl eks mget pods --cluster-contains prod --resource-starts-with nginx

  # Filter resources by name substring
  kubectl eks mget pods --cluster-contains prod --resource-contains api

  # Filter by any field value (dot-notation path into the resource)
  kubectl eks mget pods --filter status.phase=Running
  kubectl eks mget deployments --filter status.readyReplicas=3
  kubectl eks mget nodes --filter status.nodeInfo.kubeletVersion=v1.29.0
  
  # Works with any resource including CRDs
  kubectl eks mget ec2nodeclass -A`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		resourceType := args[0]
		var resourceName string
		if len(args) > 1 {
			resourceName = args[1]
		}

		// Get flags
		refresh, _ := cmd.Flags().GetBool("refresh")
		profile, _ := cmd.Flags().GetString("profile")
		profileContains, _ := cmd.Flags().GetString("profile-contains")
		nameContains, _ := cmd.Flags().GetString("cluster-contains")
		nameNotContains, _ := cmd.Flags().GetString("cluster-not-contains")
		region, _ := cmd.Flags().GetString("region")
		version, _ := cmd.Flags().GetString("version")
		namespace, _ := cmd.Flags().GetString("namespace")
		allNamespaces, _ := cmd.Flags().GetBool("all-namespaces")
		output, _ := cmd.Flags().GetString("output")
		startsWith, _ := cmd.Flags().GetString("resource-starts-with")
		contains, _ := cmd.Flags().GetString("resource-contains")
		noHeaders, _ := cmd.Flags().GetBool("no-headers")
		filter, _ := cmd.Flags().GetString("filter")

		// Load cluster list
		clusterList, err := LoadClusterList([]string{}, profile, profileContains, nameContains, nameNotContains, region, version, refresh)
		if err != nil {
			log.Fatalf("Error loading cluster list: %v", err)
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

		// Check if JSONPath output
		if strings.HasPrefix(output, "jsonpath=") {
			jsonpathExpr := strings.TrimPrefix(output, "jsonpath=")
			runJsonPathQuery(clusterList, resourceType, resourceName, jsonpathExpr, namespace, allNamespaces, startsWith, contains, filter, noHeaders)
		} else if (resourceType == "pods" || resourceType == "pod" || resourceType == "po") && output == "" && filter == "" {
			// Use existing pod listing functionality only for default output without filter
			runPodListing(clusterList, namespace, allNamespaces, startsWith, contains, noHeaders)
		} else {
			// Generic resource listing using dynamic client
			runGenericListing(clusterList, resourceType, resourceName, namespace, allNamespaces, startsWith, contains, filter, output, noHeaders)
		}

		saveCacheToDisk()
	},
}

func runPodListing(clusterList []data.ClusterInfo, namespace string, allNamespaces bool, startsWith, contains string, noHeaders bool) {
	k8SClusterPodList := []k8s.K8SClusterPodList{}

	for _, clusterInfo := range clusterList {
		err := eks.UpdateKubeConfig(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, "")
		if err != nil {
			log.Printf("Warning: Failed to update kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
			continue
		}

		k8sPodList, err := k8s.GetPods(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, clusterInfo.Arn, clusterInfo.Version, namespace, allNamespaces)
		if err != nil {
			log.Printf("Warning: Failed to get pods from cluster %s: %v", clusterInfo.ClusterName, err)
			continue
		}

		if startsWith != "" || contains != "" {
			filteredPods := make([]k8s.K8SPodInfo, 0, len(k8sPodList.Pods))
			for _, pod := range k8sPodList.Pods {
				if startsWith != "" && !strings.HasPrefix(pod.Name, startsWith) {
					continue
				}
				if contains != "" && !strings.Contains(pod.Name, contains) {
					continue
				}
				filteredPods = append(filteredPods, pod)
			}
			k8sPodList.Pods = filteredPods
		}

		k8SClusterPodList = append(k8SClusterPodList, *k8sPodList)
	}

	printutils.PrintMultiGetPods(noHeaders, k8SClusterPodList...)
}

// matchesFilter checks if an unstructured object matches a filter expression.
// Filter format: "field.path=value" where field.path is a dot-notation path
// into the resource and value is matched as an exact string.
func matchesFilter(obj map[string]interface{}, filter string) bool {
	if filter == "" {
		return true
	}

	parts := strings.SplitN(filter, "=", 2)
	if len(parts) != 2 {
		return true
	}

	fieldPath := parts[0]
	expectedValue := parts[1]

	// Walk the dot-notation path
	keys := strings.Split(fieldPath, ".")
	var current interface{} = obj
	for _, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return false
		}
		current, ok = m[key]
		if !ok {
			return false
		}
	}

	actualValue := fmt.Sprintf("%v", current)
	return actualValue == expectedValue
}

func runGenericListing(clusterList []data.ClusterInfo, resourceType, resourceName, namespace string, allNamespaces bool, startsWith, contains, filter, output string, noHeaders bool) {
	results := []data.ResourceResult{}

	for _, clusterInfo := range clusterList {
		err := eks.UpdateKubeConfig(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, "")
		if err != nil {
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

		// Create dynamic client for generic resource access
		dynamicClient, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			continue
		}

		// Create discovery client to resolve resource types
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
		if err != nil {
			continue
		}

		// Resolve the resource type to GVR
		gvr, namespaced, err := resolveResourceType(discoveryClient, resourceType)
		if err != nil {
			results = append(results, data.ResourceResult{
				Profile:     clusterInfo.AWSProfile,
				Region:      clusterInfo.Region,
				ClusterName: clusterInfo.ClusterName,
				Error:       fmt.Sprintf("Failed to resolve resource type '%s': %v", resourceType, err),
			})
			continue
		}

		namespaces := []string{}
		if namespaced {
			if allNamespaces {
				// Create typed client just for listing namespaces
				clientset, err := kubernetes.NewForConfig(restConfig)
				if err != nil {
					continue
				}
				nsList, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
				if err == nil {
					for _, ns := range nsList.Items {
						namespaces = append(namespaces, ns.Name)
					}
				}
			} else if namespace != "" {
				namespaces = append(namespaces, namespace)
			} else {
				ns, _, err := clientConfig.Namespace()
				if err != nil {
					namespaces = append(namespaces, "default")
				} else {
					namespaces = append(namespaces, ns)
				}
			}
		} else {
			// Cluster-scoped resource
			namespaces = append(namespaces, "")
		}

		for _, ns := range namespaces {
			var resourceInterface dynamic.ResourceInterface
			if namespaced && ns != "" {
				resourceInterface = dynamicClient.Resource(gvr).Namespace(ns)
			} else {
				resourceInterface = dynamicClient.Resource(gvr)
			}

			if resourceName != "" {
				// Get single resource
				obj, err := resourceInterface.Get(context.Background(), resourceName, metav1.GetOptions{})
				if err != nil {
					results = append(results, data.ResourceResult{
						Profile:     clusterInfo.AWSProfile,
						Region:      clusterInfo.Region,
						ClusterName: clusterInfo.ClusterName,
						Namespace:   ns,
						Name:        resourceName,
						Error:       err.Error(),
					})
					continue
				}
				results = append(results, data.ResourceResult{
					Profile:     clusterInfo.AWSProfile,
					Region:      clusterInfo.Region,
					ClusterName: clusterInfo.ClusterName,
					Namespace:   ns,
					Name:        obj.GetName(),
					Kind:        obj.GetKind(),
					Data:        obj.Object,
					Status:      status.ExtractStatus(obj.Object, obj.GetKind()),
				})
			} else {
				// List resources
				list, err := resourceInterface.List(context.Background(), metav1.ListOptions{})
				if err != nil {
					results = append(results, data.ResourceResult{
						Profile:     clusterInfo.AWSProfile,
						Region:      clusterInfo.Region,
						ClusterName: clusterInfo.ClusterName,
						Namespace:   ns,
						Error:       err.Error(),
					})
					continue
				}

				// Apply name and field filters
				for _, item := range list.Items {
					name := item.GetName()
					if startsWith != "" && !strings.HasPrefix(name, startsWith) {
						continue
					}
					if contains != "" && !strings.Contains(name, contains) {
						continue
					}
					if !matchesFilter(item.Object, filter) {
						continue
					}
					results = append(results, data.ResourceResult{
						Profile:     clusterInfo.AWSProfile,
						Region:      clusterInfo.Region,
						ClusterName: clusterInfo.ClusterName,
						Namespace:   ns,
						Name:        name,
						Kind:        item.GetKind(),
						Data:        item.Object,
						Status:      status.ExtractStatus(item.Object, item.GetKind()),
					})
				}
			}
		}
	}

	// Print results based on output format
	printutils.PrintGenericResults(results, output, noHeaders)
}

// resolveResourceType converts a resource type string (like "pods", "po", "deploy") to a GroupVersionResource
func resolveResourceType(discoveryClient *discovery.DiscoveryClient, resourceType string) (schema.GroupVersionResource, bool, error) {
	// Common short names mapping
	shortNames := map[string]schema.GroupVersionResource{
		"po":            {Group: "", Version: "v1", Resource: "pods"},
		"pod":           {Group: "", Version: "v1", Resource: "pods"},
		"pods":          {Group: "", Version: "v1", Resource: "pods"},
		"svc":           {Group: "", Version: "v1", Resource: "services"},
		"service":       {Group: "", Version: "v1", Resource: "services"},
		"deploy":        {Group: "apps", Version: "v1", Resource: "deployments"},
		"ds":            {Group: "apps", Version: "v1", Resource: "daemonsets"},
		"sts":           {Group: "apps", Version: "v1", Resource: "statefulsets"},
		"cm":            {Group: "", Version: "v1", Resource: "configmaps"},
		"pdb":           {Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"},
		"ing":           {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		"no":            {Group: "", Version: "v1", Resource: "nodes"},
		"node":          {Group: "", Version: "v1", Resource: "nodes"},
		"ns":            {Group: "", Version: "v1", Resource: "namespaces"},
		"pc":            {Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"},
		"priorityclass": {Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"},
	}

	// Check if it's a known short name
	if gvr, ok := shortNames[strings.ToLower(resourceType)]; ok {
		namespaced := !isClusterScoped(gvr.Resource)
		return gvr, namespaced, nil
	}

	// Use discovery to find the resource
	apiResourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		// Partial errors are okay
		if apiResourceLists == nil {
			return schema.GroupVersionResource{}, false, err
		}
	}

	// Normalize resource type (add 's' if not present for plural)
	resourceTypeLower := strings.ToLower(resourceType)

	for _, apiResourceList := range apiResourceLists {
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			continue
		}

		for _, apiResource := range apiResourceList.APIResources {
			// Match by name or short names
			if strings.ToLower(apiResource.Name) == resourceTypeLower ||
				strings.ToLower(apiResource.SingularName) == resourceTypeLower {
				return schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: apiResource.Name,
				}, apiResource.Namespaced, nil
			}

			// Check short names
			for _, shortName := range apiResource.ShortNames {
				if strings.ToLower(shortName) == resourceTypeLower {
					return schema.GroupVersionResource{
						Group:    gv.Group,
						Version:  gv.Version,
						Resource: apiResource.Name,
					}, apiResource.Namespaced, nil
				}
			}
		}
	}

	return schema.GroupVersionResource{}, false, fmt.Errorf("resource type '%s' not found", resourceType)
}

func isClusterScoped(resource string) bool {
	clusterScoped := map[string]bool{
		"nodes":                     true,
		"namespaces":                true,
		"persistentvolumes":         true,
		"clusterroles":              true,
		"clusterrolebindings":       true,
		"storageclasses":            true,
		"customresourcedefinitions": true,
		"priorityclasses":           true,
	}
	return clusterScoped[resource]
}

func runJsonPathQuery(clusterList []data.ClusterInfo, resourceType, resourceName, jsonpathExpr, namespace string, allNamespaces bool, startsWith, contains, filter string, noHeaders bool) {
	// Normalize JSONPath expression
	jsonpathExpr = strings.TrimSpace(jsonpathExpr)
	if strings.HasPrefix(jsonpathExpr, "{") && strings.HasSuffix(jsonpathExpr, "}") {
		jsonpathExpr = strings.TrimPrefix(jsonpathExpr, "{")
		jsonpathExpr = strings.TrimSuffix(jsonpathExpr, "}")
	}

	// Prepare JSONPath parser
	jp := jsonpath.New("jsonpath")
	parseExpr := jsonpathExpr
	if !strings.HasPrefix(parseExpr, "{") {
		parseExpr = "{" + parseExpr + "}"
	}
	if err := jp.Parse(parseExpr); err != nil {
		log.Fatalf("Error parsing JSONPath expression '%s': %v", jsonpathExpr, err)
	}

	results := []data.JsonPathResult{}

	for _, clusterInfo := range clusterList {
		err := eks.UpdateKubeConfig(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, "")
		if err != nil {
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

		dynamicClient, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			continue
		}

		discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
		if err != nil {
			continue
		}

		gvr, namespaced, err := resolveResourceType(discoveryClient, resourceType)
		if err != nil {
			results = append(results, data.JsonPathResult{
				Profile:     clusterInfo.AWSProfile,
				Region:      clusterInfo.Region,
				ClusterName: clusterInfo.ClusterName,
				Error:       fmt.Sprintf("Failed to resolve resource type: %v", err),
			})
			continue
		}

		namespaces := []string{}
		if namespaced {
			if allNamespaces {
				clientset, err := kubernetes.NewForConfig(restConfig)
				if err != nil {
					continue
				}
				nsList, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
				if err == nil {
					for _, ns := range nsList.Items {
						namespaces = append(namespaces, ns.Name)
					}
				}
			} else if namespace != "" {
				namespaces = append(namespaces, namespace)
			} else {
				ns, _, err := clientConfig.Namespace()
				if err != nil {
					namespaces = append(namespaces, "default")
				} else {
					namespaces = append(namespaces, ns)
				}
			}
		} else {
			namespaces = append(namespaces, "")
		}

		for _, ns := range namespaces {
			var resourceInterface dynamic.ResourceInterface
			if namespaced && ns != "" {
				resourceInterface = dynamicClient.Resource(gvr).Namespace(ns)
			} else {
				resourceInterface = dynamicClient.Resource(gvr)
			}

			var objects []*unstructured.Unstructured
			var resourceNames []string

			if resourceName != "" {
				obj, err := resourceInterface.Get(context.Background(), resourceName, metav1.GetOptions{})
				if err != nil {
					results = append(results, data.JsonPathResult{
						Profile:     clusterInfo.AWSProfile,
						Region:      clusterInfo.Region,
						ClusterName: clusterInfo.ClusterName,
						Namespace:   ns,
						Resource:    resourceName,
						Error:       err.Error(),
					})
					continue
				}
				objects = append(objects, obj)
				resourceNames = append(resourceNames, obj.GetName())
			} else {
				list, err := resourceInterface.List(context.Background(), metav1.ListOptions{})
				if err != nil {
					results = append(results, data.JsonPathResult{
						Profile:     clusterInfo.AWSProfile,
						Region:      clusterInfo.Region,
						ClusterName: clusterInfo.ClusterName,
						Namespace:   ns,
						Resource:    "all",
						Error:       err.Error(),
					})
					continue
				}

				for _, item := range list.Items {
					name := item.GetName()
					if startsWith != "" && !strings.HasPrefix(name, startsWith) {
						continue
					}
					if contains != "" && !strings.Contains(name, contains) {
						continue
					}
					if !matchesFilter(item.Object, filter) {
						continue
					}

					itemCopy := item
					objects = append(objects, &itemCopy)
					resourceNames = append(resourceNames, name)
				}
			}

			// Execute JSONPath on each object
			for i, obj := range objects {
				values, err := jp.FindResults(obj.Object)
				if err != nil {
					results = append(results, data.JsonPathResult{
						Profile:     clusterInfo.AWSProfile,
						Region:      clusterInfo.Region,
						ClusterName: clusterInfo.ClusterName,
						Namespace:   ns,
						Resource:    resourceNames[i],
						Error:       fmt.Sprintf("JSONPath error: %v", err),
					})
					continue
				}

				if len(values) == 0 || len(values[0]) == 0 {
					results = append(results, data.JsonPathResult{
						Profile:     clusterInfo.AWSProfile,
						Region:      clusterInfo.Region,
						ClusterName: clusterInfo.ClusterName,
						Namespace:   ns,
						Resource:    resourceNames[i],
						Value:       "<not found>",
					})
					continue
				}

				val := values[0][0].Interface()
				valueStr := formatValue(val)

				results = append(results, data.JsonPathResult{
					Profile:     clusterInfo.AWSProfile,
					Region:      clusterInfo.Region,
					ClusterName: clusterInfo.ClusterName,
					Namespace:   ns,
					Resource:    resourceNames[i],
					Value:       valueStr,
				})
			}
		}
	}

	printutils.PrintJsonPathResults(noHeaders, results)
}

func formatValue(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case int, int32, int64, float32, float64, bool:
		return fmt.Sprintf("%v", v)
	default:
		jsonBytes, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(jsonBytes)
	}
}

func init() {
	mGetCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	mGetCmd.Flags().StringP("profile", "p", "", "AWS profile to use")
	mGetCmd.Flags().StringP("profile-contains", "q", "", "AWS profile contains string")
	mGetCmd.Flags().StringP("cluster-contains", "c", "", "Cluster name contains string")
	mGetCmd.Flags().StringP("cluster-not-contains", "x", "", "Cluster name does not contain string")
	mGetCmd.Flags().StringP("region", "r", "", "AWS region to use")
	mGetCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	mGetCmd.Flags().StringP("namespace", "n", "", "Kubernetes namespace")
	mGetCmd.Flags().BoolP("all-namespaces", "A", false, "Query all Kubernetes namespaces")
	mGetCmd.Flags().StringP("output", "o", "", "Output format: wide|json|yaml|jsonpath=...")
	mGetCmd.Flags().StringP("resource-starts-with", "w", "", "Filter resources that start with this string")
	mGetCmd.Flags().String("resource-contains", "", "Filter resources that contain this string")
	mGetCmd.Flags().Bool("no-headers", false, "Don't print headers")
	mGetCmd.Flags().StringP("filter", "f", "", "Filter resources by field value (e.g., status.phase=Running)")

	rootCmd.AddCommand(mGetCmd)
}
