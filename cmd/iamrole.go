package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/sts"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var lookupCallerARN = sts.TryGetCallerARN

func execAWSProfile(config clientcmdapi.Config) string {
	ctx := config.Contexts[config.CurrentContext]
	if ctx == nil {
		return ""
	}
	user := config.AuthInfos[ctx.AuthInfo]
	if user == nil || user.Exec == nil {
		return ""
	}
	for _, e := range user.Exec.Env {
		if e.Name == "AWS_PROFILE" {
			return e.Value
		}
	}
	for i, a := range user.Exec.Args {
		if a == "--profile" && i+1 < len(user.Exec.Args) {
			return user.Exec.Args[i+1]
		}
		if v, ok := strings.CutPrefix(a, "--profile="); ok {
			return v
		}
	}
	return ""
}

func currentIAMRole(config clientcmdapi.Config, cluster data.ClusterInfo) string {
	profile := execAWSProfile(config)
	if profile == "" {
		profile = cluster.AWSProfile
	}

	callerARN, err := lookupCallerARN(profile, cluster.Region)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not determine the IAM role for profile %q: %v\n", profile, err)
		return "-"
	}
	return sts.RoleFromCallerARN(callerARN)
}
