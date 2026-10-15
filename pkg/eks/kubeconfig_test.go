package eks

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestBuildEnhancedCredentialProcess(t *testing.T) {
	base := "aws-okta-processor authenticate --user alice"

	tests := []struct {
		name       string
		factor     string
		app        string
		noAWSCache bool
		want       string
	}{
		{"nothing", "", "", false, base},
		{"factor only", "token:hardware:yubico", "", false, base + " --factor=token:hardware:yubico"},
		{"app only", "", "https://x/y", false, base + " --application=https://x/y"},
		{"factor and app", "f", "a", false, base + " --factor=f --application=a"},
		{"no aws cache", "f", "a", true, base + " --factor=f --application=a --no-aws-cache"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildEnhancedCredentialProcess(base, tt.factor, tt.app, tt.noAWSCache)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildTempAWSConfig(t *testing.T) {
	got := buildTempAWSConfig("okta", "cmd --flag=1")
	want := "[profile okta]\ncredential_process=cmd --flag=1\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildUpdateKubeconfigArgs(t *testing.T) {
	got := buildUpdateKubeconfigArgs("okta", "us-west-2", "my-cluster", "")
	want := []string{"eks", "update-kubeconfig", "--name", "my-cluster", "--region", "us-west-2", "--profile", "okta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	got = buildUpdateKubeconfigArgs("okta", "us-west-2", "my-cluster", "/tmp/kc")
	want = append(want, "--kubeconfig", "/tmp/kc")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUpdateKubeConfigWithEnhancedCredentials(t *testing.T) {
	var captured *exec.Cmd
	var tmpConfigPath, tmpConfigContent string

	orig := runAWSCommand
	defer func() { runAWSCommand = orig }()
	runAWSCommand = func(cmd *exec.Cmd) error {
		captured = cmd
		for _, e := range cmd.Env {
			if v, ok := strings.CutPrefix(e, "AWS_CONFIG_FILE="); ok {
				tmpConfigPath = v
			}
		}
		b, err := os.ReadFile(tmpConfigPath)
		if err != nil {
			t.Errorf("temp config not readable during run: %v", err)
		}
		tmpConfigContent = string(b)
		return nil
	}

	err := UpdateKubeConfigWithEnhancedCredentials("okta", "aws-okta-processor authenticate --user alice", "yubico", "https://app", "us-east-1", "c1", "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantConfig := "[profile okta]\ncredential_process=aws-okta-processor authenticate --user alice --factor=yubico --application=https://app --no-aws-cache\n"
	if tmpConfigContent != wantConfig {
		t.Errorf("temp config = %q, want %q", tmpConfigContent, wantConfig)
	}

	wantArgs := []string{"aws", "eks", "update-kubeconfig", "--name", "c1", "--region", "us-east-1", "--profile", "okta"}
	if !reflect.DeepEqual(captured.Args, wantArgs) {
		t.Errorf("args = %v, want %v", captured.Args, wantArgs)
	}

	if _, err := os.Stat(tmpConfigPath); !os.IsNotExist(err) {
		t.Errorf("temp config %s should be removed after run, stat err: %v", tmpConfigPath, err)
	}
}

func TestUpdateKubeConfigWithEnhancedCredentials_PropagatesError(t *testing.T) {
	orig := runAWSCommand
	defer func() { runAWSCommand = orig }()
	wantErr := errors.New("boom")
	runAWSCommand = func(cmd *exec.Cmd) error { return wantErr }

	err := UpdateKubeConfigWithEnhancedCredentials("okta", "cmd", "f", "", "us-east-1", "c1", "/tmp/kc", false)
	if !errors.Is(err, wantErr) {
		t.Errorf("got %v, want %v", err, wantErr)
	}
}
