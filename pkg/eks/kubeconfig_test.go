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
		name string
		opts EnhancedOptions
		want string
	}{
		{"nothing", EnhancedOptions{}, ""},
		{"factor only", EnhancedOptions{Factor: "token:hardware:yubico"}, " --factor=token:hardware:yubico"},
		{"app only", EnhancedOptions{Application: "https://x/y"}, " --application=https://x/y"},
		{"factor and app", EnhancedOptions{Factor: "f", Application: "a"}, " --factor=f --application=a"},
		{"role", EnhancedOptions{Factor: "f", Application: "a", Role: "arn:aws:iam::1:role/R"}, " --factor=f --application=a --role=arn:aws:iam::1:role/R"},
		{"account alias", EnhancedOptions{Factor: "f", AccountAlias: "team-a-*"}, " --factor=f --account-alias=team-a-*"},
		{"everything", EnhancedOptions{Factor: "f", Application: "a", Role: "r", AccountAlias: "al", NoAWSCache: true}, " --factor=f --application=a --role=r --account-alias=al --no-aws-cache"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.CredentialProcess = base
			got := buildEnhancedCredentialProcess(tt.opts)
			if got != base+tt.want {
				t.Errorf("got %q, want %q", got, base+tt.want)
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

	err := UpdateKubeConfigWithEnhancedCredentials("okta", "us-east-1", "c1", "", EnhancedOptions{
		CredentialProcess: "aws-okta-processor authenticate --user alice",
		Factor:            "yubico",
		Application:       "https://app",
		Role:              "arn:aws:iam::1:role/R",
		NoAWSCache:        true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantConfig := "[profile okta]\ncredential_process=aws-okta-processor authenticate --user alice --factor=yubico --application=https://app --role=arn:aws:iam::1:role/R --no-aws-cache\n"
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

	err := UpdateKubeConfigWithEnhancedCredentials("okta", "us-east-1", "c1", "/tmp/kc", EnhancedOptions{CredentialProcess: "cmd", Factor: "f"})
	if !errors.Is(err, wantErr) {
		t.Errorf("got %v, want %v", err, wantErr)
	}
}
