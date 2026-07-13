package printutils

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/jordiprats/kubectl-eks/pkg/eks"
)

func colorForStatus(status string) string {
	switch strings.ToUpper(status) {
	case "ACTIVE":
		return ColorGreen
	case "CREATING", "UPDATING":
		return ColorYellow
	case "DELETING":
		return ColorMagenta
	case "CREATE_FAILED", "DELETE_FAILED", "DEGRADED":
		return ColorRed
	default:
		return ""
	}
}

func PrintNodeGroupColored(multiCluster bool, ngInfo ...eks.EKSNodeGroupInfo) {
	if multiCluster {
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
	} else {
		sort.Slice(ngInfo, func(i, j int) bool {
			return ngInfo[i].Name < ngInfo[j].Name
		})
	}

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 4, 3, ' ', 0)

	if multiCluster {
		fmt.Fprintf(w, "AWS PROFILE\tAWS REGION\tCLUSTER NAME\tNAME\tCAPACITY TYPE\tRELEASE VERSION\tLAUNCH TEMPLATE\tINSTANCE TYPE\tDESIRED CAPACITY\tMAX CAPACITY\tMIN CAPACITY\tVERSION\tSTATUS\n")
	} else {
		fmt.Fprintf(w, "NAME\tCAPACITY TYPE\tRELEASE VERSION\tLAUNCH TEMPLATE\tINSTANCE TYPE\tDESIRED CAPACITY\tMAX CAPACITY\tMIN CAPACITY\tVERSION\tSTATUS\n")
	}

	for _, ng := range ngInfo {
		status := ng.Status
		if c := colorForStatus(status); c != "" {
			status = Colorize(status, c)
		}

		if multiCluster {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s\n",
				ng.Profile,
				ng.Region,
				ng.ClusterName,
				ng.Name,
				ng.CapacityType,
				ng.ReleaseVersion,
				ng.LaunchTemplate,
				ng.InstanceType,
				ng.DesiredCapacity,
				ng.MaxCapacity,
				ng.MinCapacity,
				ng.Version,
				status,
			)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s\n",
				ng.Name,
				ng.CapacityType,
				ng.ReleaseVersion,
				ng.LaunchTemplate,
				ng.InstanceType,
				ng.DesiredCapacity,
				ng.MaxCapacity,
				ng.MinCapacity,
				ng.Version,
				status,
			)
		}
	}

	w.Flush()

	output := buf.String()
	if IsTTY() {
		if i := strings.Index(output, "\n"); i >= 0 {
			fmt.Fprint(os.Stdout, ColorBold+output[:i]+ColorReset+output[i:])
			return
		}
	}
	fmt.Fprint(os.Stdout, output)
}
