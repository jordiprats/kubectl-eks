package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/awsconfig"
	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/okta"
	"github.com/jordiprats/kubectl-eks/pkg/sts"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func LoadClusterList(args []string, profile, profile_contains, profile_not_contains, name_contains, name_not_contains, region, version, notVersion string, refresh ...bool) ([]data.ClusterInfo, error) {
	return loadClusterList(args, profile, profile_contains, profile_not_contains, name_contains, name_not_contains, region, version, notVersion, false, refresh...)
}

func LoadAllClusterList(args []string, profile, profile_contains, profile_not_contains, name_contains, name_not_contains, region, version, notVersion string, refresh ...bool) ([]data.ClusterInfo, error) {
	return loadClusterList(args, profile, profile_contains, profile_not_contains, name_contains, name_not_contains, region, version, notVersion, true, refresh...)
}

func matchesVersion(clusterVersion, version, notVersion string) bool {
	return (version == "" || clusterVersion == version) && (notVersion == "" || clusterVersion != notVersion)
}

func loadClusterList(args []string, profile, profile_contains, profile_not_contains, name_contains, name_not_contains, region, version, notVersion string, allClusters bool, refresh ...bool) ([]data.ClusterInfo, error) {
	clusterList := []data.ClusterInfo{}

	doRefresh := len(refresh) > 0 && refresh[0]

	// if filters are empty, use current cluster
	if !allClusters && profile == "" && profile_contains == "" && profile_not_contains == "" && name_contains == "" && name_not_contains == "" && region == "" && version == "" && notVersion == "" {
		clusterArn := ""

		// Load Kubernetes configuration
		config, err := KubernetesConfigFlags.ToRawKubeConfigLoader().RawConfig()
		if err != nil {
			return nil, fmt.Errorf("error loading kubeconfig: %v", err.Error())
		}

		// Get current context
		currentContext := config.CurrentContext
		// fmt.Printf("Current context: %s\n", currentContext)

		// Retrieve cluster information
		contextDetails, exists := config.Contexts[currentContext]
		if !exists {
			return nil, fmt.Errorf("context '%s' not found in kubeconfig", currentContext)
		}

		clusterArn = contextDetails.Cluster

		// check if it is an ARN
		arnRegex := `^arn:aws:eks:([a-z0-9-]+):(\d{12}):cluster/([a-zA-Z0-9-]+)$`
		re := regexp.MustCompile(arnRegex)

		matches := re.FindStringSubmatch(clusterArn)
		if matches == nil {
			if len(args) != 1 {
				return nil, fmt.Errorf("current cluster is not an EKS cluster")
			} else {
				return nil, fmt.Errorf("invalid cluster ARN: %q", clusterArn)
			}
		}

		clusterInfo := loadClusterByArn(clusterArn)

		// clusterInfo := loadClusterByArn(clusterARN)
		if clusterInfo == nil {
			return nil, fmt.Errorf("cluster not found")
		}

		clusterList = append(clusterList, *clusterInfo)

	} else {
		loadCacheFromDisk()
		if CachedData == nil {
			CachedData = &data.KubeCtlEksCache{
				ClusterByARN: make(map[string]data.ClusterInfo),
				ClusterList:  make(map[string]map[string][]data.ClusterInfo),
			}
		}

		if doRefresh {
			CachedData.ClusterList = make(map[string]map[string][]data.ClusterInfo)
		}

		// Keep default output quiet; only show per-profile load failures in verbose mode.
		showLoadFailure := verbose

		awsProfiles := awsconfig.GetAWSProfilesWithEKSHints()
		for _, profileDetails := range awsProfiles {
			if profile != "" && profile != profileDetails.Name {
				continue
			}
			if profile_contains != "" && !strings.Contains(profileDetails.Name, profile_contains) {
				continue
			}
			if profile_not_contains != "" && strings.Contains(profileDetails.Name, profile_not_contains) {
				continue
			}
			for _, hintRegion := range profileDetails.HintEKSRegions {
				if region != "" && region != hintRegion {
					continue
				}

				cachedRegions, exists := CachedData.ClusterList[profileDetails.Name]
				if !exists {
					loadClusters(profileDetails.Name, hintRegion)
				} else {
					_, exists := cachedRegions[hintRegion]
					if !exists {
						loadClusters(profileDetails.Name, hintRegion)
					}
				}

				currentClusterList, exists := CachedData.ClusterList[profileDetails.Name][hintRegion]
				if !exists {
					if showLoadFailure {
						fmt.Fprintf(os.Stderr, "Unable to load clusters using profile: %s region: %s (LoadClusterList)\n", profileDetails.Name, hintRegion)
					}
				} else {
					if version == "" && notVersion == "" && name_contains == "" && name_not_contains == "" {
						clusterList = append(clusterList, currentClusterList...)
					} else {
						for _, cluster := range currentClusterList {
							// checking filter criteria
							shouldAdd := true

							// Check version filter
							if !matchesVersion(cluster.Version, version, notVersion) {
								shouldAdd = false
							}

							// Check name_contains filter
							if name_contains != "" && !strings.Contains(cluster.ClusterName, name_contains) {
								shouldAdd = false
							}

							// Check name_not_contains filter
							if name_not_contains != "" && strings.Contains(cluster.ClusterName, name_not_contains) {
								shouldAdd = false
							}

							// only add the cluster if it meets the criteria
							if shouldAdd {
								clusterList = append(clusterList, cluster)
							}
						}
					}
				}

			}
		}
	}

	return clusterList, nil
}

var (
	lookupCachedAccountID   = sts.TryGetAccountID
	resolveOktaRole         = okta.ResolveRole
	runUpdateKubeConfig     = eks.UpdateKubeConfig
	runUpdateKubeConfigOkta = eks.UpdateKubeConfigWithEnhancedCredentials
	setExecEnv              = k8s.SetExecEnv
)

func updateKubeConfigForCluster(discoveryProfile, region, clusterName, kubeConfig, targetAccountID string) error {
	kubeconfigProfile := awsconfig.GetEffectiveKubeconfigProfile(discoveryProfile)
	if kubeconfigProfile != "" {
		credentialProcess, factor, app := awsconfig.GetOktaEnhancements(discoveryProfile, kubeconfigProfile)
		if credentialProcess != "" && (factor != "" || app != "") {
			isoKey := eks.IsolationKey(targetAccountID, kubeconfigProfile)

			opts := eks.EnhancedOptions{
				CredentialProcess: credentialProcess,
				Factor:            factor,
				Application:       app,
				NoAWSCache:        forceReauth,
				Key:               isoKey,
			}

			needsLogin := forceReauth
			if !forceReauth && targetAccountID != "" {
				cachedAccount, err := lookupCachedAccountID(kubeconfigProfile, region)
				if err != nil {
					needsLogin = true
				} else if cachedAccount != targetAccountID {
					needsLogin = true
					opts.NoAWSCache = true
				}
			}

			if needsLogin && targetAccountID != "" {
				choice, err := resolveOktaRole(credentialProcess, factor, app, targetAccountID, awsconfig.GetRolePreference(discoveryProfile, kubeconfigProfile))
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not pre-select the Okta role (%v); falling back to interactive selection\n", err)
				} else {
					opts.Role = choice.RoleARN
					opts.AccountAlias = choice.AccountAlias
				}
			}

			if err := runUpdateKubeConfigOkta(kubeconfigProfile, region, clusterName, kubeConfig, opts); err != nil {
				return err
			}

			if isoKey != "" && kubeConfig == "" {
				clusterARN := fmt.Sprintf("arn:aws:eks:%s:%s:cluster/%s", region, targetAccountID, clusterName)
				if err := setExecEnv(clusterARN, map[string]string{"AWS_OKTA_KEY": isoKey}); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not set isolation key in kubeconfig exec env: %v\n", err)
				}
			}
			return nil
		}
		return runUpdateKubeConfig(kubeconfigProfile, region, clusterName, kubeConfig)
	}
	return runUpdateKubeConfig(discoveryProfile, region, clusterName, kubeConfig)
}

// GetRestConfigForCluster creates a temporary kubeconfig for the given cluster
// and returns a rest.Config without modifying the user's actual kubeconfig.
func GetRestConfigForCluster(clusterInfo data.ClusterInfo) (*rest.Config, error) {
	tmpFile, err := os.CreateTemp("", "kubectl-eks-*.kubeconfig")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp kubeconfig: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	err = updateKubeConfigForCluster(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, tmpPath, clusterInfo.AWSAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to update temp kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from temp kubeconfig: %v", err)
	}

	return restConfig, nil
}
