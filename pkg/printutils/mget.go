package printutils

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/printers"
)

func PrintGenericResults(results []data.ResourceResult, output string, noHeaders bool) {
	if len(results) == 0 {
		return
	}

	// Sort results by Profile, Region, ClusterName, Namespace, Name
	sort.Slice(results, func(i, j int) bool {
		if results[i].Profile != results[j].Profile {
			return results[i].Profile < results[j].Profile
		}
		if results[i].Region != results[j].Region {
			return results[i].Region < results[j].Region
		}
		if results[i].ClusterName != results[j].ClusterName {
			return results[i].ClusterName < results[j].ClusterName
		}
		if results[i].Namespace != results[j].Namespace {
			return results[i].Namespace < results[j].Namespace
		}
		return results[i].Name < results[j].Name
	})

	// Handle JSON output
	if output == "json" {
		jsonBytes, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(jsonBytes))
		return
	}

	// Handle YAML output
	if output == "yaml" {
		for i, result := range results {
			if result.Error != "" {
				fmt.Printf("# Error for %s/%s in %s: %s\n", result.Namespace, result.Name, result.ClusterName, result.Error)
				continue
			}
			yamlBytes, _ := yaml.Marshal(result.Data)
			fmt.Println("---")
			fmt.Print(string(yamlBytes))
			if i < len(results)-1 {
				fmt.Println()
			}
		}
		return
	}

	// Check if all results are PriorityClass
	isPriorityClass := false
	for _, result := range results {
		if strings.ToLower(result.Kind) == "priorityclass" {
			isPriorityClass = true
			break
		}
	}

	if isPriorityClass {
		printPriorityClassResults(results, noHeaders)
		return
	}

	// Check if results are cluster-scoped (no namespace)
	isClusterScoped := true
	for _, result := range results {
		if result.Namespace != "" {
			isClusterScoped = false
			break
		}
	}

	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	var table *v1.Table
	if output == "wide" {
		if isClusterScoped {
			table = &v1.Table{
				ColumnDefinitions: []v1.TableColumnDefinition{
					{Name: "AWS PROFILE", Type: "string"},
					{Name: "AWS REGION", Type: "string"},
					{Name: "CLUSTER NAME", Type: "string"},
					{Name: "KIND", Type: "string"},
					{Name: "NAME", Type: "string"},
					{Name: "STATUS", Type: "string"},
					{Name: "AGE", Type: "string"},
					{Name: "ADDITIONAL INFO", Type: "string"},
				},
			}
		} else {
			table = &v1.Table{
				ColumnDefinitions: []v1.TableColumnDefinition{
					{Name: "AWS PROFILE", Type: "string"},
					{Name: "AWS REGION", Type: "string"},
					{Name: "CLUSTER NAME", Type: "string"},
					{Name: "NAMESPACE", Type: "string"},
					{Name: "KIND", Type: "string"},
					{Name: "NAME", Type: "string"},
					{Name: "STATUS", Type: "string"},
					{Name: "AGE", Type: "string"},
					{Name: "ADDITIONAL INFO", Type: "string"},
				},
			}
		}
	} else {
		if isClusterScoped {
			table = &v1.Table{
				ColumnDefinitions: []v1.TableColumnDefinition{
					{Name: "AWS PROFILE", Type: "string"},
					{Name: "AWS REGION", Type: "string"},
					{Name: "CLUSTER NAME", Type: "string"},
					{Name: "KIND", Type: "string"},
					{Name: "NAME", Type: "string"},
					{Name: "STATUS", Type: "string"},
					{Name: "AGE", Type: "string"},
				},
			}
		} else {
			table = &v1.Table{
				ColumnDefinitions: []v1.TableColumnDefinition{
					{Name: "AWS PROFILE", Type: "string"},
					{Name: "AWS REGION", Type: "string"},
					{Name: "CLUSTER NAME", Type: "string"},
					{Name: "NAMESPACE", Type: "string"},
					{Name: "KIND", Type: "string"},
					{Name: "NAME", Type: "string"},
					{Name: "STATUS", Type: "string"},
					{Name: "AGE", Type: "string"},
				},
			}
		}
	}

	for _, result := range results {
		status := result.Status
		if status == "" {
			status = "-"
		}
		age := extractAge(result.Data)

		if result.Error != "" {
			status = fmt.Sprintf("ERROR: %s", result.Error)
			age = "-"
		}

		if output == "wide" {
			additionalInfo := "-"
			if result.Error == "" {
				additionalInfo = extractAdditionalInfo(result.Data, result.Kind)
			}

			if isClusterScoped {
				table.Rows = append(table.Rows, v1.TableRow{
					Cells: []interface{}{
						result.Profile,
						result.Region,
						result.ClusterName,
						result.Kind,
						result.Name,
						status,
						age,
						additionalInfo,
					},
				})
			} else {
				namespace := result.Namespace
				if namespace == "" {
					namespace = "-"
				}
				table.Rows = append(table.Rows, v1.TableRow{
					Cells: []interface{}{
						result.Profile,
						result.Region,
						result.ClusterName,
						namespace,
						result.Kind,
						result.Name,
						status,
						age,
						additionalInfo,
					},
				})
			}
		} else {
			if isClusterScoped {
				table.Rows = append(table.Rows, v1.TableRow{
					Cells: []interface{}{
						result.Profile,
						result.Region,
						result.ClusterName,
						result.Kind,
						result.Name,
						status,
						age,
					},
				})
			} else {
				namespace := result.Namespace
				if namespace == "" {
					namespace = "-"
				}
				table.Rows = append(table.Rows, v1.TableRow{
					Cells: []interface{}{
						result.Profile,
						result.Region,
						result.ClusterName,
						namespace,
						result.Kind,
						result.Name,
						status,
						age,
					},
				})
			}
		}
	}

	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

func printPriorityClassResults(results []data.ResourceResult, noHeaders bool) {
	// Sort results by Profile, Region, ClusterName, Name
	sort.Slice(results, func(i, j int) bool {
		if results[i].Profile != results[j].Profile {
			return results[i].Profile < results[j].Profile
		}
		if results[i].Region != results[j].Region {
			return results[i].Region < results[j].Region
		}
		if results[i].ClusterName != results[j].ClusterName {
			return results[i].ClusterName < results[j].ClusterName
		}
		return results[i].Name < results[j].Name
	})

	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	table := &v1.Table{
		ColumnDefinitions: []v1.TableColumnDefinition{
			{Name: "AWS PROFILE", Type: "string"},
			{Name: "AWS REGION", Type: "string"},
			{Name: "CLUSTER NAME", Type: "string"},
			{Name: "NAME", Type: "string"},
			{Name: "VALUE", Type: "integer"},
			{Name: "GLOBAL-DEFAULT", Type: "boolean"},
			{Name: "AGE", Type: "string"},
			{Name: "PREEMPTIONPOLICY", Type: "string"},
		},
	}

	for _, result := range results {
		if result.Error != "" {
			table.Rows = append(table.Rows, v1.TableRow{
				Cells: []interface{}{
					result.Profile,
					result.Region,
					result.ClusterName,
					result.Name,
					fmt.Sprintf("ERROR: %s", result.Error),
					"",
					"",
					"",
				},
			})
			continue
		}

		obj, ok := result.Data.(map[string]interface{})
		if !ok {
			continue
		}

		value, _, _ := unstructured.NestedInt64(obj, "value")
		globalDefault, _, _ := unstructured.NestedBool(obj, "globalDefault")
		preemptionPolicy, _, _ := unstructured.NestedString(obj, "preemptionPolicy")
		age := extractAge(result.Data)

		table.Rows = append(table.Rows, v1.TableRow{
			Cells: []interface{}{
				result.Profile,
				result.Region,
				result.ClusterName,
				result.Name,
				value,
				globalDefault,
				age,
				preemptionPolicy,
			},
		})
	}

	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

// extractAge extracts the age of a resource from its creation timestamp
func extractAge(data interface{}) string {
	obj, ok := data.(map[string]interface{})
	if !ok {
		return "-"
	}

	creationTimestamp, found, _ := unstructured.NestedString(obj, "metadata", "creationTimestamp")
	if !found || creationTimestamp == "" {
		return "-"
	}

	t, err := time.Parse(time.RFC3339, creationTimestamp)
	if err != nil {
		return "-"
	}

	return formatAge(t)
}

func extractNodeWideInfo(obj map[string]interface{}) string {
	addresses, found, _ := unstructured.NestedSlice(obj, "status", "addresses")
	if !found {
		return "-"
	}

	var internalIP, externalIP string
	for _, addr := range addresses {
		addrMap, ok := addr.(map[string]interface{})
		if !ok {
			continue
		}
		addrType, _ := addrMap["type"].(string)
		addrValue, _ := addrMap["address"].(string)

		switch addrType {
		case "InternalIP":
			internalIP = addrValue
		case "ExternalIP":
			externalIP = addrValue
		}
	}

	if externalIP != "" && internalIP != "" {
		return fmt.Sprintf("Internal: %s, External: %s", internalIP, externalIP)
	}
	if internalIP != "" {
		return fmt.Sprintf("Internal: %s", internalIP)
	}
	return "-"
}

// extractAdditionalInfo extracts additional info for wide output based on resource kind
func extractAdditionalInfo(data interface{}, kind string) string {
	obj, ok := data.(map[string]interface{})
	if !ok {
		return "-"
	}

	switch strings.ToLower(kind) {
	case "pod":
		return extractPodWideInfo(obj)
	case "node":
		return extractNodeWideInfo(obj)
	case "service":
		return extractServiceWideInfo(obj)
	case "deployment":
		return extractDeploymentWideInfo(obj)
	case "persistentvolumeclaim":
		return extractPVCWideInfo(obj)
	case "persistentvolume":
		return extractPVWideInfo(obj)
	case "ingress":
		return extractIngressWideInfo(obj)
	case "priorityclass":
		return extractPriorityClassWideInfo(obj)
	default:
		return "-"
	}
}

func extractPriorityClassWideInfo(obj map[string]interface{}) string {
	value, valueFound, _ := unstructured.NestedInt64(obj, "value")
	globalDefault, _, _ := unstructured.NestedBool(obj, "globalDefault")
	preemptionPolicy, _, _ := unstructured.NestedString(obj, "preemptionPolicy")

	info := []string{}

	if valueFound {
		info = append(info, fmt.Sprintf("Value: %d", value))
	}

	info = append(info, fmt.Sprintf("GlobalDefault: %t", globalDefault))

	if preemptionPolicy != "" {
		info = append(info, fmt.Sprintf("Preemption: %s", preemptionPolicy))
	}

	if len(info) > 0 {
		return strings.Join(info, " | ")
	}
	return "-"
}

func extractPodWideInfo(obj map[string]interface{}) string {
	ip, _, _ := unstructured.NestedString(obj, "status", "podIP")
	node, _, _ := unstructured.NestedString(obj, "spec", "nodeName")

	if ip != "" && node != "" {
		return fmt.Sprintf("IP: %s, Node: %s", ip, node)
	}
	if ip != "" {
		return fmt.Sprintf("IP: %s", ip)
	}
	if node != "" {
		return fmt.Sprintf("Node: %s", node)
	}
	return "-"
}

func extractServiceWideInfo(obj map[string]interface{}) string {
	clusterIP, _, _ := unstructured.NestedString(obj, "spec", "clusterIP")
	externalIPs, _, _ := unstructured.NestedStringSlice(obj, "spec", "externalIPs")
	ports, found, _ := unstructured.NestedSlice(obj, "spec", "ports")

	info := []string{}
	if clusterIP != "" {
		info = append(info, fmt.Sprintf("ClusterIP: %s", clusterIP))
	}
	if len(externalIPs) > 0 {
		info = append(info, fmt.Sprintf("ExternalIP: %s", strings.Join(externalIPs, ",")))
	}
	if found && len(ports) > 0 {
		portStrs := []string{}
		for _, port := range ports {
			portMap, ok := port.(map[string]interface{})
			if !ok {
				continue
			}
			portNum, _ := portMap["port"].(int64)
			protocol, _ := portMap["protocol"].(string)
			if portNum > 0 {
				portStrs = append(portStrs, fmt.Sprintf("%d/%s", portNum, protocol))
			}
		}
		if len(portStrs) > 0 {
			info = append(info, fmt.Sprintf("Ports: %s", strings.Join(portStrs, ",")))
		}
	}

	if len(info) > 0 {
		return strings.Join(info, " | ")
	}
	return "-"
}

func extractDeploymentWideInfo(obj map[string]interface{}) string {
	selector, found, _ := unstructured.NestedMap(obj, "spec", "selector", "matchLabels")
	if !found {
		return "-"
	}

	labels := []string{}
	for k, v := range selector {
		if strVal, ok := v.(string); ok {
			labels = append(labels, fmt.Sprintf("%s=%s", k, strVal))
		}
	}

	if len(labels) > 0 {
		return fmt.Sprintf("Selector: %s", strings.Join(labels, ","))
	}
	return "-"
}

func extractPVCWideInfo(obj map[string]interface{}) string {
	volumeName, _, _ := unstructured.NestedString(obj, "spec", "volumeName")
	storageClass, _, _ := unstructured.NestedString(obj, "spec", "storageClassName")
	capacity, found, _ := unstructured.NestedMap(obj, "status", "capacity")

	info := []string{}
	if volumeName != "" {
		info = append(info, fmt.Sprintf("Volume: %s", volumeName))
	}
	if storageClass != "" {
		info = append(info, fmt.Sprintf("StorageClass: %s", storageClass))
	}
	if found {
		if storage, ok := capacity["storage"].(string); ok {
			info = append(info, fmt.Sprintf("Capacity: %s", storage))
		}
	}

	if len(info) > 0 {
		return strings.Join(info, " | ")
	}
	return "-"
}

func extractPVWideInfo(obj map[string]interface{}) string {
	capacity, found, _ := unstructured.NestedMap(obj, "spec", "capacity")
	storageClass, _, _ := unstructured.NestedString(obj, "spec", "storageClassName")

	info := []string{}
	if found {
		if storage, ok := capacity["storage"].(string); ok {
			info = append(info, fmt.Sprintf("Capacity: %s", storage))
		}
	}
	if storageClass != "" {
		info = append(info, fmt.Sprintf("StorageClass: %s", storageClass))
	}

	if len(info) > 0 {
		return strings.Join(info, " | ")
	}
	return "-"
}

func extractIngressWideInfo(obj map[string]interface{}) string {
	ingressClass, _, _ := unstructured.NestedString(obj, "spec", "ingressClassName")
	rules, found, _ := unstructured.NestedSlice(obj, "spec", "rules")

	info := []string{}
	if ingressClass != "" {
		info = append(info, fmt.Sprintf("Class: %s", ingressClass))
	}

	if found && len(rules) > 0 {
		hosts := []string{}
		for _, rule := range rules {
			ruleMap, ok := rule.(map[string]interface{})
			if !ok {
				continue
			}
			if host, ok := ruleMap["host"].(string); ok && host != "" {
				hosts = append(hosts, host)
			}
		}
		if len(hosts) > 0 {
			info = append(info, fmt.Sprintf("Hosts: %s", strings.Join(hosts, ",")))
		}
	}

	if len(info) > 0 {
		return strings.Join(info, " | ")
	}
	return "-"
}

func formatAge(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	return fmt.Sprintf("%dd", int(duration.Hours()/24))
}
