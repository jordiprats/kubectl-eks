package printutils

import (
	"fmt"
	"os"
	"sort"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
)

func PrintHelmReleases(noHeaders bool, results []data.HelmReleaseResult) {
	if len(results) == 0 {
		return
	}

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

	table := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "AWS PROFILE", Type: "string"},
			{Name: "AWS REGION", Type: "string"},
			{Name: "CLUSTER NAME", Type: "string"},
			{Name: "NAMESPACE", Type: "string"},
			{Name: "NAME", Type: "string"},
			{Name: "REVISION", Type: "integer"},
			{Name: "UPDATED", Type: "string"},
			{Name: "STATUS", Type: "string"},
			{Name: "CHART", Type: "string"},
			{Name: "APP VERSION", Type: "string"},
		},
	}

	for _, result := range results {
		if result.Error != "" {
			result.Status = fmt.Sprintf("ERROR: %s", result.Error)
		}
		table.Rows = append(table.Rows, metav1.TableRow{Cells: []interface{}{
			result.Profile,
			result.Region,
			result.ClusterName,
			valueOrDash(result.Namespace),
			valueOrDash(result.Name),
			result.Revision,
			valueOrDash(result.Updated),
			valueOrDash(result.Status),
			valueOrDash(result.Chart),
			valueOrDash(result.AppVersion),
		}})
	}

	printer := printers.NewTablePrinter(printers.PrintOptions{NoHeaders: noHeaders})
	if err := printer.PrintObj(table, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error printing Helm releases: %v\n", err)
	}
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
