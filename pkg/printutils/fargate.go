package printutils

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/eks"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
)

func PrintFargateProfiles(noHeaders bool, profiles ...eks.FargateProfileInfo) {
	multiCluster := false
	if len(profiles) > 0 {
		first := profiles[0].ClusterName
		for _, p := range profiles[1:] {
			if p.ClusterName != first {
				multiCluster = true
				break
			}
		}
	}

	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Profile != profiles[j].Profile {
			return profiles[i].Profile < profiles[j].Profile
		}
		if profiles[i].Region != profiles[j].Region {
			return profiles[i].Region < profiles[j].Region
		}
		if profiles[i].ClusterName != profiles[j].ClusterName {
			return profiles[i].ClusterName < profiles[j].ClusterName
		}
		return profiles[i].Name < profiles[j].Name
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
		v1.TableColumnDefinition{Name: "NAMESPACE", Type: "string"},
		v1.TableColumnDefinition{Name: "SELECTOR", Type: "string"},
		v1.TableColumnDefinition{Name: "SUBNETS", Type: "number"},
	)

	table := &v1.Table{ColumnDefinitions: columns}

	for _, p := range profiles {
		for _, sel := range p.Selectors {
			cells := []interface{}{}
			if multiCluster {
				cells = append(cells, p.Profile, p.Region, p.ClusterName)
			}
			cells = append(cells,
				p.Name,
				p.Status,
				sel.Namespace,
				formatLabels(sel.Labels),
				len(p.Subnets),
			)
			table.Rows = append(table.Rows, v1.TableRow{Cells: cells})
		}
	}

	err := printer.PrintObj(table, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error printing table: %v\n", err)
		os.Exit(1)
	}
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "<none>"
	}

	pairs := make([]string, 0, len(labels))
	for k, v := range labels {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}
