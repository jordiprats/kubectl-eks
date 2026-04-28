package printutils

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
)

func PrintMultiClusterNodes(noHeaders bool, wide bool, nodes []data.ClusterNodeInfo) {
	if len(nodes) == 0 {
		return
	}

	// Sort by Profile, Region, ClusterName, Name
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Profile != nodes[j].Profile {
			return nodes[i].Profile < nodes[j].Profile
		}
		if nodes[i].Region != nodes[j].Region {
			return nodes[i].Region < nodes[j].Region
		}
		if nodes[i].ClusterName != nodes[j].ClusterName {
			return nodes[i].ClusterName < nodes[j].ClusterName
		}
		return nodes[i].Node.Name < nodes[j].Node.Name
	})

	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	table := &v1.Table{
		ColumnDefinitions: []v1.TableColumnDefinition{
			{Name: "AWS PROFILE", Type: "string"},
			{Name: "AWS REGION", Type: "string"},
			{Name: "CLUSTER NAME", Type: "string"},
			{Name: "NAME", Type: "string"},
			{Name: "STATUS", Type: "string"},
			{Name: "INSTANCE TYPE", Type: "string"},
			{Name: "COMPUTE", Type: "string"},
			{Name: "MANAGED BY", Type: "string"},
		},
	}

	if wide {
		table.ColumnDefinitions = append(table.ColumnDefinitions,
			v1.TableColumnDefinition{Name: "CPU USED/TOTAL (REM)", Type: "string"},
			v1.TableColumnDefinition{Name: "MEMORY USED/TOTAL (REM)", Type: "string"},
			v1.TableColumnDefinition{Name: "PODS", Type: "string"},
			v1.TableColumnDefinition{Name: "CONDITIONS", Type: "string"},
		)
	}

	table.ColumnDefinitions = append(table.ColumnDefinitions, v1.TableColumnDefinition{Name: "AGE", Type: "string"})

	for _, n := range nodes {
		cells := []interface{}{
			n.Profile,
			n.Region,
			n.ClusterName,
			n.Node.Name,
			n.Node.Status,
			n.Node.InstanceType,
			n.Node.Compute,
			n.Node.ManagedBy,
		}

		if wide {
			cells = append(cells,
				formatCPUUsedTotalRemaining(n.Node.CPUUsed, n.Node.CPUCapacity, n.Node.CPUAllocatable),
				formatMemoryUsedTotalRemaining(n.Node.MemoryUsed, n.Node.MemoryCapacity, n.Node.MemoryAllocatable),
				n.Node.PodsRunning,
				formatNodeConditions(n.Node),
			)
		}

		cells = append(cells, formatAge(n.Node.Created))

		table.Rows = append(table.Rows, v1.TableRow{Cells: cells})
	}

	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

func formatNodeConditions(node data.NodeInfo) string {
	conditions := make([]string, 0, 4)

	if node.MemoryPressure == "True" {
		conditions = append(conditions, "MemoryPressure")
	}
	if node.DiskPressure == "True" {
		conditions = append(conditions, "DiskPressure")
	}
	if node.PIDPressure == "True" {
		conditions = append(conditions, "PIDPressure")
	}
	if node.NetworkUnavailable == "True" {
		conditions = append(conditions, "NetworkUnavailable")
	}

	if len(conditions) == 0 {
		return "-"
	}

	return strings.Join(conditions, ",")
}

func formatUsedTotal(used, total string) string {
	return fmt.Sprintf("%s/%s", used, total)
}

func formatUsedTotalRemaining(used, total, remaining string) string {
	return fmt.Sprintf("%s/%s (%s)", used, total, remaining)
}

func formatCPUUsedTotalRemaining(used, total, remaining string) string {
	return fmt.Sprintf("%s/%s (%s)", formatCPUQuantityCores(used), formatCPUQuantityCores(total), formatCPUQuantityCores(remaining))
}

func formatMemoryUsedTotalRemaining(used, total, remaining string) string {
	return fmt.Sprintf("%s/%s (%s)", formatMemoryQuantityGi(used), formatMemoryQuantityGi(total), formatMemoryQuantityGi(remaining))
}

func formatCPUQuantityCores(value string) string {
	if value == "-" {
		return value
	}

	q, err := resource.ParseQuantity(value)
	if err != nil {
		return value
	}

	cores := float64(q.MilliValue()) / 1000.0
	formatted := fmt.Sprintf("%.1f", cores)
	if strings.HasSuffix(formatted, ".0") {
		return strings.TrimSuffix(formatted, ".0")
	}

	return formatted
}

func formatMemoryQuantityGi(value string) string {
	if value == "-" {
		return value
	}

	q, err := resource.ParseQuantity(value)
	if err != nil {
		return value
	}

	gi := q.AsApproximateFloat64() / (1024 * 1024 * 1024)
	formatted := fmt.Sprintf("%.1fGi", gi)
	if strings.HasSuffix(formatted, ".0Gi") {
		return strings.TrimSuffix(formatted, ".0Gi") + "Gi"
	}

	return formatted
}
