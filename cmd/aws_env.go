package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/awsconfig"
	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/okta"
	"github.com/spf13/cobra"
)

type credentialProcessOutput struct {
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
}

func runCredentialProcess(credentialProcess string) (*credentialProcessOutput, error) {
	args, err := okta.SplitCommand(credentialProcess)
	if err != nil {
		return nil, fmt.Errorf("failed to parse credential_process: %w", err)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("empty credential_process")
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("credential_process failed: %w", err)
	}

	start := bytes.IndexByte(out, '{')
	if start < 0 {
		return nil, fmt.Errorf("no JSON found in credential_process output")
	}

	var creds credentialProcessOutput
	if err := json.Unmarshal(out[start:], &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credential_process output: %w", err)
	}
	return &creds, nil
}

var awsEnvCmd = &cobra.Command{
	Use:   "aws-env [cluster-name-or-arn]",
	Short: "Print AWS environment variables for a cluster",
	Long: `Print export statements that configure the AWS CLI for the current cluster
(or a specified cluster).

When the cluster's AWS profile uses an Okta credential_process, the command
authenticates via Okta and exports temporary credentials (AWS_ACCESS_KEY_ID,
AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN). For non-Okta profiles it exports
AWS_PROFILE instead.

AWS_DEFAULT_REGION is always set to the cluster's region.

When no arguments or filters are provided, uses the current kubeconfig context.

You can specify a cluster by partial name, ARN, or use filters to narrow down
the cluster list. When multiple clusters match, use --oldest or --newest to
pick one.

Usage:
  eval $(kubectl eks aws-env)`,
	Example: `  # Configure AWS CLI for the current cluster
  eval $(kubectl eks aws-env)

  # Configure AWS CLI for a cluster by partial name
  eval $(kubectl eks aws-env my-cluster)

  # Configure AWS CLI filtering by cluster name substring
  eval $(kubectl eks aws-env --cluster-contains dev)

  # Configure AWS CLI filtering by region
  eval $(kubectl eks aws-env --region us-east-1)

  # Configure AWS CLI filtering by AWS profile name substring
  eval $(kubectl eks aws-env --profile-contains prod)

  # When multiple clusters match, pick the oldest or newest
  eval $(kubectl eks aws-env --cluster-contains dev --oldest)`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := ""
		if len(args) == 1 {
			target = strings.TrimSpace(args[0])
		}

		profileContains, _ := cmd.Flags().GetString("profile-contains")
		profileNotContains, _ := cmd.Flags().GetString("profile-not-contains")
		nameContains, _ := cmd.Flags().GetString("cluster-contains")
		nameNotContains, _ := cmd.Flags().GetString("cluster-not-contains")
		region, _ := cmd.Flags().GetString("region")
		version, _ := cmd.Flags().GetString("version")
		notVersion, _ := cmd.Flags().GetString("not-version")
		refresh, _ := cmd.Flags().GetBool("refresh")
		oldest, _ := cmd.Flags().GetBool("oldest")
		newest, _ := cmd.Flags().GetBool("newest")

		hasFilters := profileContains != "" || profileNotContains != "" || nameContains != "" || nameNotContains != "" || region != "" || version != "" || notVersion != "" || refresh || oldest || newest

		var clusterInfo data.ClusterInfo

		if hasFilters || target != "" {
			resolved, ambiguousMatches, err := resolveClusterForUse(target, "", profileContains, profileNotContains, nameContains, nameNotContains, region, version, notVersion, refresh, oldest, newest)
			if err != nil {
				if len(ambiguousMatches) > 1 {
					printAmbiguousSelectionHelp(target, ambiguousMatches)
				} else {
					fmt.Fprintln(os.Stderr, err.Error())
				}
				os.Exit(1)
			}
			clusterInfo = *resolved
		} else {
			config, err := KubernetesConfigFlags.ToRawKubeConfigLoader().RawConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading kubeconfig: %v\n", err.Error())
				os.Exit(1)
			}

			currentContext := config.CurrentContext
			contextDetails, exists := config.Contexts[currentContext]
			if !exists {
				fmt.Fprintf(os.Stderr, "Context '%s' not found in kubeconfig\n", currentContext)
				os.Exit(1)
			}

			clusterArn := contextDetails.Cluster

			loadCacheFromDisk()
			if CachedData == nil {
				CachedData = &data.KubeCtlEksCache{
					ClusterByARN: make(map[string]data.ClusterInfo),
					ClusterList:  make(map[string]map[string][]data.ClusterInfo),
				}
			}

			cached, found := CachedData.ClusterByARN[clusterArn]
			if !found || cached.Arn != clusterArn {
				CachedData = &data.KubeCtlEksCache{
					ClusterByARN: make(map[string]data.ClusterInfo),
					ClusterList:  make(map[string]map[string][]data.ClusterInfo),
				}
				foundClusterInfo := loadClusterByArn(clusterArn)
				if foundClusterInfo == nil {
					fmt.Fprintln(os.Stderr, "Current cluster is not an EKS cluster")
					os.Exit(1)
				}
				cached = *foundClusterInfo
			}
			clusterInfo = cached
		}

		discoveryProfile := clusterInfo.AWSProfile
		kubeconfigProfile := awsconfig.GetEffectiveKubeconfigProfile(discoveryProfile)
		credentialProcess, factor, app := awsconfig.GetOktaEnhancements(discoveryProfile, kubeconfigProfile)

		if credentialProcess != "" && (factor != "" || app != "") {
			accountID := resolveAccountID(&clusterInfo)
			isoKey := eks.IsolationKey(accountID, kubeconfigProfile)

			opts := eks.EnhancedOptions{
				CredentialProcess: credentialProcess,
				Factor:            factor,
				Application:       app,
				Key:               isoKey,
			}

			if accountID != "" {
				choice, err := resolveOktaRole(credentialProcess, factor, app, accountID, awsconfig.GetRolePreference(discoveryProfile, kubeconfigProfile))
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not pre-select the Okta role (%v); falling back to interactive selection\n", err)
				} else {
					opts.Role = choice.RoleARN
					opts.AccountAlias = choice.AccountAlias
				}
			}

			enhanced := eks.EnhancedCredentialProcess(opts)
			creds, err := runCredentialProcess(enhanced)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error obtaining credentials: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("export AWS_ACCESS_KEY_ID=%s\n", creds.AccessKeyID)
			fmt.Printf("export AWS_SECRET_ACCESS_KEY=%s\n", creds.SecretAccessKey)
			fmt.Printf("export AWS_SESSION_TOKEN=%s\n", creds.SessionToken)
		} else {
			fmt.Printf("export AWS_PROFILE=%s\n", clusterInfo.AWSProfile)
		}

		fmt.Printf("export AWS_DEFAULT_REGION=%s\n", clusterInfo.Region)
	},
}

func init() {
	awsEnvCmd.Flags().BoolP("refresh", "u", false, "Refresh data from AWS")
	awsEnvCmd.Flags().StringP("profile-contains", "q", "", "Filter by AWS profile name (account) substring")
	awsEnvCmd.Flags().StringP("profile-not-contains", "Q", "", "Exclude profiles whose name contains this substring")
	awsEnvCmd.Flags().StringP("cluster-contains", "c", "", "Filter by cluster name substring")
	awsEnvCmd.Flags().StringP("cluster-not-contains", "x", "", "Exclude clusters whose name contains this substring")
	awsEnvCmd.Flags().StringP("region", "r", "", "Filter by AWS region")
	awsEnvCmd.Flags().StringP("version", "v", "", "Filter by EKS version")
	awsEnvCmd.Flags().StringP("not-version", "V", "", "Exclude clusters with this EKS version")
	awsEnvCmd.Flags().Bool("oldest", false, "When multiple clusters match, use the oldest cluster")
	awsEnvCmd.Flags().Bool("newest", false, "When multiple clusters match, use the newest cluster")

	rootCmd.AddCommand(awsEnvCmd)
}
