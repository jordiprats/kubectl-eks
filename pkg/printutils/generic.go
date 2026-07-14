package printutils

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jordiprats/kubectl-eks/pkg/cf"
	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/printers"
)

func PrintMultiGetPods(noHeaders bool, podList ...k8s.K8SClusterPodList) {
	// Create a table printer
	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	// Create a Table object
	table := &v1.Table{
		ColumnDefinitions: []v1.TableColumnDefinition{
			{Name: "AWS PROFILE", Type: "string"},
			{Name: "AWS REGION", Type: "string"},
			{Name: "CLUSTER NAME", Type: "string"},
			{Name: "ARN", Type: "string"},
			{Name: "VERSION", Type: "string"},
			{Name: "NAMESPACE", Type: "string"},
			{Name: "POD NAME", Type: "string"},
			{Name: "READY", Type: "string"},
			{Name: "STATUS", Type: "string"},
			{Name: "RESTARTS", Type: "number"},
			{Name: "AGE", Type: "string"},
		},
	}

	// Flatten into a sortable structure
	type podRow struct {
		profile     string
		region      string
		clusterName string
		arn         string
		version     string
		namespace   string
		name        string
		ready       string
		status      string
		restarts    int
		age         time.Time
	}

	var rows []podRow
	for _, clusterList := range podList {
		for _, pod := range clusterList.Pods {
			rows = append(rows, podRow{
				profile:     clusterList.AWSProfile,
				region:      clusterList.Region,
				clusterName: clusterList.ClusterName,
				arn:         clusterList.Arn,
				version:     clusterList.Version,
				namespace:   pod.Namespace,
				name:        pod.Name,
				ready:       pod.Ready,
				status:      pod.Status,
				restarts:    pod.Restarts,
				age:         pod.Age.Time,
			})
		}
	}

	// Sort by Profile, Region, ClusterName, Namespace, Name
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].profile != rows[j].profile {
			return rows[i].profile < rows[j].profile
		}
		if rows[i].region != rows[j].region {
			return rows[i].region < rows[j].region
		}
		if rows[i].clusterName != rows[j].clusterName {
			return rows[i].clusterName < rows[j].clusterName
		}
		if rows[i].namespace != rows[j].namespace {
			return rows[i].namespace < rows[j].namespace
		}
		return rows[i].name < rows[j].name
	})

	// Populate table rows
	for _, r := range rows {
		humanAge := duration.ShortHumanDuration(time.Since(r.age))
		table.Rows = append(table.Rows, v1.TableRow{
			Cells: []interface{}{
				r.profile,
				r.region,
				r.clusterName,
				r.arn,
				r.version,
				r.namespace,
				r.name,
				r.ready,
				r.status,
				r.restarts,
				humanAge,
			},
		})
	}

	// Print the table
	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

// print k8s stats in a kubectl-style table format
func PrintK8SStats(noHeaders bool, statsList ...k8s.K8Sstats) {
	// Create a table printer
	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	// Create a Table object
	table := &v1.Table{
		ColumnDefinitions: []v1.TableColumnDefinition{
			{Name: "AWS PROFILE", Type: "string"},
			{Name: "AWS REGION", Type: "string"},
			{Name: "CLUSTER NAME", Type: "string"},
			{Name: "ARN", Type: "string"},
			{Name: "VERSION", Type: "string"},
			{Name: "NAMESPACES", Type: "number"},
			{Name: "POD COUNT", Type: "number"},
			{Name: "NODE COUNT", Type: "number"},
			{Name: "NODES NOT READY", Type: "number"},
			{Name: "PODS NOT RUNNING", Type: "number"},
			{Name: "PODS WITH RESTARTS", Type: "number"},
		},
	}

	// Populate rows with data from the variadic K8Sstats
	for _, stats := range statsList {
		table.Rows = append(table.Rows, v1.TableRow{
			Cells: []interface{}{
				stats.AWSProfile,
				stats.Region,
				stats.ClusterName,
				stats.Arn,
				stats.Version,
				stats.NamespaceCount,
				stats.PodCount,
				stats.NodeCount,
				stats.NodesNotReady,
				stats.PodsNotRunning,
				stats.PodsWithRestartsCount,
			},
		})
	}

	// Print the table
	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

// printResults prints results in a kubectl-style table format
func PrintClusters(noHeaders bool, clusterInfos ...data.ClusterInfo) {
	PrintClustersWithOptions(noHeaders, false, clusterInfos...)
}

// PrintClustersWithOptions prints cluster info with optional wide columns.
func PrintClustersWithOptions(noHeaders bool, wide bool, clusterInfos ...data.ClusterInfo) {
	// Sort by Profile, Region, ClusterName
	sort.Slice(clusterInfos, func(i, j int) bool {
		if clusterInfos[i].AWSProfile != clusterInfos[j].AWSProfile {
			return clusterInfos[i].AWSProfile < clusterInfos[j].AWSProfile
		}
		if clusterInfos[i].Region != clusterInfos[j].Region {
			return clusterInfos[i].Region < clusterInfos[j].Region
		}
		return clusterInfos[i].ClusterName < clusterInfos[j].ClusterName
	})

	// Create a table printer
	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	// Create a Table object
	var table *v1.Table

	if len(clusterInfos) == 1 && clusterInfos[0].Namespace != "" {
		table = &v1.Table{
			ColumnDefinitions: []v1.TableColumnDefinition{
				// {Name: "AWS ACCOUNT ID", Type: "string"},
				{Name: "AWS PROFILE", Type: "string"},
				{Name: "AWS REGION", Type: "string"},
				{Name: "CLUSTER NAME", Type: "string"},
				{Name: "NAMESPACE", Type: "string"},
				{Name: "STATUS", Type: "string"},
				{Name: "VERSION", Type: "string"},
				{Name: "CREATED", Type: "string"},
				{Name: "ARN", Type: "string"},
			},
		}
		if wide {
			table.ColumnDefinitions = append(table.ColumnDefinitions,
				v1.TableColumnDefinition{Name: "NODE HEALTH", Type: "string"},
				v1.TableColumnDefinition{Name: "CPU USED/TOTAL (REM)", Type: "string"},
				v1.TableColumnDefinition{Name: "MEMORY USED/TOTAL (REM)", Type: "string"},
			)
		}
	} else {
		table = &v1.Table{
			ColumnDefinitions: []v1.TableColumnDefinition{
				// {Name: "AWS ACCOUNT ID", Type: "string"},
				{Name: "AWS PROFILE", Type: "string"},
				{Name: "AWS REGION", Type: "string"},
				{Name: "CLUSTER NAME", Type: "string"},
				{Name: "STATUS", Type: "string"},
				{Name: "VERSION", Type: "string"},
				{Name: "CREATED", Type: "string"},
				{Name: "ARN", Type: "string"},
			},
		}
		if wide {
			table.ColumnDefinitions = append(table.ColumnDefinitions,
				v1.TableColumnDefinition{Name: "NODE HEALTH", Type: "string"},
				v1.TableColumnDefinition{Name: "CPU USED/TOTAL (REM)", Type: "string"},
				v1.TableColumnDefinition{Name: "MEMORY USED/TOTAL (REM)", Type: "string"},
			)
		}
	}

	// Populate rows with data from the variadic ClusterInfo
	for _, clusterInfo := range clusterInfos {
		if len(clusterInfos) == 1 && clusterInfo.Namespace != "" {
			cells := []interface{}{
				// clusterInfo.AWSAccountID,
				clusterInfo.AWSProfile,
				clusterInfo.Region,
				clusterInfo.ClusterName,
				clusterInfo.Namespace,
				clusterInfo.Status,
				clusterInfo.Version,
				clusterInfo.CreatedAt,
				clusterInfo.Arn,
			}
			if wide {
				cells = append(cells,
					formatClusterNodeHealth(clusterInfo.NodeCount, clusterInfo.NodeReady, clusterInfo.NodeNotReady, clusterInfo.NodeSchedDisabled),
					formatClusterCPUUsedTotalRemaining(clusterInfo.CPUUsedTotal, clusterInfo.CPUCapacityTotal, clusterInfo.CPUAllocatableTotal),
					formatClusterMemoryUsedTotalRemaining(clusterInfo.MemoryUsedTotal, clusterInfo.MemoryCapacityTotal, clusterInfo.MemoryAllocatableTotal),
				)
			}
			table.Rows = append(table.Rows, v1.TableRow{Cells: cells})
		} else {
			cells := []interface{}{
				// clusterInfo.AWSAccountID,
				clusterInfo.AWSProfile,
				clusterInfo.Region,
				clusterInfo.ClusterName,
				clusterInfo.Status,
				clusterInfo.Version,
				clusterInfo.CreatedAt,
				clusterInfo.Arn,
			}
			if wide {
				cells = append(cells,
					formatClusterNodeHealth(clusterInfo.NodeCount, clusterInfo.NodeReady, clusterInfo.NodeNotReady, clusterInfo.NodeSchedDisabled),
					formatClusterCPUUsedTotalRemaining(clusterInfo.CPUUsedTotal, clusterInfo.CPUCapacityTotal, clusterInfo.CPUAllocatableTotal),
					formatClusterMemoryUsedTotalRemaining(clusterInfo.MemoryUsedTotal, clusterInfo.MemoryCapacityTotal, clusterInfo.MemoryAllocatableTotal),
				)
			}
			table.Rows = append(table.Rows, v1.TableRow{Cells: cells})
		}
	}

	// Print the table
	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

func formatClusterCPUUsedTotalRemaining(used, total, remaining string) string {
	return fmt.Sprintf("%s/%s (%s)", formatClusterCPUQuantityCores(used), formatClusterCPUQuantityCores(total), formatClusterCPUQuantityCores(remaining))
}

func formatClusterNodeHealth(total, ready, notReady, schedDisabled int) string {
	if total == 0 {
		return "-"
	}

	return fmt.Sprintf("%d/%d Ready (NR:%d SD:%d)", ready, total, notReady, schedDisabled)
}

func formatClusterCPUQuantityCores(value string) string {
	if value == "" || value == "-" {
		return "-"
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

func formatClusterMemoryUsedTotalRemaining(used, total, remaining string) string {
	return fmt.Sprintf("%s/%s (%s)", formatClusterMemoryQuantityGi(used), formatClusterMemoryQuantityGi(total), formatClusterMemoryQuantityGi(remaining))
}

func formatClusterMemoryQuantityGi(value string) string {
	if value == "" || value == "-" {
		return "-"
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

// PrintJsonPathResults prints the results in a kubectl-style table format
func PrintJsonPathResults(noHeaders bool, results []data.JsonPathResult) {
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
		return results[i].Resource < results[j].Resource
	})

	// Create a table printer
	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	// Create a Table object
	table := &v1.Table{
		ColumnDefinitions: []v1.TableColumnDefinition{
			{Name: "PROFILE", Type: "string"},
			{Name: "REGION", Type: "string"},
			{Name: "CLUSTER", Type: "string"},
			{Name: "NAMESPACE", Type: "string"},
			{Name: "NAME", Type: "string"},
			{Name: "VALUE", Type: "string"},
		},
	}

	// Populate rows with data
	for _, result := range results {
		value := result.Value
		if result.Error != "" {
			value = "ERROR: " + result.Error
		}

		table.Rows = append(table.Rows, v1.TableRow{
			Cells: []interface{}{
				result.Profile,
				result.Region,
				result.ClusterName,
				result.Namespace,
				result.Resource,
				value,
			},
		})
	}

	// Print the table
	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

func PrintInsights(noHeaders bool, insights ...data.EKSInsightInfo) {
	multiCluster := false
	if len(insights) > 0 {
		first := insights[0].ClusterName
		for _, ins := range insights[1:] {
			if ins.ClusterName != first {
				multiCluster = true
				break
			}
		}
	}

	sort.Slice(insights, func(i, j int) bool {
		if insights[i].Profile != insights[j].Profile {
			return insights[i].Profile < insights[j].Profile
		}
		if insights[i].Region != insights[j].Region {
			return insights[i].Region < insights[j].Region
		}
		if insights[i].ClusterName != insights[j].ClusterName {
			return insights[i].ClusterName < insights[j].ClusterName
		}
		return insights[i].ID < insights[j].ID
	})

	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	columns := []v1.TableColumnDefinition{}
	if multiCluster {
		columns = append(columns,
			v1.TableColumnDefinition{Name: "AWS PROFILE", Type: "string"},
			v1.TableColumnDefinition{Name: "AWS REGION", Type: "string"},
			v1.TableColumnDefinition{Name: "CLUSTER NAME", Type: "string"},
		)
	}
	columns = append(columns,
		v1.TableColumnDefinition{Name: "ID", Type: "string"},
		v1.TableColumnDefinition{Name: "CATEGORY", Type: "string"},
		v1.TableColumnDefinition{Name: "STATUS", Type: "string"},
		v1.TableColumnDefinition{Name: "REASON", Type: "string"},
	)

	table := &v1.Table{ColumnDefinitions: columns}

	for _, eachInsight := range insights {
		cells := []interface{}{}
		if multiCluster {
			cells = append(cells, eachInsight.Profile, eachInsight.Region, eachInsight.ClusterName)
		}
		cells = append(cells,
			eachInsight.ID,
			eachInsight.Category,
			eachInsight.Status,
			eachInsight.Reason,
		)
		table.Rows = append(table.Rows, v1.TableRow{Cells: cells})
	}

	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

func PrintStacks(noHeaders bool, stackList ...cf.StackInfo) {
	multiCluster := false
	if len(stackList) > 0 {
		first := stackList[0].ClusterName
		for _, s := range stackList[1:] {
			if s.ClusterName != first {
				multiCluster = true
				break
			}
		}
	}

	sort.Slice(stackList, func(i, j int) bool {
		if stackList[i].Profile != stackList[j].Profile {
			return stackList[i].Profile < stackList[j].Profile
		}
		if stackList[i].Region != stackList[j].Region {
			return stackList[i].Region < stackList[j].Region
		}
		if stackList[i].ClusterName != stackList[j].ClusterName {
			return stackList[i].ClusterName < stackList[j].ClusterName
		}
		return stackList[i].Name < stackList[j].Name
	})

	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	columns := []v1.TableColumnDefinition{}
	if multiCluster {
		columns = append(columns,
			v1.TableColumnDefinition{Name: "AWS PROFILE", Type: "string"},
			v1.TableColumnDefinition{Name: "AWS REGION", Type: "string"},
			v1.TableColumnDefinition{Name: "CLUSTER NAME", Type: "string"},
		)
	}
	columns = append(columns,
		v1.TableColumnDefinition{Name: "NAME", Type: "string"},
		v1.TableColumnDefinition{Name: "STATUS", Type: "string"},
	)

	table := &v1.Table{ColumnDefinitions: columns}

	for _, stackInfo := range stackList {
		cells := []interface{}{}
		if multiCluster {
			cells = append(cells, stackInfo.Profile, stackInfo.Region, stackInfo.ClusterName)
		}
		cells = append(cells, stackInfo.Name, stackInfo.Status)
		table.Rows = append(table.Rows, v1.TableRow{Cells: cells})
	}

	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

func PrintUpdates(noHeaders bool, updateList ...eks.EKSUpdateInfo) {
	multiCluster := false
	if len(updateList) > 0 {
		first := updateList[0].ClusterName
		for _, u := range updateList[1:] {
			if u.ClusterName != first {
				multiCluster = true
				break
			}
		}
	}

	sort.Slice(updateList, func(i, j int) bool {
		if updateList[i].Profile != updateList[j].Profile {
			return updateList[i].Profile < updateList[j].Profile
		}
		if updateList[i].Region != updateList[j].Region {
			return updateList[i].Region < updateList[j].Region
		}
		if updateList[i].ClusterName != updateList[j].ClusterName {
			return updateList[i].ClusterName < updateList[j].ClusterName
		}
		return updateList[i].Type < updateList[j].Type
	})

	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	columns := []v1.TableColumnDefinition{}
	if multiCluster {
		columns = append(columns,
			v1.TableColumnDefinition{Name: "AWS PROFILE", Type: "string"},
			v1.TableColumnDefinition{Name: "AWS REGION", Type: "string"},
			v1.TableColumnDefinition{Name: "CLUSTER NAME", Type: "string"},
		)
	}
	columns = append(columns,
		v1.TableColumnDefinition{Name: "TYPE", Type: "string"},
		v1.TableColumnDefinition{Name: "STATUS", Type: "string"},
		v1.TableColumnDefinition{Name: "ERRORS", Type: "string"},
	)

	table := &v1.Table{ColumnDefinitions: columns}

	for _, eachUpdate := range updateList {
		cells := []interface{}{}
		if multiCluster {
			cells = append(cells, eachUpdate.Profile, eachUpdate.Region, eachUpdate.ClusterName)
		}
		cells = append(cells,
			eachUpdate.Type,
			eachUpdate.Status,
			strings.Join(eachUpdate.Errors, ", "),
		)
		table.Rows = append(table.Rows, v1.TableRow{Cells: cells})
	}

	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

// printResults prints results in a kubectl-style table format
func PrintAMIs(noHeaders bool, amiInfos ...data.AMIInfo) {
	// Sort the clusterInfos by ClusterName (you can customize the field for sorting)
	sort.Slice(amiInfos, func(i, j int) bool {
		return amiInfos[i].Name < amiInfos[j].Name
	})

	// Create a table printer
	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	// Create a Table object
	table := &v1.Table{
		ColumnDefinitions: []v1.TableColumnDefinition{
			{Name: "NAME", Type: "string"},
			{Name: "ARCHITECTURE", Type: "string"},
			{Name: "STATE", Type: "string"},
			{Name: "DEPRECATION TIME", Type: "string"},
		},
	}

	// Populate rows with data from the variadic ClusterInfo
	for _, amiInfo := range amiInfos {
		table.Rows = append(table.Rows, v1.TableRow{
			Cells: []interface{}{
				amiInfo.Name,
				amiInfo.Architecture,
				amiInfo.State,
				amiInfo.DeprecationTime,
			},
		})
	}

	// Print the table
	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

func PrintNodeGroup(noHeaders bool, ngInfo ...eks.EKSNodeGroupInfo) {
	// Determine if multiple clusters are present
	multiCluster := false
	if len(ngInfo) > 0 {
		firstKey := ngInfo[0].Profile + "|" + ngInfo[0].Region + "|" + ngInfo[0].ClusterName
		for _, ng := range ngInfo[1:] {
			if ng.Profile+"|"+ng.Region+"|"+ng.ClusterName != firstKey {
				multiCluster = true
				break
			}
		}
	}

	// Sort by Profile, Region, ClusterName, Name
	sort.Slice(ngInfo, func(i, j int) bool {
		if ngInfo[i].Profile != ngInfo[j].Profile {
			return ngInfo[i].Profile < ngInfo[j].Profile
		}
		if ngInfo[i].Region != ngInfo[j].Region {
			return ngInfo[i].Region < ngInfo[j].Region
		}
		if ngInfo[i].ClusterName != ngInfo[j].ClusterName {
			return ngInfo[i].ClusterName < ngInfo[j].ClusterName
		}
		return ngInfo[i].Name < ngInfo[j].Name
	})

	// Create a table printer
	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	// Create a Table object
	columns := []v1.TableColumnDefinition{}
	if multiCluster {
		columns = append(columns,
			v1.TableColumnDefinition{Name: "AWS PROFILE", Type: "string"},
			v1.TableColumnDefinition{Name: "AWS REGION", Type: "string"},
			v1.TableColumnDefinition{Name: "CLUSTER NAME", Type: "string"},
		)
	}
	columns = append(columns,
		v1.TableColumnDefinition{Name: "NAME", Type: "string"},
		v1.TableColumnDefinition{Name: "CAPACITY TYPE", Type: "string"},
		v1.TableColumnDefinition{Name: "RELEASE VERSION", Type: "string"},
		v1.TableColumnDefinition{Name: "LAUNCH TEMPLATE", Type: "string"},
		v1.TableColumnDefinition{Name: "INSTANCE TYPE", Type: "string"},
		v1.TableColumnDefinition{Name: "DESIRED CAPACITY", Type: "string"},
		v1.TableColumnDefinition{Name: "MAX CAPACITY", Type: "string"},
		v1.TableColumnDefinition{Name: "MIN CAPACITY", Type: "string"},
		v1.TableColumnDefinition{Name: "VERSION", Type: "string"},
		v1.TableColumnDefinition{Name: "STATUS", Type: "string"},
	)

	table := &v1.Table{ColumnDefinitions: columns}

	// Populate rows
	for _, eachNG := range ngInfo {
		cells := []interface{}{}
		if multiCluster {
			cells = append(cells, eachNG.Profile, eachNG.Region, eachNG.ClusterName)
		}
		cells = append(cells,
			eachNG.Name,
			eachNG.CapacityType,
			eachNG.ReleaseVersion,
			eachNG.LaunchTemplate,
			eachNG.InstanceType,
			eachNG.DesiredCapacity,
			eachNG.MaxCapacity,
			eachNG.MinCapacity,
			eachNG.Version,
			eachNG.Status,
		)
		table.Rows = append(table.Rows, v1.TableRow{Cells: cells})
	}

	// Print the table
	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}
