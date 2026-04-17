package cmd

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var useCmdCommentPrefix string

func selectClusterByAge(clusterList []data.ClusterInfo, useOldest bool) (*data.ClusterInfo, error) {
	if len(clusterList) == 0 {
		return nil, fmt.Errorf("no clusters available for age-based selection")
	}

	selected := clusterList[0]
	selectedTime, err := time.Parse("2006-01-02 15:04:05", selected.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("unable to parse cluster creation date for %q", selected.ClusterName)
	}

	for _, cluster := range clusterList[1:] {
		clusterTime, err := time.Parse("2006-01-02 15:04:05", cluster.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("unable to parse cluster creation date for %q", cluster.ClusterName)
		}

		if useOldest {
			if clusterTime.Before(selectedTime) {
				selected = cluster
				selectedTime = clusterTime
			}
		} else {
			if clusterTime.After(selectedTime) {
				selected = cluster
				selectedTime = clusterTime
			}
		}
	}

	return &selected, nil
}

func printAmbiguousSelectionHelp(target string, matches []data.ClusterInfo) {
	fmt.Printf("multiple clusters matched (%d). no switch performed.\n\n", len(matches))

	fmt.Println("choose exactly one cluster by using one of:")
	fmt.Println("  - --oldest")
	fmt.Println("  - --newest")

	if target != "" {
		fmt.Printf("  - an exact cluster name or ARN instead of %q\n", target)
	}

	fmt.Println("")
	fmt.Println("matching clusters:")
	fmt.Println("")
	printIndentedClusters("  ", matches)
}

func printIndentedClusters(indent string, matches []data.ClusterInfo) {
	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		printutils.PrintClusters(false, matches...)
		return
	}

	os.Stdout = writer
	printutils.PrintClusters(false, matches...)
	writer.Close()
	os.Stdout = originalStdout

	content, err := io.ReadAll(reader)
	if err != nil {
		fmt.Print(string(content))
		return
	}

	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	for _, line := range lines {
		fmt.Printf("%s%s\n", indent, line)
	}
}

// tryFastSwitch attempts to switch to the target cluster using only local
// kubeconfig data and the on-disk cache, without any AWS API calls.
// Returns the matched ARN on success, or "" if the fast path cannot be used.
func tryFastSwitch(target, namespace string) string {
	if target == "" {
		return ""
	}

	arnRegex := `^arn:aws:eks:([a-z0-9-]+):(\d{12}):cluster/([a-zA-Z0-9-]+)$`
	re := regexp.MustCompile(arnRegex)

	var candidateARN string

	if re.MatchString(target) {
		// Target is an ARN — use it directly
		candidateARN = target
	} else {
		// Target is a cluster name (or substring) — look it up in the cache
		loadCacheFromDisk()
		if CachedData != nil && CachedData.ClusterByARN != nil {
			var matches []string
			for arn, info := range CachedData.ClusterByARN {
				if info.ClusterName == target || strings.Contains(info.ClusterName, target) {
					matches = append(matches, arn)
				}
			}
			if len(matches) == 1 {
				candidateARN = matches[0]
			}
			// If 0 or >1 matches, fall through to the slow path
		}
	}

	if candidateARN == "" {
		return ""
	}

	contextName, ok := k8s.FindContextForCluster(candidateARN)
	if !ok {
		return ""
	}

	if err := k8s.UseContext(contextName); err != nil {
		return ""
	}

	if namespace != "" {
		if err := k8s.SetNamespace(namespace); err != nil {
			fmt.Printf("Failed to set namespace: %s\n", err.Error())
			os.Exit(1)
		}
	}
	cachedProfile := ""
	if CachedData != nil {
		if info, ok := CachedData.ClusterByARN[candidateARN]; ok {
			cachedProfile = info.AWSProfile
		}
	}
	printSwitchSuccess(candidateARN, namespace, cachedProfile)
	return candidateARN
}

func resolveClusterForUse(target, profile, profileContains, nameContains, nameNotContains, region, version string, refresh, oldest, newest bool) (*data.ClusterInfo, []data.ClusterInfo, error) {
	if oldest && newest {
		return nil, nil, fmt.Errorf("--oldest and --newest are mutually exclusive")
	}

	arnRegex := `^arn:aws:eks:([a-z0-9-]+):(\d{12}):cluster/([a-zA-Z0-9-]+)$`
	re := regexp.MustCompile(arnRegex)

	target = strings.TrimSpace(target)

	if re.MatchString(target) {
		clusterInfo := loadClusterByArn(target)
		if clusterInfo == nil {
			return nil, nil, fmt.Errorf("cluster not found")
		}

		return clusterInfo, nil, nil
	}

	if strings.HasPrefix(target, "arn:aws:eks:") {
		return nil, nil, fmt.Errorf("invalid cluster ARN: %q", target)
	}

	clusterList, err := LoadClusterList([]string{}, profile, profileContains, nameContains, nameNotContains, region, version, refresh)
	if err != nil {
		return nil, nil, err
	}

	if target == "" {
		if len(clusterList) == 0 {
			return nil, nil, fmt.Errorf("no clusters matched the provided filters")
		}

		if len(clusterList) > 1 {
			if oldest {
				selected, selErr := selectClusterByAge(clusterList, true)
				return selected, nil, selErr
			}

			if newest {
				selected, selErr := selectClusterByAge(clusterList, false)
				return selected, nil, selErr
			}

			return nil, clusterList, fmt.Errorf("multiple clusters matched the provided filters; add --oldest or --newest, or refine filters")
		}

		return &clusterList[0], nil, nil
	}

	matches := make([]data.ClusterInfo, 0, len(clusterList))
	for _, cluster := range clusterList {
		if strings.Contains(cluster.ClusterName, target) {
			matches = append(matches, cluster)
		}
	}

	if len(matches) == 0 {
		return nil, nil, fmt.Errorf("no clusters matched %q", target)
	}

	if len(matches) > 1 {
		if oldest {
			selected, selErr := selectClusterByAge(matches, true)
			return selected, nil, selErr
		}

		if newest {
			selected, selErr := selectClusterByAge(matches, false)
			return selected, nil, selErr
		}

		return nil, matches, fmt.Errorf("multiple clusters matched %q; add --oldest or --newest, or refine the filters", target)
	}

	return &matches[0], nil, nil
}

func SwitchToCluster(clusterArn, namespace, profile string) {
	// Fast path: if no profile override, check if kubeconfig already has a
	// context for this cluster and switch to it directly, avoiding expensive
	// AWS API calls.
	if profile == "" {
		contextName, found := k8s.FindContextForCluster(clusterArn)
		if found {
			if err := k8s.UseContext(contextName); err == nil {
				if namespace != "" {
					if err := k8s.SetNamespace(namespace); err != nil {
						fmt.Printf("Failed to set namespace: %s\n", err.Error())
						os.Exit(1)
					}
				}
				cachedProfile := ""
				loadCacheFromDisk()
				if CachedData != nil {
					if info, ok := CachedData.ClusterByARN[clusterArn]; ok {
						cachedProfile = info.AWSProfile
					}
				}
				printSwitchSuccess(clusterArn, namespace, cachedProfile)
				return
			}
		}
	}

	clusterInfo := loadClusterByArn(clusterArn)

	// clusterInfo := loadClusterByArn(clusterARN)
	if clusterInfo == nil {
		fmt.Println("Cluster not found")
		os.Exit(1)
	}

	if profile != "" {
		clusterInfo.AWSProfile = profile
	}

	err := eks.UpdateKubeConfig(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, "")
	if err != nil {
		fmt.Printf("Failed to update kubeconfig: %s\n", err.Error())
		os.Exit(1)
	}

	if namespace != "" {
		err = k8s.SetNamespace(namespace)
		if err != nil {
			fmt.Printf("Failed to set namespace: %s\n", err.Error())
			os.Exit(1)
		} else {
			fmt.Printf("%sSwitched to EKS cluster %q (namespace: %q) in region %q using profile %q\n", useCmdCommentPrefix, clusterInfo.ClusterName, namespace, clusterInfo.Region, clusterInfo.AWSProfile)
		}
	} else {
		fmt.Printf("%sSwitched to EKS cluster %q in region %q using profile %q\n", useCmdCommentPrefix, clusterInfo.ClusterName, clusterInfo.Region, clusterInfo.AWSProfile)
	}
}

// printSwitchSuccess prints the context-switch confirmation message.
// It extracts cluster name and region from the ARN when clusterName/region are
// not supplied directly.
func printSwitchSuccess(clusterArn, namespace, profile string) {
	clusterName := clusterArn
	region := ""

	arnRegex := `^arn:aws:eks:([a-z0-9-]+):\d{12}:cluster/([a-zA-Z0-9-]+)$`
	if m := regexp.MustCompile(arnRegex).FindStringSubmatch(clusterArn); m != nil {
		region = m[1]
		clusterName = m[2]
	}

	switch {
	case namespace != "" && profile != "":
		fmt.Printf("%sSwitched to EKS cluster %q (namespace: %q) in region %q using profile %q\n", useCmdCommentPrefix, clusterName, namespace, region, profile)
	case namespace != "":
		fmt.Printf("%sSwitched to EKS cluster %q (namespace: %q) in region %q\n", useCmdCommentPrefix, clusterName, namespace, region)
	case profile != "":
		fmt.Printf("%sSwitched to EKS cluster %q in region %q using profile %q\n", useCmdCommentPrefix, clusterName, region, profile)
	default:
		fmt.Printf("%sSwitched to EKS cluster %q in region %q\n", useCmdCommentPrefix, clusterName, region)
	}
}

// switchClusterWithInfo switches to an EKS cluster using already-resolved
// cluster information, avoiding a redundant loadClusterByArn call.
func switchClusterWithInfo(clusterInfo *data.ClusterInfo, namespace, profile string) {
	// Fast path: context already exists in kubeconfig
	if profile == "" {
		contextName, found := k8s.FindContextForCluster(clusterInfo.Arn)
		if found {
			if err := k8s.UseContext(contextName); err == nil {
				if namespace != "" {
					if err := k8s.SetNamespace(namespace); err != nil {
						fmt.Printf("Failed to set namespace: %s\n", err.Error())
						os.Exit(1)
					}
				}
				printSwitchSuccess(clusterInfo.Arn, namespace, clusterInfo.AWSProfile)
				return
			}
		}
	}

	effectiveProfile := clusterInfo.AWSProfile
	if profile != "" {
		effectiveProfile = profile
	}

	err := eks.UpdateKubeConfig(effectiveProfile, clusterInfo.Region, clusterInfo.ClusterName, "")
	if err != nil {
		fmt.Printf("Failed to update kubeconfig: %s\n", err.Error())
		os.Exit(1)
	}

	if namespace != "" {
		err = k8s.SetNamespace(namespace)
		if err != nil {
			fmt.Printf("Failed to set namespace: %s\n", err.Error())
			os.Exit(1)
		}
	}
	printSwitchSuccess(clusterInfo.Arn, namespace, effectiveProfile)
}

var useCmd = &cobra.Command{
	Use:   "use [cluster-name-or-arn]",
	Short: "Switch kubectl context to a different EKS cluster",
	Long: `Switch kubectl context to a different EKS cluster by updating kubeconfig.

Accepts either a cluster ARN or partial cluster name. When using a partial
name, the command applies the same cluster filters as 'list' and switches only
when exactly one cluster matches by default.

When multiple clusters match, you can choose one with --oldest or --newest.

If a kubeconfig context for the target cluster already exists and the
credentials are still valid, the switch is performed locally without calling
AWS APIs, making it significantly faster. When credentials have expired or no
matching context exists, a full 'aws eks update-kubeconfig' is performed.

Optionally specify a namespace to set as default, or use a different AWS
profile for authentication.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := ""
		if len(args) == 1 {
			target = strings.TrimSpace(args[0])
		}

		namespace, err := cmd.Flags().GetString("namespace")
		if err != nil {
			namespace = ""
		}

		profile, err := cmd.Flags().GetString("profile")
		if err != nil {
			profile = ""
		}

		profileContains, err := cmd.Flags().GetString("profile-contains")
		if err != nil {
			profileContains = ""
		}

		nameContains, err := cmd.Flags().GetString("cluster-contains")
		if err != nil {
			nameContains = ""
		}

		nameNotContains, err := cmd.Flags().GetString("cluster-not-contains")
		if err != nil {
			nameNotContains = ""
		}

		region, err := cmd.Flags().GetString("region")
		if err != nil {
			region = ""
		}

		version, err := cmd.Flags().GetString("version")
		if err != nil {
			version = ""
		}

		refresh, err := cmd.Flags().GetBool("refresh")
		if err != nil {
			refresh = false
		}

		oldest, err := cmd.Flags().GetBool("oldest")
		if err != nil {
			oldest = false
		}

		newest, err := cmd.Flags().GetBool("newest")
		if err != nil {
			newest = false
		}

		comment, err := cmd.Flags().GetBool("bash-comment")
		if err != nil {
			comment = false
		}
		if comment {
			useCmdCommentPrefix = "# "
		} else {
			useCmdCommentPrefix = ""
		}

		// Fast path: try to reuse an existing kubeconfig context without
		// any AWS API calls. Works for both ARN and name-based lookups.
		if profile == "" && !refresh {
			arn := tryFastSwitch(target, namespace)
			if arn != "" {
				return
			}
		}

		clusterInfo, ambiguousMatches, err := resolveClusterForUse(target, profile, profileContains, nameContains, nameNotContains, region, version, refresh, oldest, newest)
		if err != nil {
			if len(ambiguousMatches) > 1 {
				printAmbiguousSelectionHelp(target, ambiguousMatches)
			} else {
				fmt.Println(err.Error())
			}
			os.Exit(1)
		}

		switchClusterWithInfo(clusterInfo, namespace, profile)
	},
}

func init() {
	useCmd.Flags().BoolP("refresh", "u", false, "Refresh data from AWS")
	useCmd.Flags().StringP("namespace", "n", "", "Set specific namespace for the context")
	useCmd.Flags().StringP("profile", "p", "", "Set specific AWS profile for the context")
	useCmd.Flags().StringP("profile-contains", "q", "", "AWS profile contains string")
	useCmd.Flags().StringP("cluster-contains", "c", "", "Cluster name contains string")
	useCmd.Flags().StringP("cluster-not-contains", "x", "", "Cluster name does not contain string")
	useCmd.Flags().StringP("region", "r", "", "AWS region to use")
	useCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	useCmd.Flags().Bool("oldest", false, "When multiple clusters match, switch to the oldest cluster")
	useCmd.Flags().Bool("newest", false, "When multiple clusters match, switch to the newest cluster")
	useCmd.Flags().Bool("bash-comment", false, "Prefix output with '# ' so it looks like a bash comment")

	rootCmd.AddCommand(useCmd)
}
