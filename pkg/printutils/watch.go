package printutils

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/jordiprats/kubectl-eks/pkg/cf"
	"github.com/jordiprats/kubectl-eks/pkg/data"
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

func PrintNodeGroupColored(multiCluster bool, wide bool, ngInfo ...eks.EKSNodeGroupInfo) {
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

	hasBootstrapMatch := false
	for _, ng := range ngInfo {
		if ng.BootstrapMatch != "" {
			hasBootstrapMatch = true
			break
		}
	}

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 4, 3, ' ', 0)

	if multiCluster {
		header := "AWS PROFILE\tAWS REGION\tCLUSTER NAME\tNAME\tCAPACITY TYPE\tRELEASE VERSION\tLAUNCH TEMPLATE\tINSTANCE TYPE\tDESIRED CAPACITY\tMAX CAPACITY\tMIN CAPACITY\tVERSION\tSTATUS"
		if hasBootstrapMatch {
			header += "\tBOOTSTRAP MATCH"
		}
		if wide {
			header += "\tBOOTSTRAP ARGUMENTS"
		}
		fmt.Fprintf(w, "%s\n", header)
	} else {
		header := "NAME\tCAPACITY TYPE\tRELEASE VERSION\tLAUNCH TEMPLATE\tINSTANCE TYPE\tDESIRED CAPACITY\tMAX CAPACITY\tMIN CAPACITY\tVERSION\tSTATUS"
		if hasBootstrapMatch {
			header += "\tBOOTSTRAP MATCH"
		}
		if wide {
			header += "\tBOOTSTRAP ARGUMENTS"
		}
		fmt.Fprintf(w, "%s\n", header)
	}

	for _, ng := range ngInfo {
		status := ng.Status
		if c := colorForStatus(status); c != "" {
			status = Colorize(status, c)
		}

		if multiCluster {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s",
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
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s",
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
		if hasBootstrapMatch {
			fmt.Fprintf(w, "\t%s", ng.BootstrapMatch)
		}
		if wide {
			fmt.Fprintf(w, "\t%s", sanitizeForColumn(ng.BootstrapArguments))
		}
		fmt.Fprintf(w, "\n")
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

func colorForNodeStatus(status string) string {
	switch strings.ToUpper(status) {
	case "READY":
		return ColorGreen
	case "NOTREADY", "NOT_READY":
		return ColorRed
	case "SCHEDULINGDISABLED":
		return ColorYellow
	default:
		return ""
	}
}

// flushBoldHeader writes buf to stdout, bolding the first line.
func flushBoldHeader(output string) {
	if IsTTY() {
		if i := strings.Index(output, "\n"); i >= 0 {
			fmt.Fprint(os.Stdout, ColorBold+output[:i]+ColorReset+output[i:])
			return
		}
	}
	fmt.Fprint(os.Stdout, output)
}

// applyLineColors replaces plain status text with colored versions line-by-line
// (skipping the header at index 0). Each entry in colors maps the plain text of
// row i to its colored replacement; only the first match per line is replaced so
// coincidental matches elsewhere in the row are left alone.
func applyLineColors(output string, colors []string) string {
	lines := strings.Split(output, "\n")
	for i, colored := range colors {
		lineIdx := i + 1 // +1 to skip the header line
		if lineIdx < len(lines) {
			lines[lineIdx] = strings.Replace(lines[lineIdx], extractPlain(colored), colored, 1)
		}
	}
	return strings.Join(lines, "\n")
}

// extractPlain strips ANSI escape sequences from s to recover the plain text.
func extractPlain(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// skip until 'm'
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			i = j + 1
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func PrintMultiClusterNodesColored(wide bool, nodes []data.ClusterNodeInfo) {
	if len(nodes) == 0 {
		return
	}

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

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 4, 3, ' ', 0)

	if wide {
		fmt.Fprintf(w, "AWS PROFILE\tAWS REGION\tCLUSTER NAME\tNAME\tSTATUS\tINSTANCE TYPE\tCOMPUTE\tMANAGED BY\tCPU USED/TOTAL (REM)\tMEMORY USED/TOTAL (REM)\tPODS\tCONDITIONS\tAGE\n")
	} else {
		fmt.Fprintf(w, "AWS PROFILE\tAWS REGION\tCLUSTER NAME\tNAME\tSTATUS\tINSTANCE TYPE\tCOMPUTE\tMANAGED BY\tAGE\n")
	}

	// STATUS is NOT the last column — write plain status to tabwriter so widths
	// are calculated correctly; collect the colored versions to inject afterwards.
	var statusColors []string
	for _, n := range nodes {
		plain := n.Node.Status
		colored := plain
		if c := colorForNodeStatus(plain); c != "" {
			colored = Colorize(plain, c)
		}
		statusColors = append(statusColors, colored)

		if wide {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
				n.Profile,
				n.Region,
				n.ClusterName,
				n.Node.Name,
				plain,
				n.Node.InstanceType,
				n.Node.Compute,
				n.Node.ManagedBy,
				formatCPUUsedTotalRemaining(n.Node.CPUUsed, n.Node.CPUCapacity, n.Node.CPUAllocatable),
				formatMemoryUsedTotalRemaining(n.Node.MemoryUsed, n.Node.MemoryCapacity, n.Node.MemoryAllocatable),
				n.Node.PodsRunning,
				formatNodeConditions(n.Node),
				formatAge(n.Node.Created),
			)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				n.Profile,
				n.Region,
				n.ClusterName,
				n.Node.Name,
				plain,
				n.Node.InstanceType,
				n.Node.Compute,
				n.Node.ManagedBy,
				formatAge(n.Node.Created),
			)
		}
	}

	w.Flush()

	output := buf.String()
	if IsTTY() {
		output = applyLineColors(output, statusColors)
	}
	flushBoldHeader(output)
}

func colorForStackStatus(status string) string {
	s := strings.ToUpper(status)
	switch {
	case strings.HasSuffix(s, "_COMPLETE") && !strings.HasPrefix(s, "DELETE") && !strings.HasPrefix(s, "ROLLBACK"):
		return ColorGreen
	case strings.HasSuffix(s, "_IN_PROGRESS"):
		return ColorYellow
	case strings.HasSuffix(s, "_FAILED"), strings.HasPrefix(s, "ROLLBACK"):
		return ColorRed
	case strings.HasPrefix(s, "DELETE"):
		return ColorMagenta
	default:
		return ""
	}
}

func PrintStacksColored(stackList []cf.StackInfo) {
	if len(stackList) == 0 {
		return
	}

	multiCluster := false
	first := stackList[0].ClusterName
	for _, s := range stackList[1:] {
		if s.ClusterName != first {
			multiCluster = true
			break
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

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 4, 3, ' ', 0)

	if multiCluster {
		fmt.Fprintf(w, "AWS PROFILE\tAWS REGION\tCLUSTER NAME\tNAME\tSTATUS\n")
	} else {
		fmt.Fprintf(w, "NAME\tSTATUS\n")
	}

	// STATUS is the last column in stacks — colorize directly.
	for _, s := range stackList {
		status := s.Status
		if c := colorForStackStatus(status); c != "" {
			status = Colorize(status, c)
		}

		if multiCluster {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Profile, s.Region, s.ClusterName, s.Name, status)
		} else {
			fmt.Fprintf(w, "%s\t%s\n", s.Name, status)
		}
	}

	w.Flush()

	flushBoldHeader(buf.String())
}
