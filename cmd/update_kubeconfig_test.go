package cmd

import (
	"errors"
	"testing"

	"github.com/jordiprats/kubectl-eks/pkg/awsconfig"
	"github.com/jordiprats/kubectl-eks/pkg/data"
)

type kubeconfigCalls struct {
	plain      []string
	okta       []string
	noAWSCache bool
	lookups    int
}

func setupKubeconfigFakes(t *testing.T, cfg *data.AWSConfig, cachedAccount string, lookupErr error) *kubeconfigCalls {
	t.Helper()
	calls := &kubeconfigCalls{}

	origCfg, origLookup, origPlain, origOkta := awsconfig.ConfigData, lookupCachedAccountID, runUpdateKubeConfig, runUpdateKubeConfigOkta
	t.Cleanup(func() {
		awsconfig.ConfigData, lookupCachedAccountID, runUpdateKubeConfig, runUpdateKubeConfigOkta = origCfg, origLookup, origPlain, origOkta
	})

	awsconfig.ConfigData = cfg
	lookupCachedAccountID = func(profile, region string) (string, error) {
		calls.lookups++
		return cachedAccount, lookupErr
	}
	runUpdateKubeConfig = func(profile, region, cluster, kubeConfig string) error {
		calls.plain = []string{profile, region, cluster, kubeConfig}
		return nil
	}
	runUpdateKubeConfigOkta = func(profile, cp, factor, app, region, cluster, kubeConfig string, noAWSCache bool) error {
		calls.okta = []string{profile, cp, factor, app, region, cluster, kubeConfig}
		calls.noAWSCache = noAWSCache
		return nil
	}
	return calls
}

func oktaConfig() *data.AWSConfig {
	return &data.AWSConfig{
		KubeconfigProfile: "okta",
		OktaFactor:        "yubico",
		Profiles: map[string]data.AWSProfile{
			"":    {},
			"dev": {Name: "dev"},
			"okta": {
				Name:              "okta",
				CredentialProcess: "aws-okta-processor authenticate --user alice",
			},
		},
	}
}

func TestUpdateKubeConfigForCluster_NoOverride(t *testing.T) {
	cfg := &data.AWSConfig{Profiles: map[string]data.AWSProfile{"dev": {Name: "dev"}}}
	calls := setupKubeconfigFakes(t, cfg, "", nil)

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.okta != nil {
		t.Errorf("enhanced path should not run, got %v", calls.okta)
	}
	if calls.lookups != 0 {
		t.Errorf("no STS lookup expected, got %d", calls.lookups)
	}
	want := []string{"dev", "us-east-1", "c1", ""}
	if len(calls.plain) != 4 || calls.plain[0] != want[0] || calls.plain[2] != want[2] {
		t.Errorf("plain call = %v, want %v", calls.plain, want)
	}
}

func TestUpdateKubeConfigForCluster_OverrideWithoutEnhancements(t *testing.T) {
	cfg := &data.AWSConfig{
		KubeconfigProfile: "okta",
		Profiles: map[string]data.AWSProfile{
			"dev":  {Name: "dev"},
			"okta": {Name: "okta", CredentialProcess: "aws-okta-processor authenticate"},
		},
	}
	calls := setupKubeconfigFakes(t, cfg, "", nil)

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.okta != nil {
		t.Errorf("enhanced path should not run without factor/app")
	}
	if len(calls.plain) == 0 || calls.plain[0] != "okta" {
		t.Errorf("expected plain call with override profile okta, got %v", calls.plain)
	}
}

func TestUpdateKubeConfigForCluster_OverrideWithoutCredentialProcess(t *testing.T) {
	cfg := oktaConfig()
	cfg.Profiles["okta"] = data.AWSProfile{Name: "okta"}
	calls := setupKubeconfigFakes(t, cfg, "", nil)

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.okta != nil {
		t.Errorf("enhanced path should not run without credential_process")
	}
	if len(calls.plain) == 0 || calls.plain[0] != "okta" {
		t.Errorf("expected plain call with okta, got %v", calls.plain)
	}
}

func TestUpdateKubeConfigForCluster_SameAccountReusesCache(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "111111111111", nil)

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "/tmp/kc", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.okta == nil {
		t.Fatal("enhanced path should run")
	}
	if calls.noAWSCache {
		t.Error("same account should not force --no-aws-cache")
	}
	if calls.okta[0] != "okta" || calls.okta[2] != "yubico" || calls.okta[6] != "/tmp/kc" {
		t.Errorf("unexpected enhanced args: %v", calls.okta)
	}
}

func TestUpdateKubeConfigForCluster_DifferentAccountForcesReauth(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "222222222222", nil)

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if !calls.noAWSCache {
		t.Error("different account should force --no-aws-cache")
	}
}

func TestUpdateKubeConfigForCluster_LookupFailureKeepsCache(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "", errors.New("no creds"))

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.okta == nil {
		t.Fatal("enhanced path should still run")
	}
	if calls.noAWSCache {
		t.Error("STS failure must not force --no-aws-cache")
	}
}

func TestUpdateKubeConfigForCluster_UnknownTargetAccountSkipsLookup(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "222222222222", nil)

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", ""); err != nil {
		t.Fatal(err)
	}
	if calls.lookups != 0 {
		t.Errorf("no STS lookup expected when target account unknown, got %d", calls.lookups)
	}
	if calls.noAWSCache {
		t.Error("should not force --no-aws-cache when target account unknown")
	}
}

func TestUpdateKubeConfigForCluster_PropagatesError(t *testing.T) {
	setupKubeconfigFakes(t, oktaConfig(), "111111111111", nil)
	wantErr := errors.New("boom")
	runUpdateKubeConfigOkta = func(string, string, string, string, string, string, string, bool) error { return wantErr }

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); !errors.Is(err, wantErr) {
		t.Errorf("got %v, want %v", err, wantErr)
	}
}
