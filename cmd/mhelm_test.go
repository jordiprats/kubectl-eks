package cmd

import (
	"testing"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

func TestHelmNamespace(t *testing.T) {
	assert.Equal(t, "default", helmNamespace("", false))
	assert.Equal(t, "kube-system", helmNamespace("kube-system", false))
	assert.Equal(t, "", helmNamespace("", true))
	assert.Equal(t, "", helmNamespace("kube-system", true))
}

func TestLatestHelmReleases(t *testing.T) {
	releases := []*release.Release{
		{Name: "api", Namespace: "apps", Version: 1},
		{Name: "api", Namespace: "apps", Version: 3},
		{Name: "api", Namespace: "other", Version: 2},
		{Name: "web", Namespace: "apps", Version: 1},
		nil,
	}

	actual := latestHelmReleases(releases)

	assert.Len(t, actual, 3)
	assert.Equal(t, "api", actual[0].Name)
	assert.Equal(t, 3, actual[0].Version)
	assert.Equal(t, "web", actual[1].Name)
	assert.Equal(t, "other", actual[2].Namespace)
}

func TestHelmReleaseResult(t *testing.T) {
	cluster := data.ClusterInfo{AWSProfile: "prod", Region: "eu-west-1", ClusterName: "main"}
	helmRelease := &release.Release{
		Name:      "api",
		Namespace: "apps",
		Version:   4,
		Info:      &release.Info{Status: release.StatusDeployed},
		Chart: &chart.Chart{Metadata: &chart.Metadata{
			Name:       "api-chart",
			Version:    "1.2.3",
			AppVersion: "2.0.0",
		}},
	}

	actual := helmReleaseResult(cluster, helmRelease)

	assert.Equal(t, "prod", actual.Profile)
	assert.Equal(t, "deployed", actual.Status)
	assert.Equal(t, "api-chart-1.2.3", actual.Chart)
	assert.Equal(t, "2.0.0", actual.AppVersion)
	assert.Equal(t, "-", actual.Updated)
}

func TestHelmReleaseResultHandlesMissingMetadata(t *testing.T) {
	actual := helmReleaseResult(data.ClusterInfo{}, &release.Release{Name: "api"})

	assert.Equal(t, "unknown", actual.Status)
	assert.Equal(t, "-", actual.Chart)
	assert.Equal(t, "-", actual.AppVersion)
}
