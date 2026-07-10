package printutils

import (
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

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 3, ' ', 0)

	if multiCluster {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			Colorize("AWS PROFILE", ColorBold),
			Colorize("AWS REGION", ColorBold),
			Colorize("CLUSTER NAME", ColorBold),
			Colorize("NAME", ColorBold),
			Colorize("CAPACITY TYPE", ColorBold),
			Colorize("RELEASE VERSION", ColorBold),
			Colorize("LAUNCH TEMPLATE", ColorBold),
			Colorize("INSTANCE TYPE", ColorBold),
			Colorize("DESIRED CAPACITY", ColorBold),
			Colorize("MAX CAPACITY", ColorBold),
			Colorize("MIN CAPACITY", ColorBold),
			Colorize("VERSION", ColorBold),
			Colorize("STATUS", ColorBold),
		)
	} else {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			Colorize("NAME", ColorBold),
			Colorize("CAPACITY TYPE", ColorBold),
			Colorize("RELEASE VERSION", ColorBold),
			Colorize("LAUNCH TEMPLATE", ColorBold),
			Colorize("INSTANCE TYPE", ColorBold),
			Colorize("DESIRED CAPACITY", ColorBold),
			Colorize("MAX CAPACITY", ColorBold),
			Colorize("MIN CAPACITY", ColorBold),
			Colorize("VERSION", ColorBold),
			Colorize("STATUS", ColorBold),
		)
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
}
