package awsconfig

import (
	"strings"
	"testing"
)

func TestLoadAWSConfigFromReader_GlobalHints(t *testing.T) {
	config := `
# kubectl-eks-kubeconfig-profile=okta
# kubectl-eks-okta-factor=token:hardware:yubico
# kubectl-eks-okta-app-url=https://example.okta.com/home/amazon_aws/0oaEXAMPLEAPPID/272
# kubectl-eks-role-preference=PowerUser,Ops,ReadOnly

[profile dev]
# kubectl-eks-regions=us-east-1,us-west-2
region = us-east-1
`
	cfg := loadAWSConfigFromReader(strings.NewReader(config))

	if cfg.KubeconfigProfile != "okta" {
		t.Errorf("expected global KubeconfigProfile=okta, got %q", cfg.KubeconfigProfile)
	}
	if cfg.OktaFactor != "token:hardware:yubico" {
		t.Errorf("expected global OktaFactor=token:hardware:yubico, got %q", cfg.OktaFactor)
	}
	if cfg.AWSApp != "https://example.okta.com/home/amazon_aws/0oaEXAMPLEAPPID/272" {
		t.Errorf("expected global AWSApp URL, got %q", cfg.AWSApp)
	}
	if len(cfg.RolePreference) != 3 || cfg.RolePreference[0] != "PowerUser" || cfg.RolePreference[1] != "Ops" || cfg.RolePreference[2] != "ReadOnly" {
		t.Errorf("expected global RolePreference=[PowerUser Ops ReadOnly], got %v", cfg.RolePreference)
	}

	dev, ok := cfg.Profiles["dev"]
	if !ok {
		t.Fatal("expected 'dev' profile to exist")
	}
	if dev.DefaultRegion != "us-east-1" {
		t.Errorf("expected dev region=us-east-1, got %q", dev.DefaultRegion)
	}
	if len(dev.HintEKSRegions) != 2 || dev.HintEKSRegions[0] != "us-east-1" || dev.HintEKSRegions[1] != "us-west-2" {
		t.Errorf("expected dev HintEKSRegions=[us-east-1 us-west-2], got %v", dev.HintEKSRegions)
	}
}

func TestLoadAWSConfigFromReader_PerProfileHints(t *testing.T) {
	config := `
# kubectl-eks-kubeconfig-profile=global-okta
# kubectl-eks-okta-factor=global-factor

[profile dev]
# kubectl-eks-regions=us-east-1
# kubectl-eks-kubeconfig-profile=profile-okta
# kubectl-eks-okta-factor=profile-factor
region = us-east-1
`
	cfg := loadAWSConfigFromReader(strings.NewReader(config))

	if cfg.KubeconfigProfile != "global-okta" {
		t.Errorf("expected global KubeconfigProfile=global-okta, got %q", cfg.KubeconfigProfile)
	}
	if cfg.OktaFactor != "global-factor" {
		t.Errorf("expected global OktaFactor=global-factor, got %q", cfg.OktaFactor)
	}

	dev := cfg.Profiles["dev"]
	if dev.KubeconfigProfile != "profile-okta" {
		t.Errorf("expected dev KubeconfigProfile=profile-okta, got %q", dev.KubeconfigProfile)
	}
	if dev.OktaFactor != "profile-factor" {
		t.Errorf("expected dev OktaFactor=profile-factor, got %q", dev.OktaFactor)
	}
}

func TestLoadAWSConfigFromReader_CredentialProcess(t *testing.T) {
	config := `
[profile okta]
credential_process=aws-okta-processor authenticate --user alice --organization example.okta.com
region = us-east-1
`
	cfg := loadAWSConfigFromReader(strings.NewReader(config))

	okta, ok := cfg.Profiles["okta"]
	if !ok {
		t.Fatal("expected 'okta' profile to exist")
	}
	expected := "aws-okta-processor authenticate --user alice --organization example.okta.com"
	if okta.CredentialProcess != expected {
		t.Errorf("expected credential_process=%q, got %q", expected, okta.CredentialProcess)
	}
	if okta.DefaultRegion != "us-east-1" {
		t.Errorf("expected region=us-east-1, got %q", okta.DefaultRegion)
	}
}

func TestLoadAWSConfigFromReader_CredentialProcessWithEquals(t *testing.T) {
	config := `
[profile okta]
credential_process=aws-okta-processor authenticate --user=alice --organization=example.okta.com
`
	cfg := loadAWSConfigFromReader(strings.NewReader(config))

	okta := cfg.Profiles["okta"]
	expected := "aws-okta-processor authenticate --user=alice --organization=example.okta.com"
	if okta.CredentialProcess != expected {
		t.Errorf("expected credential_process=%q, got %q", expected, okta.CredentialProcess)
	}
}

func TestGetEffectiveKubeconfigProfile(t *testing.T) {
	config := `
# kubectl-eks-kubeconfig-profile=global-okta

[profile with-override]
# kubectl-eks-kubeconfig-profile=override-okta
# kubectl-eks-regions=us-east-1
region = us-east-1

[profile no-override]
# kubectl-eks-regions=us-west-2
region = us-west-2
`
	ConfigData = loadAWSConfigFromReader(strings.NewReader(config))

	if got := GetEffectiveKubeconfigProfile("with-override"); got != "override-okta" {
		t.Errorf("expected per-profile override 'override-okta', got %q", got)
	}
	if got := GetEffectiveKubeconfigProfile("no-override"); got != "global-okta" {
		t.Errorf("expected global fallback 'global-okta', got %q", got)
	}
	if got := GetEffectiveKubeconfigProfile("nonexistent"); got != "global-okta" {
		t.Errorf("expected global fallback 'global-okta', got %q", got)
	}
}

func TestGetOktaEnhancements(t *testing.T) {
	config := `
# kubectl-eks-okta-factor=global-factor
# kubectl-eks-okta-app-url=global-app

[profile okta-custom]
# kubectl-eks-okta-factor=profile-factor
credential_process=aws-okta-processor authenticate --user alice

[profile okta-default]
credential_process=aws-okta-processor authenticate --user other
`
	ConfigData = loadAWSConfigFromReader(strings.NewReader(config))

	cp, factor, app := GetOktaEnhancements("okta-custom")
	if cp != "aws-okta-processor authenticate --user alice" {
		t.Errorf("unexpected credential_process: %q", cp)
	}
	if factor != "profile-factor" {
		t.Errorf("expected profile-level factor 'profile-factor', got %q", factor)
	}
	if app != "global-app" {
		t.Errorf("expected global fallback app 'global-app', got %q", app)
	}

	cp, factor, app = GetOktaEnhancements("okta-default")
	if cp != "aws-okta-processor authenticate --user other" {
		t.Errorf("unexpected credential_process: %q", cp)
	}
	if factor != "global-factor" {
		t.Errorf("expected global factor 'global-factor', got %q", factor)
	}
	if app != "global-app" {
		t.Errorf("expected global app 'global-app', got %q", app)
	}

	cp, factor, app = GetOktaEnhancements("nonexistent")
	if cp != "" || factor != "" || app != "" {
		t.Errorf("expected empty results for nonexistent profile, got cp=%q factor=%q app=%q", cp, factor, app)
	}
}
