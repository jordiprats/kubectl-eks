package printutils

import (
	"fmt"
	"os"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
)

// PrintHealthDetails prints only the detailed health check results (no summary)
func PrintHealthDetails(noHeaders bool, results []data.HealthCheckResult, summaries []data.ClusterHealthSummary) {
	if len(results) == 0 {
		PrintHealthSummary(noHeaders, summaries)
		return
	}

	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	table := &v1.Table{
		ColumnDefinitions: []v1.TableColumnDefinition{
			{Name: "AWS PROFILE", Type: "string"},
			{Name: "AWS REGION", Type: "string"},
			{Name: "CLUSTER NAME", Type: "string"},
			{Name: "KIND", Type: "string"},
			{Name: "NAMESPACE", Type: "string"},
			{Name: "NAME", Type: "string"},
			{Name: "READY", Type: "string"},
			{Name: "STATUS", Type: "string"},
			{Name: "MESSAGE", Type: "string"},
		},
	}

	for _, r := range results {
		namespace := r.Namespace
		if namespace == "" {
			namespace = "-"
		}

		table.Rows = append(table.Rows, v1.TableRow{
			Cells: []interface{}{
				r.Profile,
				r.Region,
				r.ClusterName,
				r.Kind,
				namespace,
				r.Name,
				r.Ready,
				r.Status,
				r.Message,
			},
		})
	}

	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

// PrintHealthSummary prints only the cluster health summary (no details)
func PrintHealthSummary(noHeaders bool, summaries []data.ClusterHealthSummary) {
	if len(summaries) == 0 {
		return
	}

	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})

	table := &v1.Table{
		ColumnDefinitions: []v1.TableColumnDefinition{
			{Name: "AWS PROFILE", Type: "string"},
			{Name: "AWS REGION", Type: "string"},
			{Name: "CLUSTER NAME", Type: "string"},
			{Name: "PODS", Type: "string"},
			{Name: "DEPLOYMENTS", Type: "string"},
			{Name: "STATEFULSETS", Type: "string"},
			{Name: "DAEMONSETS", Type: "string"},
			{Name: "REPLICASETS", Type: "string"},
			{Name: "NODES", Type: "string"},
			{Name: "STATUS", Type: "string"},
		},
	}

	for _, s := range summaries {
		table.Rows = append(table.Rows, v1.TableRow{
			Cells: []interface{}{
				s.Profile,
				s.Region,
				s.ClusterName,
				fmt.Sprintf("%d/%d", s.HealthyPods, s.TotalPods),
				fmt.Sprintf("%d/%d", s.HealthyDeployments, s.TotalDeployments),
				fmt.Sprintf("%d/%d", s.HealthyStatefulSets, s.TotalStatefulSets),
				fmt.Sprintf("%d/%d", s.HealthyDaemonSets, s.TotalDaemonSets),
				fmt.Sprintf("%d/%d", s.HealthyReplicaSets, s.TotalReplicaSets),
				fmt.Sprintf("%d/%d", s.HealthyNodes, s.TotalNodes),
				s.OverallStatus,
			},
		})
	}

	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}
