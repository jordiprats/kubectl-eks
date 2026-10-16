package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// writeKubeconfig writes a clientcmdapi.Config to a temp file and sets the
// KUBECONFIG env var so client-go picks it up.
func writeKubeconfig(t *testing.T, cfg *clientcmdapi.Config) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	require.NoError(t, clientcmd.WriteToFile(*cfg, path))
	t.Setenv("KUBECONFIG", path)
	return path
}

func TestFindContextForCluster_NotFound(t *testing.T) {
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["other"] = &clientcmdapi.Cluster{Server: "https://other.example.com"}
	cfg.Contexts["other-ctx"] = &clientcmdapi.Context{Cluster: "other"}
	cfg.CurrentContext = "other-ctx"
	writeKubeconfig(t, cfg)

	name, ok := FindContextForCluster("arn:aws:eks:us-east-1:123456789012:cluster/my-cluster")
	assert.False(t, ok)
	assert.Empty(t, name)
}

func TestFindContextForCluster_Found(t *testing.T) {
	clusterARN := "arn:aws:eks:us-east-1:123456789012:cluster/my-cluster"
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters[clusterARN] = &clientcmdapi.Cluster{Server: "https://example.com"}
	cfg.Contexts["my-ctx"] = &clientcmdapi.Context{Cluster: clusterARN}
	cfg.CurrentContext = "my-ctx"
	writeKubeconfig(t, cfg)

	name, ok := FindContextForCluster(clusterARN)
	assert.True(t, ok)
	assert.Equal(t, "my-ctx", name)
}

func TestUseContext_Switches(t *testing.T) {
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["c1"] = &clientcmdapi.Cluster{Server: "https://c1.example.com"}
	cfg.Clusters["c2"] = &clientcmdapi.Cluster{Server: "https://c2.example.com"}
	cfg.Contexts["ctx1"] = &clientcmdapi.Context{Cluster: "c1"}
	cfg.Contexts["ctx2"] = &clientcmdapi.Context{Cluster: "c2"}
	cfg.CurrentContext = "ctx1"
	path := writeKubeconfig(t, cfg)

	err := UseContext("ctx2")
	require.NoError(t, err)

	// Read back and verify
	updated, err := clientcmd.LoadFromFile(path)
	require.NoError(t, err)
	assert.Equal(t, "ctx2", updated.CurrentContext)
}

func TestUseContext_NotFound(t *testing.T) {
	cfg := clientcmdapi.NewConfig()
	cfg.Contexts["ctx1"] = &clientcmdapi.Context{Cluster: "c1"}
	cfg.CurrentContext = "ctx1"
	writeKubeconfig(t, cfg)

	err := UseContext("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestUseContext_EmptyConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	require.NoError(t, os.WriteFile(path, []byte(""), 0600))
	t.Setenv("KUBECONFIG", path)

	err := UseContext("anything")
	assert.Error(t, err)
}

func TestSetExecEnv_AddsNewVar(t *testing.T) {
	clusterARN := "arn:aws:eks:us-east-1:123456789012:cluster/my-cluster"
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters[clusterARN] = &clientcmdapi.Cluster{Server: "https://example.com"}
	cfg.AuthInfos[clusterARN] = &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1beta1",
			Command:    "aws",
			Args:       []string{"eks", "get-token"},
			Env: []clientcmdapi.ExecEnvVar{
				{Name: "AWS_PROFILE", Value: "okta"},
			},
		},
	}
	cfg.Contexts["my-ctx"] = &clientcmdapi.Context{Cluster: clusterARN, AuthInfo: clusterARN}
	cfg.CurrentContext = "my-ctx"
	path := writeKubeconfig(t, cfg)

	err := SetExecEnv(clusterARN, map[string]string{"AWS_OKTA_KEY": "123-okta"})
	require.NoError(t, err)

	updated, err := clientcmd.LoadFromFile(path)
	require.NoError(t, err)

	user := updated.AuthInfos[clusterARN]
	require.NotNil(t, user)
	require.NotNil(t, user.Exec)

	envMap := make(map[string]string)
	for _, e := range user.Exec.Env {
		envMap[e.Name] = e.Value
	}
	assert.Equal(t, "okta", envMap["AWS_PROFILE"])
	assert.Equal(t, "123-okta", envMap["AWS_OKTA_KEY"])
}

func TestSetExecEnv_UpdatesExistingVar(t *testing.T) {
	clusterARN := "arn:aws:eks:us-east-1:123456789012:cluster/my-cluster"
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters[clusterARN] = &clientcmdapi.Cluster{Server: "https://example.com"}
	cfg.AuthInfos[clusterARN] = &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1beta1",
			Command:    "aws",
			Env: []clientcmdapi.ExecEnvVar{
				{Name: "AWS_PROFILE", Value: "okta"},
				{Name: "AWS_OKTA_KEY", Value: "old-key"},
			},
		},
	}
	cfg.Contexts["my-ctx"] = &clientcmdapi.Context{Cluster: clusterARN, AuthInfo: clusterARN}
	cfg.CurrentContext = "my-ctx"
	path := writeKubeconfig(t, cfg)

	err := SetExecEnv(clusterARN, map[string]string{"AWS_OKTA_KEY": "new-key"})
	require.NoError(t, err)

	updated, err := clientcmd.LoadFromFile(path)
	require.NoError(t, err)

	envMap := make(map[string]string)
	for _, e := range updated.AuthInfos[clusterARN].Exec.Env {
		envMap[e.Name] = e.Value
	}
	assert.Equal(t, "new-key", envMap["AWS_OKTA_KEY"])
	assert.Equal(t, "okta", envMap["AWS_PROFILE"])
}

func TestSetExecEnv_NoMatchingContext(t *testing.T) {
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["other"] = &clientcmdapi.Cluster{Server: "https://other.example.com"}
	cfg.Contexts["other-ctx"] = &clientcmdapi.Context{Cluster: "other", AuthInfo: "other"}
	cfg.CurrentContext = "other-ctx"
	writeKubeconfig(t, cfg)

	err := SetExecEnv("arn:aws:eks:us-east-1:123456789012:cluster/missing", map[string]string{"AWS_OKTA_KEY": "k"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no context found")
}

func TestSetExecEnv_EmptyEnvVars(t *testing.T) {
	err := SetExecEnv("anything", map[string]string{})
	assert.NoError(t, err)
}
