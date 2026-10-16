package cmd

import (
	"errors"
	"testing"

	"github.com/jordiprats/kubectl-eks/pkg/awsconfig"
	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/eks"
	"github.com/jordiprats/kubectl-eks/pkg/okta"
)

type kubeconfigCalls struct {
	plain       []string
	okta        []string
	opts        eks.EnhancedOptions
	noAWSCache  bool
	lookups     int
	roleLookups int
	rolePref    []string
	roleAccount string
	execEnvARN  string
	execEnvVars map[string]string
}

func setupKubeconfigFakes(t *testing.T, cfg *data.AWSConfig, cachedAccount string, lookupErr error) *kubeconfigCalls {
	t.Helper()
	calls := &kubeconfigCalls{}

	origCfg, origLookup, origPlain, origOkta, origRole, origExecEnv := awsconfig.ConfigData, lookupCachedAccountID, runUpdateKubeConfig, runUpdateKubeConfigOkta, resolveOktaRole, setExecEnv
	t.Cleanup(func() {
		awsconfig.ConfigData, lookupCachedAccountID, runUpdateKubeConfig, runUpdateKubeConfigOkta, resolveOktaRole, setExecEnv = origCfg, origLookup, origPlain, origOkta, origRole, origExecEnv
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
	runUpdateKubeConfigOkta = func(profile, region, cluster, kubeConfig string, opts eks.EnhancedOptions) error {
		calls.okta = []string{profile, opts.CredentialProcess, opts.Factor, opts.Application, region, cluster, kubeConfig}
		calls.opts = opts
		calls.noAWSCache = opts.NoAWSCache
		return nil
	}
	resolveOktaRole = func(cp, factor, app, accountID string, pref []string) (okta.Choice, error) {
		calls.roleLookups++
		calls.roleAccount = accountID
		calls.rolePref = pref
		return okta.Choice{RoleARN: "arn:aws:iam::" + accountID + ":role/Team-PowerUser"}, nil
	}
	setExecEnv = func(clusterARN string, envVars map[string]string) error {
		calls.execEnvARN = clusterARN
		calls.execEnvVars = envVars
		return nil
	}
	return calls
}

func oktaConfig() *data.AWSConfig {
	return &data.AWSConfig{
		KubeconfigProfile: "okta",
		OktaFactor:        "yubico",
		RolePreference:    []string{"PowerUser", "Ops"},
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
	runUpdateKubeConfigOkta = func(string, string, string, string, eks.EnhancedOptions) error { return wantErr }

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); !errors.Is(err, wantErr) {
		t.Errorf("got %v, want %v", err, wantErr)
	}
}

func TestUpdateKubeConfigForCluster_ForceReauthSkipsLookupAndDisablesCache(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "111111111111", nil)

	orig := forceReauth
	forceReauth = true
	t.Cleanup(func() { forceReauth = orig })

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.okta == nil {
		t.Fatal("enhanced path should run")
	}
	if !calls.noAWSCache {
		t.Error("--reauth should force --no-aws-cache even for the same account")
	}
	if calls.lookups != 0 {
		t.Errorf("--reauth should skip the STS lookup, got %d lookups", calls.lookups)
	}
}

func TestUseCmdHasReauthFlag(t *testing.T) {
	f := useCmd.Flags().Lookup("reauth")
	if f == nil {
		t.Fatal("use command should define --reauth")
	}
	if f.DefValue != "" {
		t.Errorf("--reauth default = %q, want empty", f.DefValue)
	}
	if f.NoOptDefVal != reauthModeAlways {
		t.Errorf("bare --reauth = %q, want %q", f.NoOptDefVal, reauthModeAlways)
	}
}

func TestUpdateKubeConfigForCluster_DifferentAccountPreselectsRole(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "222222222222", nil)

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.roleLookups != 1 || calls.roleAccount != "111111111111" {
		t.Fatalf("expected one role lookup for the target account, got %d (%q)", calls.roleLookups, calls.roleAccount)
	}
	if len(calls.rolePref) != 2 || calls.rolePref[0] != "PowerUser" {
		t.Errorf("role preference not passed through: %v", calls.rolePref)
	}
	if calls.opts.Role != "arn:aws:iam::111111111111:role/Team-PowerUser" {
		t.Errorf("role not set on options: %+v", calls.opts)
	}
}

func TestUpdateKubeConfigForCluster_SameAccountSkipsRoleLookup(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "111111111111", nil)

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.roleLookups != 0 {
		t.Errorf("cached creds are valid, no role lookup expected, got %d", calls.roleLookups)
	}
	if calls.opts.Role != "" || calls.opts.AccountAlias != "" {
		t.Errorf("no role/alias expected, got %+v", calls.opts)
	}
}

func TestUpdateKubeConfigForCluster_ExpiredCredsPreselectRoleWithoutDisablingCache(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "", errors.New("expired"))

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.roleLookups != 1 || calls.opts.Role == "" {
		t.Errorf("a login will happen, so the role should be pre-selected: %+v (lookups %d)", calls.opts, calls.roleLookups)
	}
	if calls.noAWSCache {
		t.Error("STS failure must not force --no-aws-cache")
	}
}

func TestUpdateKubeConfigForCluster_ForceReauthPreselectsRole(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "111111111111", nil)

	orig := forceReauth
	forceReauth = true
	t.Cleanup(func() { forceReauth = orig })

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.roleLookups != 1 || calls.opts.Role == "" {
		t.Errorf("--reauth should pre-select the role: %+v", calls.opts)
	}
}

func TestUpdateKubeConfigForCluster_AccountAliasWhenNoPreferredRole(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "222222222222", nil)
	resolveOktaRole = func(string, string, string, string, []string) (okta.Choice, error) {
		return okta.Choice{AccountAlias: "team-a-dev"}, nil
	}

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.opts.Role != "" || calls.opts.AccountAlias != "team-a-dev" {
		t.Errorf("expected only the account alias filter, got %+v", calls.opts)
	}
}

func TestUpdateKubeConfigForCluster_RoleResolutionFailureFallsBackToPrompt(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "222222222222", nil)
	resolveOktaRole = func(string, string, string, string, []string) (okta.Choice, error) {
		return okta.Choice{}, errors.New("account not in app")
	}

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatalf("role resolution failure must not fail the switch: %v", err)
	}
	if calls.okta == nil {
		t.Fatal("enhanced update should still run")
	}
	if calls.opts.Role != "" || calls.opts.AccountAlias != "" {
		t.Errorf("no role/alias expected on fallback, got %+v", calls.opts)
	}
}

func TestUpdateKubeConfigForCluster_SetsIsolationKey(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "111111111111", nil)

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.opts.Key != "111111111111-okta" {
		t.Errorf("isolation key = %q, want %q", calls.opts.Key, "111111111111-okta")
	}
	wantARN := "arn:aws:eks:us-east-1:111111111111:cluster/c1"
	if calls.execEnvARN != wantARN {
		t.Errorf("exec env ARN = %q, want %q", calls.execEnvARN, wantARN)
	}
	if calls.execEnvVars["AWS_OKTA_KEY"] != "111111111111-okta" {
		t.Errorf("exec env AWS_OKTA_KEY = %q, want %q", calls.execEnvVars["AWS_OKTA_KEY"], "111111111111-okta")
	}
}

func TestUpdateKubeConfigForCluster_NoIsolationKeyWithoutAccountID(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "222222222222", nil)

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", ""); err != nil {
		t.Fatal(err)
	}
	if calls.opts.Key != "" {
		t.Errorf("isolation key should be empty without account ID, got %q", calls.opts.Key)
	}
	if calls.execEnvARN != "" {
		t.Errorf("exec env should not be patched without account ID, got ARN %q", calls.execEnvARN)
	}
}

func TestUpdateKubeConfigForCluster_NoIsolationKeyForTempKubeconfig(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "111111111111", nil)

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "/tmp/kc", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.opts.Key != "111111111111-okta" {
		t.Errorf("key should still be passed to credential_process, got %q", calls.opts.Key)
	}
	if calls.execEnvARN != "" {
		t.Errorf("exec env should not be patched for temp kubeconfig, got ARN %q", calls.execEnvARN)
	}
}

func TestUpdateKubeConfigForCluster_UnknownTargetAccountSkipsRoleLookup(t *testing.T) {
	calls := setupKubeconfigFakes(t, oktaConfig(), "222222222222", nil)

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", ""); err != nil {
		t.Fatal(err)
	}
	if calls.roleLookups != 0 {
		t.Errorf("no role lookup expected without a target account, got %d", calls.roleLookups)
	}
}

func TestUpdateKubeConfigForCluster_DiscoveryProfileAppOverridesGlobal(t *testing.T) {
	cfg := oktaConfig()
	cfg.AWSApp = "https://example.okta.com/global-app"
	cfg.Profiles["pci"] = data.AWSProfile{Name: "pci", AWSApp: "https://example.okta.com/pci-app"}
	calls := setupKubeconfigFakes(t, cfg, "111111111111", nil)

	if err := updateKubeConfigForCluster("pci", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.opts.Application != "https://example.okta.com/pci-app" {
		t.Errorf("discovery profile app should win, got %q", calls.opts.Application)
	}

	if err := updateKubeConfigForCluster("dev", "us-east-1", "c1", "", "111111111111"); err != nil {
		t.Fatal(err)
	}
	if calls.opts.Application != "https://example.okta.com/global-app" {
		t.Errorf("profiles without their own app should use the global one, got %q", calls.opts.Application)
	}
}

func TestParseReauthMode(t *testing.T) {
	tests := []struct {
		in             string
		always, onProf bool
		profile        string
	}{
		{"", false, false, ""},
		{"false", false, false, ""},
		{"always", true, false, ""},
		{"true", true, false, ""},
		{"profile", false, true, ""},
		{"PROFILE", false, true, ""},
		{"okta", false, true, "okta"},
	}
	for _, tt := range tests {
		always, onProf, profile := parseReauthMode(tt.in)
		if always != tt.always || onProf != tt.onProf || profile != tt.profile {
			t.Errorf("parseReauthMode(%q) = %v, %v, %q", tt.in, always, onProf, profile)
		}
	}
}

func TestCheckCmdFlags(t *testing.T) {
	q := checkCmd.Flags().Lookup("quiet")
	if q == nil {
		t.Fatal("check command should define --quiet")
	}
	if q.Shorthand != "q" || q.DefValue != "false" {
		t.Errorf("--quiet shorthand/default = %q/%q, want q/false", q.Shorthand, q.DefValue)
	}
	r := checkCmd.Flags().Lookup("reauth")
	if r == nil {
		t.Fatal("check command should define --reauth")
	}
	if r.DefValue != "false" {
		t.Errorf("--reauth default = %q, want false", r.DefValue)
	}
}

func TestCheckCmdRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"check"})
	if err != nil || cmd != checkCmd {
		t.Fatalf("check should be registered on the root command, got %v, %v", cmd, err)
	}
}
