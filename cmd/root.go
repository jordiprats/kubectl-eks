package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/jordiprats/kubectl-eks/pkg/awsconfig"
	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/jordiprats/kubectl-eks/pkg/sts"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
)

var KubernetesConfigFlags *genericclioptions.ConfigFlags
var verbose bool

var HomeDir string
var CachedData *data.KubeCtlEksCache = nil

func loadCacheFromDisk() {
	// Load configuration from file
	configFile := HomeDir + "/.kube/.kubectl-eks-cache"
	configData, err := os.ReadFile(configFile)
	if err == nil {
		// load json data into ConfigData
		CachedData = &data.KubeCtlEksCache{}
		err = json.Unmarshal(configData, CachedData)
		if err != nil {
			fmt.Println("Error loading configuration file")
			os.Exit(1)
		}
	}
}

func saveCacheToDisk() {
	// Save configuration to file
	configFile := HomeDir + "/.kube/.kubectl-eks-cache"
	configData, err := json.Marshal(CachedData)
	if err != nil {
		fmt.Println("Error saving configuration file")
		os.Exit(1)
	}
	err = os.WriteFile(configFile, configData, 0644)
	if err != nil {
		fmt.Println("Error saving configuration file")
		os.Exit(1)
	}
}

func loadClusterByArn(clusterArn string) *data.ClusterInfo {

	clusterInfo := data.ClusterInfo{}

	// check if it is an ARN
	arnRegex := `^arn:aws:eks:([a-z0-9-]+):(\d{12}):cluster/([a-zA-Z0-9-]+)$`
	re := regexp.MustCompile(arnRegex)

	matches := re.FindStringSubmatch(clusterArn)
	if matches == nil {
		return nil
	}

	// Check on-disk cache first
	loadCacheFromDisk()
	if CachedData != nil {
		if cached, exists := CachedData.ClusterByARN[clusterArn]; exists {
			if cached.Arn == clusterArn && cached.AWSProfile != "" && cached.AWSProfile != "-" {
				return &cached
			}
		}
	}

	// search for an AWS profile that matches the account ID, region and cluster name
	awsProfiles := awsconfig.GetAWSProfilesWithEKSHints()
	foundAwsProfile := ""
	for _, profileDetails := range awsProfiles {
		accountID, err := sts.GetAccountID(profileDetails.Name, matches[1])
		if err != nil {
			continue
		}

		if accountID != matches[2] {
			continue
		}

		for _, hintRegion := range profileDetails.HintEKSRegions {
			if hintRegion != matches[1] {
				continue
			}

			// aws eks list-clusters --region <region> --profile <profile>
			clusters, err := eks.GetClusters(profileDetails.Name, hintRegion)
			if err != nil {
				continue
			}

			for _, cluster := range clusters {
				if cluster != nil {
					if *cluster == matches[3] {
						foundAwsProfile = profileDetails.Name
					}
				}
			}
		}
	}

	if foundAwsProfile == "" {
		foundAwsProfile = "-"
	}

	// create clusterInfo
	clusterInfo = data.ClusterInfo{ClusterName: matches[3], Region: matches[1], AWSProfile: foundAwsProfile, AWSAccountID: matches[2]}

	clusterDesc, err := eks.DescribeCluster(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName)
	if err != nil || clusterDesc == nil {
		fmt.Fprintf(os.Stderr, "Error describing cluster %s: %v\n", clusterInfo.ClusterName, err.Error())
	} else {
		clusterInfo.Status = string(clusterDesc.Status)
		clusterInfo.Version = *clusterDesc.Version
		clusterInfo.Arn = *clusterDesc.Arn
		clusterInfo.CreatedAt = clusterDesc.CreatedAt.Format("2006-01-02 15:04:05")
	}

	if CachedData == nil {
		CachedData = &data.KubeCtlEksCache{}
	}

	if CachedData.ClusterByARN == nil {
		CachedData.ClusterByARN = make(map[string]data.ClusterInfo)
	}

	// save update loaded configuration
	CachedData.ClusterByARN[clusterArn] = clusterInfo
	saveCacheToDisk()

	return &clusterInfo
}

func GetCurrentClusterInfo() (data.ClusterInfo, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := loadingRules.Load()
	if err != nil {
		return data.ClusterInfo{}, err
	}

	context := config.Contexts[config.CurrentContext]
	if context == nil {
		return data.ClusterInfo{}, fmt.Errorf("no current context")
	}

	clusterArn := context.Cluster

	loadCacheFromDisk()
	if CachedData == nil {
		CachedData = &data.KubeCtlEksCache{
			ClusterByARN: make(map[string]data.ClusterInfo),
			ClusterList:  make(map[string]map[string][]data.ClusterInfo),
		}
	}

	clusterInfo, exists := CachedData.ClusterByARN[clusterArn]

	// Refresh if not in cache, ARN mismatch, or missing AWSProfile
	if !exists || clusterInfo.Arn != clusterArn || clusterInfo.AWSProfile == "" {
		foundClusterInfo := loadClusterByArn(clusterArn)
		if foundClusterInfo == nil {
			return data.ClusterInfo{ClusterName: clusterArn}, nil
		}
		clusterInfo = *foundClusterInfo
	}

	return clusterInfo, nil
}

var rootCmd = &cobra.Command{
	Use:   "kubectl-eks",
	Short: "A kubectl plugin for managing Amazon EKS clusters",
	Long: `kubectl-eks provides convenient commands for listing, inspecting, 
and switching between EKS clusters and their associated resources.

Prerequisites:
- AWS CLI installed and configured
- kubectl installed and configured`,
	Example: `  # Show current cluster info
  kubectl eks
  
  # List all clusters
  kubectl eks list
  
  # Switch to a cluster
  kubectl eks use my-cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		refresh, err := cmd.Flags().GetBool("refresh")
		if err != nil {
			refresh = false
		}

		region, err := cmd.Flags().GetString("region")
		if err != nil {
			region = ""
		}

		output, _ := cmd.Flags().GetString("output")
		if output != "" && output != "wide" {
			fmt.Fprintf(os.Stderr, "invalid output format %q: supported format is wide\n", output)
			os.Exit(1)
		}

		// Load Kubernetes configuration
		config, err := KubernetesConfigFlags.ToRawKubeConfigLoader().RawConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading kubeconfig: %v\n", err.Error())
			os.Exit(1)
		}

		// Get current context
		currentContext := config.CurrentContext
		// fmt.Printf("Current context: %s\n", currentContext)

		// Retrieve cluster information
		contextDetails, exists := config.Contexts[currentContext]
		if !exists {
			fmt.Fprintf(os.Stderr, "Context '%s' not found in kubeconfig\n", currentContext)
			os.Exit(1)
		}

		clusterArn := contextDetails.Cluster

		currentNamespace, err := k8s.GetCurrentNamespace()
		if err != nil {
			currentNamespace = ""
		}

		if region != "" {
			// arn:aws:eks:us-east-1:123456789123:cluster/demo

			arnRegex := `^arn:aws:eks:([a-z0-9-]+):(\d{12}):cluster/([a-zA-Z0-9-]+)$`
			re := regexp.MustCompile(arnRegex)
			matches := re.FindStringSubmatch(clusterArn)
			if matches == nil {
				fmt.Printf("Invalid cluster ARN: %q\n", clusterArn)
				os.Exit(1)
			}

			newClusterArn := fmt.Sprintf("arn:aws:eks:%s:%s:cluster/%s", region, matches[2], matches[3])

			SwitchToCluster(newClusterArn, currentNamespace, "")

		} else {
			loadCacheFromDisk()
			if CachedData == nil {
				CachedData = &data.KubeCtlEksCache{
					ClusterByARN: make(map[string]data.ClusterInfo),
					ClusterList:  make(map[string]map[string][]data.ClusterInfo),
				}
			}

			clusterInfo, exists := CachedData.ClusterByARN[clusterArn]
			if !exists || refresh {
				foundClusterInfo := loadClusterByArn(clusterArn)

				if foundClusterInfo == nil {
					fmt.Println("Current cluster is not an EKS cluster")
					os.Exit(1)
				} else {
					clusterInfo = *foundClusterInfo
				}
			}

			// validate cached data, if invalid, refresh
			if clusterInfo.Arn != clusterArn {
				CachedData = &data.KubeCtlEksCache{
					ClusterByARN: make(map[string]data.ClusterInfo),
					ClusterList:  make(map[string]map[string][]data.ClusterInfo),
				}
				foundClusterInfo := loadClusterByArn(clusterArn)
				if foundClusterInfo == nil {
					fmt.Println("Current cluster is not an EKS cluster")
					os.Exit(1)
				} else {
					clusterInfo = *foundClusterInfo
				}
			}

			if currentNamespace == "" {
				clusterInfo.Namespace = "default"
			} else {
				clusterInfo.Namespace = currentNamespace
			}

			noHeaders, err := cmd.Flags().GetBool("no-headers")
			if err != nil {
				noHeaders = false
			}

			if clusterInfo.Arn != clusterArn {
				fmt.Printf("%s\n", clusterArn)
			} else if output == "wide" {
				printutils.PrintCurrentClusterWide(noHeaders, currentIAMRole(config, clusterInfo), clusterInfo)
			} else {
				printutils.PrintClusters(noHeaders, clusterInfo)
			}

			// save data to configuration
			saveCacheToDisk()

			os.Exit(0)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringP("region", "r", "", "Switch to the same cluster in a different region")
	rootCmd.Flags().BoolP("refresh", "u", false, "Do not use cached data, refresh from AWS")
	rootCmd.Flags().StringP("output", "o", "", "Output format. Supported: wide (adds the IAM role in use)")
	rootCmd.PersistentFlags().Bool("no-headers", false, "When using the default or custom-column output format, don't print headers (default print headers)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Show verbose discovery warnings and diagnostics")

	KubernetesConfigFlags = genericclioptions.NewConfigFlags(true)
	KubernetesConfigFlags.AddFlags(rootCmd.PersistentFlags())
}
