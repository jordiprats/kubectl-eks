package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/awsconfig"
	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func LoadClusterList(args []string, profile, profile_contains, name_contains, name_not_contains, region, version string, refresh ...bool) ([]data.ClusterInfo, error) {
	clusterList := []data.ClusterInfo{}

	doRefresh := len(refresh) > 0 && refresh[0]

	// if filters are empty, use current cluster
	if profile == "" && profile_contains == "" && name_contains == "" && name_not_contains == "" && region == "" && version == "" {
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
					if version == "" && name_contains == "" && name_not_contains == "" {
						clusterList = append(clusterList, currentClusterList...)
					} else {
						for _, cluster := range currentClusterList {
							// checking filter criteria
							shouldAdd := true

							// Check version filter
							if version != "" && cluster.Version != version {
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

	err = eks.UpdateKubeConfig(clusterInfo.AWSProfile, clusterInfo.Region, clusterInfo.ClusterName, tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to update temp kubeconfig for cluster %s: %v", clusterInfo.ClusterName, err)
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from temp kubeconfig: %v", err)
	}

	return restConfig, nil
}
