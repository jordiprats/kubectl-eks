package cmd

import (
	"errors"
	"testing"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func kubeconfigWithExec(exec *clientcmdapi.ExecConfig) clientcmdapi.Config {
	return clientcmdapi.Config{
		CurrentContext: "ctx",
		Contexts:       map[string]*clientcmdapi.Context{"ctx": {AuthInfo: "user", Cluster: "cluster"}},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{"user": {Exec: exec}},
	}
}

func TestExecAWSProfile(t *testing.T) {
	tests := []struct {
		name string
		cfg  clientcmdapi.Config
		want string
	}{
		{"env AWS_PROFILE", kubeconfigWithExec(&clientcmdapi.ExecConfig{
			Env: []clientcmdapi.ExecEnvVar{{Name: "OTHER", Value: "x"}, {Name: "AWS_PROFILE", Value: "okta"}},
		}), "okta"},
		{"--profile arg", kubeconfigWithExec(&clientcmdapi.ExecConfig{
			Args: []string{"eks", "get-token", "--profile", "from-arg"},
		}), "from-arg"},
		{"--profile=value arg", kubeconfigWithExec(&clientcmdapi.ExecConfig{
			Args: []string{"eks", "--profile=inline"},
		}), "inline"},
		{"env wins over args", kubeconfigWithExec(&clientcmdapi.ExecConfig{
			Env:  []clientcmdapi.ExecEnvVar{{Name: "AWS_PROFILE", Value: "env"}},
			Args: []string{"--profile", "arg"},
		}), "env"},
		{"no exec", kubeconfigWithExec(nil), ""},
		{"exec without profile", kubeconfigWithExec(&clientcmdapi.ExecConfig{Args: []string{"eks", "get-token"}}), ""},
		{"no current context", clientcmdapi.Config{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := execAWSProfile(tt.cfg); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCurrentIAMRole(t *testing.T) {
	orig := lookupCallerARN
	defer func() { lookupCallerARN = orig }()

	var gotProfile, gotRegion string
	lookupCallerARN = func(profile, region string) (string, error) {
		gotProfile, gotRegion = profile, region
		return "arn:aws:sts::111111111111:assumed-role/Team-A-PowerUser/alice", nil
	}
	cluster := data.ClusterInfo{AWSProfile: "discovery", Region: "us-west-2"}

	cfg := kubeconfigWithExec(&clientcmdapi.ExecConfig{Env: []clientcmdapi.ExecEnvVar{{Name: "AWS_PROFILE", Value: "okta"}}})
	if got := currentIAMRole(cfg, cluster); got != "Team-A-PowerUser" {
		t.Errorf("role = %q", got)
	}
	if gotProfile != "okta" || gotRegion != "us-west-2" {
		t.Errorf("should query with the kubeconfig's profile, got profile=%q region=%q", gotProfile, gotRegion)
	}

	if got := currentIAMRole(kubeconfigWithExec(nil), cluster); got != "Team-A-PowerUser" || gotProfile != "discovery" {
		t.Errorf("should fall back to the discovery profile, got role=%q profile=%q", got, gotProfile)
	}

	lookupCallerARN = func(string, string) (string, error) { return "", errors.New("expired") }
	if got := currentIAMRole(cfg, cluster); got != "-" {
		t.Errorf("lookup failure should show '-', got %q", got)
	}
}

func TestRootCmdHasOutputFlag(t *testing.T) {
	f := rootCmd.Flags().Lookup("output")
	if f == nil || f.Shorthand != "o" {
		t.Fatalf("root command should define -o/--output, got %+v", f)
	}
}
