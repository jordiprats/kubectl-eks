package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/jordiprats/kubectl-eks/pkg/sts"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check whether the current credentials are valid",
	Long: `Verify that the AWS credentials for the current kubeconfig context are
still valid by calling STS GetCallerIdentity. Exits 0 on success, 1 on failure.

With --reauth, automatically refreshes credentials when they are expired
by rebuilding the kubeconfig for the current cluster (equivalent to running
k eks use --reauth on the current context).

Useful before submitting long-running workflows to catch expired credentials
early, and in scripts as a guard (k eks check --reauth && argo submit ...).`,
	Run: func(cmd *cobra.Command, args []string) {
		clusterInfo, err := GetCurrentClusterInfo()
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: cannot resolve current cluster: %v\n", err)
			os.Exit(1)
		}

		quiet, _ := cmd.Flags().GetBool("quiet")
		reauth, _ := cmd.Flags().GetBool("reauth")

		start := time.Now()
		account, err := sts.TryGetAccountID(clusterInfo.AWSProfile, clusterInfo.Region)
		elapsed := time.Since(start)

		if err != nil && reauth {
			if !quiet {
				fmt.Fprintf(os.Stderr, "credentials expired, refreshing...\n")
			}
			forceReauth = true
			reauthErr := updateKubeConfigForCluster(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, "", clusterInfo.AWSAccountID)
			forceReauth = false
			if reauthErr != nil {
				fmt.Fprintf(os.Stderr, "FAIL: reauth failed: %v\n", reauthErr)
				os.Exit(1)
			}

			start = time.Now()
			account, err = sts.TryGetAccountID(clusterInfo.AWSProfile, clusterInfo.Region)
			elapsed = time.Since(start)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: credentials expired or invalid for profile %q (%v)\n", clusterInfo.AWSProfile, err)
			os.Exit(1)
		}

		if !quiet {
			fmt.Printf("OK: account %s, profile %q, cluster %q (%s)\n", account, clusterInfo.AWSProfile, clusterInfo.ClusterName, elapsed.Round(time.Millisecond))
		}
	},
}

func init() {
	checkCmd.Flags().BoolP("quiet", "q", false, "Suppress output on success (exit code only)")
	checkCmd.Flags().Bool("reauth", false, "Automatically refresh credentials if expired")
	rootCmd.AddCommand(checkCmd)
}
